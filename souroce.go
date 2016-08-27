package scp

import (
	"strings"

	"golang.org/x/crypto/ssh"
)

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

func escapeShellArg(arg string) string {
	return "'" + strings.Replace(arg, "'", `'\''`, -1) + "'"
}
