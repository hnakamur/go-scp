package scp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

func CopyFromReaderToRemote(client *ssh.Client, info FileInfo, r io.ReadCloser, remoteFilename string) error {
	remoteFilename = filepath.Clean(remoteFilename)
	destDir := filepath.Dir(remoteFilename)
	destFilename := filepath.Base(remoteFilename)
	if info.name != destFilename {
		info = NewFileInfo(destFilename, info.Size(), info.Mode(), info.ModTime(), info.AccessTime())
	}

	s, err := NewSourceSession(client, destDir, true, "", false, true)
	defer s.Close()
	if err != nil {
		return err
	}
	err = func() error {
		defer s.CloseStdin()

		err = s.WriteFile(info, r)
		if err != nil {
			return fmt.Errorf("failed to copy file: err=%s", err)
		}
		return nil
	}()
	if err != nil {
		return err
	}
	return s.Wait()
}

func CopyFileToRemote(client *ssh.Client, localFilename, remoteFilename string) error {
	localFilename = filepath.Clean(localFilename)
	remoteFilename = filepath.Clean(remoteFilename)

	destDir := filepath.Dir(remoteFilename)
	destFilename := filepath.Base(remoteFilename)

	s, err := NewSourceSession(client, destDir, true, "", false, true)
	defer s.Close()
	if err != nil {
		return err
	}
	err = func() error {
		defer s.CloseStdin()

		osFileInfo, err := os.Stat(localFilename)
		if err != nil {
			return fmt.Errorf("failed to stat source file: err=%s", err)
		}
		fi := NewFileInfoFromOS(osFileInfo, true, destFilename)

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
	}()
	if err != nil {
		return err
	}
	return s.Wait()
}

func CopyRecursivelyToRemote(client *ssh.Client, srcDir, destDir string, walkFn filepath.WalkFunc) error {
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)

	s, err := NewSourceSession(client, destDir, true, "", true, true)
	defer s.Close()
	if err != nil {
		return err
	}
	err = func() error {
		defer s.CloseStdin()

		endDirectories := func(prevDir, dir string) ([]string, error) {
			rel, err := filepath.Rel(prevDir, dir)
			if err != nil {
				return nil, err
			}
			var dirs []string
			for _, comp := range strings.Split(rel, string([]rune{filepath.Separator})) {
				if comp == ".." {
					err := s.EndDirectory()
					if err != nil {
						return nil, err
					}
				} else if comp == "." {
					continue
				} else {
					dirs = append(dirs, comp)
				}
			}
			return dirs, nil
		}

		isSrcDir := true
		var srcDirInfo os.FileInfo
		prevDir := srcDir
		myWalkFn := func(path string, info os.FileInfo, err error) error {
			if isSrcDir {
				srcDirInfo = info
				isSrcDir = false
			}

			isDir := info.IsDir()
			var dir string
			if isDir {
				dir = path
			} else {
				dir = filepath.Dir(path)
			}

			newDirs, err := endDirectories(prevDir, dir)
			if err != nil {
				return err
			}

			err = walkFn(path, info, err)
			if err != nil {
				return err
			}

			defer func() {
				prevDir = dir
			}()

			for _, newDir := range newDirs {
				fi := NewFileInfoFromOS(info, true, newDir)
				err := s.StartDirectory(fi)
				if err != nil {
					return err
				}
			}

			if !isDir {
				fi := NewFileInfoFromOS(info, true, "")
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				err = s.WriteFile(fi, file)
				if err != nil {
					return err
				}
			}
			return nil
		}
		err = filepath.Walk(srcDir, myWalkFn)
		if err != nil {
			return err
		}

		_, err = endDirectories(prevDir, srcDir)
		if err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		return err
	}
	return s.Wait()
}

type SourceSession struct {
	client            *ssh.Client
	session           *ssh.Session
	remoteDestPath    string
	remoteDestIsDir   bool
	scpPath           string
	recursive         bool
	updatesPermission bool
	stdin             io.WriteCloser
	stdout            io.Reader
	*sourceProtocol
}

func NewSourceSession(client *ssh.Client, remoteDestPath string, remoteDestIsDir bool, scpPath string, recursive, updatesPermission bool) (*SourceSession, error) {
	s := &SourceSession{
		client:            client,
		remoteDestPath:    remoteDestPath,
		remoteDestIsDir:   remoteDestIsDir,
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

	cmd := s.scpPath + " " + string(opt) + " " + EscapeShellArg(s.remoteDestPath)
	err = s.session.Start(cmd)
	if err != nil {
		return s, err
	}

	s.sourceProtocol, err = newSourceProtocol(s.stdin, s.stdout)
	return s, err
}

func (s *SourceSession) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Close()
}

func (s *SourceSession) Wait() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Wait()
}

func (s *SourceSession) CloseStdin() error {
	if s == nil || s.stdin == nil {
		return nil
	}
	return s.stdin.Close()
}
