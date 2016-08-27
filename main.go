package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	auth, err := sshAgent()
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{auth},
	}

	client, err := ssh.Dial("tcp", "10.155.92.21:22", config)
	if err != nil {
		return err
	}
	defer client.Close()

	destDir := "/tmp"
	copier := func(s *Source) error {
		mode := os.FileMode(0644)
		filename := "test1"
		content := "content1\n"
		modTime := time.Date(2006, 1, 2, 15, 04, 05, 678901000, time.Local)
		accessTime := time.Date(2018, 8, 31, 23, 59, 58, 999999000, time.Local)
		fi := NewFileInfo(filename, int64(len(content)), mode, modTime, accessTime)
		err = s.WriteFile(fi, ioutil.NopCloser(bytes.NewBufferString(content)))
		if err != nil {
			return err
		}

		di := NewDirInfo("test2", os.FileMode(0755), time.Time{}, time.Time{})
		err = s.StartDirectory(di)
		if err != nil {
			return err
		}

		di = NewDirInfo("sub", os.FileMode(0750), time.Time{}, time.Time{})
		err = s.StartDirectory(di)
		if err != nil {
			return err
		}

		mode = os.FileMode(0604)
		filename = "test2"
		content = ""
		fi = NewFileInfo(filename, int64(len(content)), mode, time.Time{}, time.Time{})
		err = s.WriteFile(fi, ioutil.NopCloser(bytes.NewBufferString(content)))
		if err != nil {
			return err
		}

		err = s.EndDirectory()
		if err != nil {
			return err
		}

		return s.EndDirectory()
	}
	return NewSource(client, destDir, "", true, true).CopyToRemote(copier)
}

func sshAgent() (ssh.AuthMethod, error) {
	agentSock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(agentSock).Signers), nil
}

func escapeShellArg(arg string) string {
	return "'" + strings.Replace(arg, "'", `'\''`, -1) + "'"
}

type Source struct {
	client            *ssh.Client
	session           *ssh.Session
	remoteDestDir     string
	scpPath           string
	recursive         bool
	updatesPermission bool
	*sourceProtocol
}

func NewSource(client *ssh.Client, remoteDestDir, scpPath string, recursive, updatesPermission bool) *Source {
	return &Source{
		client:            client,
		remoteDestDir:     remoteDestDir,
		scpPath:           scpPath,
		recursive:         recursive,
		updatesPermission: updatesPermission,
	}
}

func (s *Source) CopyToRemote(copier func(src *Source) error) error {
	var err error
	s.session, err = s.client.NewSession()
	if err != nil {
		return err
	}
	defer s.session.Close()

	stdout, err := s.session.StdoutPipe()
	if err != nil {
		return err
	}

	stdin, err := s.session.StdinPipe()
	if err != nil {
		return err
	}

	err = func() error {
		defer stdin.Close()

		if s.scpPath == "" {
			s.scpPath = "scp"
		}

		opt := []byte("-t")
		if s.recursive {
			opt = append(opt, 'r')
		}
		if s.updatesPermission {
			opt = append(opt, 'p')
		}

		cmd := s.scpPath + " " + string(opt) + " " + escapeShellArg(s.remoteDestDir)
		err = s.session.Start(cmd)
		if err != nil {
			return err
		}

		s.sourceProtocol, err = newSourceProtocol(stdin, stdout)
		if err != nil {
			return err
		}

		return copier(s)
	}()
	if err != nil {
		return err
	}

	return s.session.Wait()
}

type FileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     SysFileInfo
}

type SysFileInfo struct {
	AccessTime time.Time
}

func NewFileInfo(name string, size int64, mode os.FileMode, modTime, accessTime time.Time) FileInfo {
	return FileInfo{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
		sys:     SysFileInfo{AccessTime: accessTime},
	}
}

func NewDirInfo(name string, mode os.FileMode, modTime, accessTime time.Time) FileInfo {
	return FileInfo{
		name:    name,
		mode:    mode,
		modTime: modTime,
		isDir:   true,
		sys:     SysFileInfo{AccessTime: accessTime},
	}
}

func (i *FileInfo) Name() string       { return i.name }
func (i *FileInfo) Size() int64        { return i.size }
func (i *FileInfo) Mode() os.FileMode  { return i.mode }
func (i *FileInfo) ModTime() time.Time { return i.modTime }
func (i *FileInfo) IsDir() bool        { return i.isDir }
func (i *FileInfo) Sys() interface{}   { return i.sys }

const (
	msgCopyFile       = 'C'
	msgStartDirectory = 'D'
	msgEndDirectory   = 'E'
	msgTime           = 'T'
)

const (
	replyOK         = '\x00'
	replyError      = '\x01'
	replyFatalError = '\x02'
)

type sourceProtocol struct {
	remIn     io.WriteCloser
	remOut    io.Reader
	remReader *bufio.Reader
}

func newSourceProtocol(remIn io.WriteCloser, remOut io.Reader) (*sourceProtocol, error) {
	s := &sourceProtocol{
		remIn:     remIn,
		remOut:    remOut,
		remReader: bufio.NewReader(remOut),
	}

	return s, s.readReply()
}

func (s *sourceProtocol) WriteFile(fileInfo FileInfo, body io.ReadCloser) error {
	if !fileInfo.modTime.IsZero() || !fileInfo.sys.AccessTime.IsZero() {
		err := s.setTime(fileInfo.modTime, fileInfo.sys.AccessTime)
		if err != nil {
			return err
		}
	}
	return s.writeFile(fileInfo.mode, fileInfo.size, fileInfo.name, body)
}

func (s *sourceProtocol) StartDirectory(dirInfo FileInfo) error {
	if !dirInfo.modTime.IsZero() || !dirInfo.sys.AccessTime.IsZero() {
		err := s.setTime(dirInfo.modTime, dirInfo.sys.AccessTime)
		if err != nil {
			return err
		}
	}
	return s.startDirectory(dirInfo.mode, dirInfo.name)
}

func (s *sourceProtocol) EndDirectory() error {
	return s.endDirectory()
}

func (s *sourceProtocol) setTime(mtime, atime time.Time) error {
	ms, mus := secondsAndMicroseconds(mtime)
	as, aus := secondsAndMicroseconds(atime)
	_, err := fmt.Fprintf(s.remIn, "%c%d %d %d %d\n", msgTime, ms, mus, as, aus)
	if err != nil {
		return fmt.Errorf("failed to write scp time header: err=%s", err)
	}
	return s.readReply()
}

func secondsAndMicroseconds(t time.Time) (seconds int64, microseconds int) {
	rounded := t.Round(time.Microsecond)
	return rounded.Unix(), rounded.Nanosecond() / int(int64(time.Microsecond)/int64(time.Nanosecond))
}

func (s *sourceProtocol) writeFile(mode os.FileMode, length int64, filename string, body io.ReadCloser) error {
	_, err := fmt.Fprintf(s.remIn, "%c%#4o %d %s\n", msgCopyFile, mode, length, filename)
	if err != nil {
		return fmt.Errorf("failed to write scp file header: err=%s", err)
	}
	_, err = io.Copy(s.remIn, body)
	// NOTE: We close body whether or not copy fails and ignore an error from closing body.
	body.Close()
	if err != nil {
		return fmt.Errorf("failed to write scp file body: err=%s", err)
	}
	err = s.readReply()
	if err != nil {
		return err
	}

	_, err = s.remIn.Write([]byte{replyOK})
	if err != nil {
		return fmt.Errorf("failed to write scp replyOK reply: err=%s", err)
	}
	return s.readReply()
}

func (s *sourceProtocol) startDirectory(mode os.FileMode, dirname string) error {
	// length is not used.
	length := 0
	_, err := fmt.Fprintf(s.remIn, "%c%#4o %d %s\n", msgStartDirectory, mode, length, dirname)
	if err != nil {
		return fmt.Errorf("failed to write scp start directory header: err=%s", err)
	}
	return s.readReply()
}

func (s *sourceProtocol) endDirectory() error {
	_, err := fmt.Fprintf(s.remIn, "%c\n", msgEndDirectory)
	if err != nil {
		return fmt.Errorf("failed to write scp end directory header: err=%s", err)
	}
	return s.readReply()
}

func (s *sourceProtocol) readReply() error {
	b, err := s.remReader.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read scp reply type: err=%s", err)
	}
	if b == replyOK {
		return nil
	}
	if b != replyError && b != replyFatalError {
		return fmt.Errorf("unexpected scp reply type: %v", b)
	}
	var line []byte
	line, err = s.remReader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("failed to read scp reply message: err=%s", err)
	}
	return &SCPProtocolError{
		msg:   string(line),
		fatal: b == replyFatalError,
	}
}

type SCPProtocolError struct {
	msg   string
	fatal bool
}

func (e *SCPProtocolError) Error() string { return e.msg }
func (e *SCPProtocolError) Fatal() bool   { return e.fatal }
