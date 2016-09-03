package scp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func CopyFileFromRemote(client *ssh.Client, remoteFilename, localFilename string, updatesPermission, setTime bool) error {
	remoteFilename = filepath.Clean(remoteFilename)
	localFilename = filepath.Clean(localFilename)

	s, err := NewSinkSession(client, remoteFilename, false, "", false, updatesPermission)
	defer s.Close()
	if err != nil {
		return err
	}

	var timeHeader TimeMsgHeader
	if setTime {
		h, err := s.ReadHeaderOrReply()
		if err != nil {
			return fmt.Errorf("failed to read scp message header: err=%s", err)
		}
		var ok bool
		timeHeader, ok = h.(TimeMsgHeader)
		if !ok {
			return fmt.Errorf("expected time message header, got %+v", h)
		}
	}

	h, err := s.ReadHeaderOrReply()
	if err != nil {
		return fmt.Errorf("failed to read scp message header: err=%s", err)
	}
	fileHeader, ok := h.(FileMsgHeader)
	if !ok {
		return fmt.Errorf("expected file message header, got %+v", h)
	}

	err = copyFileBodyFromRemote(s, localFilename, timeHeader, fileHeader, updatesPermission, setTime)
	if err != nil {
		return err
	}
	return s.Wait()
}

func copyFileBodyFromRemote(s *SinkSession, localFilename string, timeHeader TimeMsgHeader, fileHeader FileMsgHeader, updatesPermission, setTime bool) error {
	file, err := os.OpenFile(localFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileHeader.Mode)
	if err != nil {
		return fmt.Errorf("failed to open destination file: err=%s", err)
	}

	err = s.CopyFileBodyTo(fileHeader, file)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to copy file: err=%s", err)
	}
	file.Close()

	if updatesPermission {
		err := os.Chmod(localFilename, fileHeader.Mode)
		if err != nil {
			return fmt.Errorf("failed to change file mode: err=%s", err)
		}
	}

	if setTime {
		err := os.Chtimes(localFilename, timeHeader.Atime, timeHeader.Mtime)
		if err != nil {
			return fmt.Errorf("failed to change file time: err=%s", err)
		}
	}

	return nil
}

func CopyRecursivelyFromRemote(client *ssh.Client, srcDir, destDir string, updatesPermission, setTime bool) error {
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)

	s, err := NewSinkSession(client, srcDir, true, "", true, updatesPermission)
	defer s.Close()
	if err != nil {
		return err
	}

	curDir := destDir
	var timeHeader TimeMsgHeader
	var timeHeaders []TimeMsgHeader
	isFirstStartDirectory := true
	for {
		h, err := s.ReadHeaderOrReply()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed to read scp message header: err=%s", err)
		}
		switch h.(type) {
		case TimeMsgHeader:
			timeHeader = h.(TimeMsgHeader)
		case StartDirectoryMsgHeader:
			if isFirstStartDirectory {
				isFirstStartDirectory = false
				continue
			}
			dirHeader := h.(StartDirectoryMsgHeader)
			curDir = filepath.Join(curDir, dirHeader.Name)
			err := os.MkdirAll(curDir, dirHeader.Mode)
			if err != nil {
				return fmt.Errorf("failed to create directory: err=%s", err)
			}

			if updatesPermission {
				err := os.Chmod(curDir, dirHeader.Mode)
				if err != nil {
					return fmt.Errorf("failed to change directory mode: err=%s", err)
				}
			}

			if setTime {
				timeHeaders = append(timeHeaders, timeHeader)
			}
		case EndDirectoryMsgHeader:
			if setTime && len(timeHeaders) > 0 {
				timeHeader = timeHeaders[len(timeHeaders)-1]
				timeHeaders = timeHeaders[:len(timeHeaders)-1]
				err := os.Chtimes(curDir, timeHeader.Atime, timeHeader.Mtime)
				if err != nil {
					return fmt.Errorf("failed to change directory time: err=%s", err)
				}
			}
			curDir = filepath.Dir(curDir)
		case FileMsgHeader:
			fileHeader := h.(FileMsgHeader)
			localFilename := filepath.Join(curDir, fileHeader.Name)
			err := copyFileBodyFromRemote(s, localFilename, timeHeader, fileHeader, updatesPermission, setTime)
			if err != nil {
				return err
			}
		case OKMsg:
			// do nothing
		}
	}
	return s.Wait()
}

type SinkSession struct {
	client            *ssh.Client
	session           *ssh.Session
	remoteSrcPath     string
	remoteSrcIsDir    bool
	scpPath           string
	recursive         bool
	updatesPermission bool
	stdin             io.WriteCloser
	stdout            io.Reader
	*sinkProtocol
}

func NewSinkSession(client *ssh.Client, remoteSrcPath string, remoteSrcIsDir bool, scpPath string, recursive, updatesPermission bool) (*SinkSession, error) {
	s := &SinkSession{
		client:            client,
		remoteSrcPath:     remoteSrcPath,
		remoteSrcIsDir:    remoteSrcIsDir,
		scpPath:           scpPath,
		recursive:         recursive,
		updatesPermission: updatesPermission,
	}

	var err error
	s.session, err = s.client.NewSession()
	if err != nil {
		return s, err
	}

	s.stdout, err = s.session.StdoutPipe()
	if err != nil {
		return s, err
	}

	s.stdin, err = s.session.StdinPipe()
	if err != nil {
		return s, err
	}

	if s.scpPath == "" {
		s.scpPath = "scp"
	}

	opt := []byte("-f")
	if s.updatesPermission {
		opt = append(opt, 'p')
	}
	if s.recursive {
		opt = append(opt, 'r')
	}
	if s.remoteSrcIsDir {
		opt = append(opt, 'd')
	}

	cmd := s.scpPath + " " + string(opt) + " " + EscapeShellArg(s.remoteSrcPath)
	err = s.session.Start(cmd)
	if err != nil {
		return s, err
	}

	s.sinkProtocol, err = newSinkProtocol(s.stdin, s.stdout)
	return s, err
}

func (s *SinkSession) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Close()
}

func (s *SinkSession) Wait() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Wait()
}
