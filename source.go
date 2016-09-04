package scp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// CopyFromReaderToRemote reads a single local file content from the r,
// and copies it to the remote file with the name remoteFilename.
// The time and permission will be set with the value of info.
// The r will be closed after copying. If you don't want for r to be
// closed, you can pass the result of ioutil.NopCloser(r).
func CopyFromReaderToRemote(client *ssh.Client, info *FileInfo, r io.ReadCloser, remoteFilename string) error {
	remoteFilename = filepath.Clean(remoteFilename)
	destDir := filepath.Dir(remoteFilename)
	destFilename := filepath.Base(remoteFilename)
	if info.name != destFilename {
		info = NewFileInfo(destFilename, info.size, info.mode, info.modTime, info.accessTime)
	}

	return runSourceSession(client, destDir, true, "", false, true, func(s *sourceSession) error {
		err := s.WriteFile(info, r)
		if err != nil {
			return fmt.Errorf("failed to copy file: err=%s", err)
		}
		return nil
	})
}

// CopyFileToRemote copies a single local file to the remote server.
// The time and permission will be set with the value of the source file.
func CopyFileToRemote(client *ssh.Client, localFilename, remoteFilename string) error {
	localFilename = filepath.Clean(localFilename)
	remoteFilename = filepath.Clean(remoteFilename)

	destDir := filepath.Dir(remoteFilename)
	destFilename := filepath.Base(remoteFilename)

	return runSourceSession(client, destDir, true, "", false, true, func(s *sourceSession) error {
		osFileInfo, err := os.Stat(localFilename)
		if err != nil {
			return fmt.Errorf("failed to stat source file: err=%s", err)
		}
		fi := newFileInfoFromOS(osFileInfo, destFilename)

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
	})
}

// AcceptFunc is the type of the function called for each file or directory
// to determine whether is should be copied or not.
// In CopyRecursivelyToRemote, parentDir will be a directory under srcDir.
// In CopyRecursivelyFromRemote, parentDir will be a directory under destDir.
type AcceptFunc func(parentDir string, info os.FileInfo) (bool, error)

func acceptAny(parentDir string, info os.FileInfo) (bool, error) {
	return true, nil
}

// CopyRecursivelyToRemote copies files and directories under the local srcDir to
// to the remote destDir. You can filter the files and directories to be copied with acceptFn.
// However this filtering is done at the receiver side, so all file bodies are transferred
// over the network even if some files are filtered out. If you need more efficiency,
// it is better to use another method like the tar command.
// If acceptFn is nil, all files and directories will be copied.
// The time and permission will be set to the same value of the source file or directory.
func CopyRecursivelyToRemote(client *ssh.Client, srcDir, destDir string, acceptFn AcceptFunc) error {
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)
	if acceptFn == nil {
		acceptFn = acceptAny
	}

	return runSourceSession(client, destDir, true, "", true, true, func(s *sourceSession) error {
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

			scpFileInfo := newFileInfoFromOS(info, path)
			accepted, err := acceptFn(filepath.Dir(path), scpFileInfo)
			if err != nil {
				return err
			}
			if isDir && !accepted {
				return filepath.SkipDir
			}

			defer func() {
				prevDir = dir
			}()

			for _, newDir := range newDirs {
				fi := newFileInfoFromOS(info, newDir)
				err := s.StartDirectory(fi)
				if err != nil {
					return err
				}
			}

			if !isDir && accepted {
				fi := newFileInfoFromOS(info, "")
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
		err := filepath.Walk(srcDir, myWalkFn)
		if err != nil {
			return err
		}

		_, err = endDirectories(prevDir, srcDir)
		if err != nil {
			return err
		}
		return nil
	})
}

type sourceSession struct {
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

func newSourceSession(client *ssh.Client, remoteDestPath string, remoteDestIsDir bool, scpPath string, recursive, updatesPermission bool) (*sourceSession, error) {
	s := &sourceSession{
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

	cmd := s.scpPath + " " + string(opt) + " " + escapeShellArg(s.remoteDestPath)
	err = s.session.Start(cmd)
	if err != nil {
		return s, err
	}

	s.sourceProtocol, err = newSourceProtocol(s.stdin, s.stdout)
	return s, err
}

func (s *sourceSession) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Close()
}

func (s *sourceSession) Wait() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Wait()
}

func (s *sourceSession) CloseStdin() error {
	if s == nil || s.stdin == nil {
		return nil
	}
	return s.stdin.Close()
}

func runSourceSession(client *ssh.Client, remoteDestPath string, remoteDestIsDir bool, scpPath string, recursive, updatesPermission bool, handler func(s *sourceSession) error) error {
	s, err := newSourceSession(client, remoteDestPath, remoteDestIsDir, scpPath, recursive, updatesPermission)
	defer s.Close()
	if err != nil {
		return err
	}
	err = func() error {
		defer s.CloseStdin()

		return handler(s)
	}()
	if err != nil {
		return err
	}
	return s.Wait()
}
