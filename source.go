package scp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

func CopyFileToRemote(client *ssh.Client, localFilename, remoteFilename string, updatesPermission, setTime bool) error {
	destDir := filepath.Dir(remoteFilename)
	destFilename := filepath.Base(remoteFilename)

	s := NewSource(client, destDir, true, "", false, updatesPermission)

	osFileInfo, err := os.Stat(localFilename)
	if err != nil {
		return fmt.Errorf("failed to stat source file: err=%s", err)
	}
	fi := NewFileInfoFromOS(osFileInfo, setTime, destFilename)

	copier := func(s *Source) error {
		file, err := os.Open(localFilename)
		if err != nil {
			return fmt.Errorf("failed to open source file: err=%s", err)
		}
		// NOTE: file will be closed by WriteFile.
		err = s.WriteFile(fi, file)
		if err != nil {
			return fmt.Errorf("failed to copy file: err=%s", err)
		}
		return nil
	}
	return s.CopyToRemote(copier)
}

type Source struct {
	client            *ssh.Client
	session           *ssh.Session
	remoteDestPath    string
	remoteDestIsDir   bool
	scpPath           string
	recursive         bool
	updatesPermission bool
	*sourceProtocol
}

func NewSource(client *ssh.Client, remoteDestPath string, remoteDestIsDir bool, scpPath string, recursive, updatesPermission bool) *Source {
	return &Source{
		client:            client,
		remoteDestPath:    remoteDestPath,
		remoteDestIsDir:   remoteDestIsDir,
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
		if s.updatesPermission {
			opt = append(opt, 'p')
		}
		if s.recursive {
			opt = append(opt, 'r')
		}
		if s.remoteDestIsDir {
			opt = append(opt, 'd')
		}

		cmd := s.scpPath + " " + string(opt) + " " + escapeShellArg(s.remoteDestPath)
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

func escapeShellArg(arg string) string {
	return "'" + strings.Replace(arg, "'", `'\''`, -1) + "'"
}
