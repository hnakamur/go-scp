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
	f := func(session *ssh.Session, s *source) error {
		err = s.setTime(
			time.Date(2006, 1, 2, 15, 04, 05, 678901000, time.Local),
			time.Date(2018, 8, 31, 23, 59, 58, 999999000, time.Local),
		)
		if err != nil {
			return err
		}

		mode := os.FileMode(0644)
		filename := "test1"
		content := "content1\n"
		err = s.writeFile(mode, int64(len(content)), filename, ioutil.NopCloser(bytes.NewBufferString(content)))
		if err != nil {
			return err
		}

		err = s.startDirectory(os.FileMode(0755), "test2")
		if err != nil {
			return err
		}

		err = s.startDirectory(os.FileMode(0750), "sub")
		if err != nil {
			return err
		}

		mode = os.FileMode(0604)
		filename = "test2"
		content = ""
		err = s.writeFile(mode, int64(len(content)), filename, ioutil.NopCloser(bytes.NewBufferString(content)))
		if err != nil {
			return err
		}

		err = s.endDirectory()
		if err != nil {
			return err
		}

		return s.endDirectory()
	}
	return copyToRemote(client, destDir, "", true, true, f)
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

func copyToRemote(client *ssh.Client, destDir, scpPath string, recursive, updatesPermission bool, f func(session *ssh.Session, src *source) error) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	err = func() error {
		defer stdin.Close()

		if scpPath == "" {
			scpPath = "scp"
		}

		opt := []byte("-t")
		if recursive {
			opt = append(opt, 'r')
		}
		if updatesPermission {
			opt = append(opt, 'p')
		}

		cmd := scpPath + " " + string(opt) + " " + escapeShellArg(destDir)
		err = session.Start(cmd)
		if err != nil {
			return err
		}

		s, err := newSource(stdin, stdout)
		if err != nil {
			return err
		}

		return f(session, s)
	}()
	if err != nil {
		return err
	}

	return session.Wait()
}

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

type source struct {
	remIn     io.WriteCloser
	remOut    io.Reader
	remReader *bufio.Reader
}

func newSource(remIn io.WriteCloser, remOut io.Reader) (*source, error) {
	s := &source{
		remIn:     remIn,
		remOut:    remOut,
		remReader: bufio.NewReader(remOut),
	}

	return s, s.readReply()
}

func (s *source) setTime(mtime, atime time.Time) error {
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

func (s *source) writeFile(mode os.FileMode, length int64, filename string, body io.ReadCloser) error {
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

func (s *source) startDirectory(mode os.FileMode, dirname string) error {
	// length is not used.
	length := 0
	_, err := fmt.Fprintf(s.remIn, "%c%#4o %d %s\n", msgStartDirectory, mode, length, dirname)
	if err != nil {
		return fmt.Errorf("failed to write scp start directory header: err=%s", err)
	}
	return s.readReply()
}

func (s *source) endDirectory() error {
	_, err := fmt.Fprintf(s.remIn, "%c\n", msgEndDirectory)
	if err != nil {
		return fmt.Errorf("failed to write scp end directory header: err=%s", err)
	}
	return s.readReply()
}

type SCPProtocolError struct {
	msg   string
	fatal bool
}

func (e *SCPProtocolError) Error() string { return e.msg }
func (e *SCPProtocolError) Fatal() bool   { return e.fatal }

func (s *source) readReply() error {
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
