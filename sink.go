package scp

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

func CopyFromRemoteToWriter(client *ssh.Client, remoteFilename string, dest io.Writer) (os.FileInfo, error) {
	var info os.FileInfo
	err := runSinkSession(client, remoteFilename, false, "", false, true, func(s *sinkSession) error {
		var timeHeader timeMsgHeader
		h, err := s.ReadHeaderOrReply()
		if err != nil {
			return fmt.Errorf("failed to read scp message header: err=%s", err)
		}
		var ok bool
		timeHeader, ok = h.(timeMsgHeader)
		if !ok {
			return fmt.Errorf("expected time message header, got %+v", h)
		}

		h, err = s.ReadHeaderOrReply()
		if err != nil {
			return fmt.Errorf("failed to read scp message header: err=%s", err)
		}
		fileHeader, ok := h.(fileMsgHeader)
		if !ok {
			return fmt.Errorf("expected file message header, got %+v", h)
		}
		err = s.CopyFileBodyTo(fileHeader, dest)
		if err != nil {
			return fmt.Errorf("failed to copy file: err=%s", err)
		}

		info = newFileInfo(remoteFilename, fileHeader.Size, fileHeader.Mode, timeHeader.Mtime, timeHeader.Atime)
		return nil
	})
	return info, err
}

func CopyFileFromRemote(client *ssh.Client, remoteFilename, localFilename string) error {
	remoteFilename = filepath.Clean(remoteFilename)
	localFilename = filepath.Clean(localFilename)

	return runSinkSession(client, remoteFilename, false, "", false, true, func(s *sinkSession) error {
		h, err := s.ReadHeaderOrReply()
		if err != nil {
			return fmt.Errorf("failed to read scp message header: err=%s", err)
		}
		timeHeader, ok := h.(timeMsgHeader)
		if !ok {
			return fmt.Errorf("expected time message header, got %+v", h)
		}

		h, err = s.ReadHeaderOrReply()
		if err != nil {
			return fmt.Errorf("failed to read scp message header: err=%s", err)
		}
		fileHeader, ok := h.(fileMsgHeader)
		if !ok {
			return fmt.Errorf("expected file message header, got %+v", h)
		}

		return copyFileBodyFromRemote(s, localFilename, timeHeader, fileHeader)
	})
}

func copyFileBodyFromRemote(s *sinkSession, localFilename string, timeHeader timeMsgHeader, fileHeader fileMsgHeader) error {
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

	err = os.Chmod(localFilename, fileHeader.Mode)
	if err != nil {
		return fmt.Errorf("failed to change file mode: err=%s", err)
	}

	err = os.Chtimes(localFilename, timeHeader.Atime, timeHeader.Mtime)
	if err != nil {
		return fmt.Errorf("failed to change file time: err=%s", err)
	}

	return nil
}

func CopyRecursivelyFromRemote(client *ssh.Client, srcDir, destDir string, acceptFn AcceptFunc) error {
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)

	if acceptFn == nil {
		acceptFn = acceptAny
	}

	return runSinkSession(client, srcDir, true, "", true, true, func(s *sinkSession) error {
		curDir := destDir
		var timeHeader timeMsgHeader
		var timeHeaders []timeMsgHeader
		isFirstStartDirectory := true
		var skipBaseDir string
		for {
			h, err := s.ReadHeaderOrReply()
			if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("failed to read scp message header: err=%s", err)
			}
			switch h.(type) {
			case timeMsgHeader:
				timeHeader = h.(timeMsgHeader)
			case startDirectoryMsgHeader:
				if isFirstStartDirectory {
					isFirstStartDirectory = false
					continue
				}

				dirHeader := h.(startDirectoryMsgHeader)
				curDir = filepath.Join(curDir, dirHeader.Name)
				timeHeaders = append(timeHeaders, timeHeader)

				if skipBaseDir != "" {
					continue
				}

				info := newDirInfo(curDir, dirHeader.Mode, timeHeader.Mtime, timeHeader.Atime)
				accepted, err := acceptFn(info)
				if err != nil {
					return fmt.Errorf("error from accessFn: err=%s", err)
				}
				if !accepted {
					skipBaseDir = curDir
					continue
				}

				err = os.MkdirAll(curDir, dirHeader.Mode)
				if err != nil {
					return fmt.Errorf("failed to create directory: err=%s", err)
				}

				err = os.Chmod(curDir, dirHeader.Mode)
				if err != nil {
					return fmt.Errorf("failed to change directory mode: err=%s", err)
				}
			case endDirectoryMsgHeader:
				if len(timeHeaders) > 0 {
					timeHeader = timeHeaders[len(timeHeaders)-1]
					timeHeaders = timeHeaders[:len(timeHeaders)-1]
					if skipBaseDir == "" {
						err := os.Chtimes(curDir, timeHeader.Atime, timeHeader.Mtime)
						if err != nil {
							return fmt.Errorf("failed to change directory time: err=%s", err)
						}
					}
				}
				curDir = filepath.Dir(curDir)
				if skipBaseDir != "" {
					var sub bool
					if curDir == "" {
						sub = true
					} else {
						var err error
						sub, err = isSubdirectory(skipBaseDir, curDir)
						if err != nil {
							return fmt.Errorf("failed to check directory is subdirectory: err=%s", err)
						}
					}
					if !sub {
						skipBaseDir = ""
					}
				}
			case fileMsgHeader:
				fileHeader := h.(fileMsgHeader)
				localFilename := filepath.Join(curDir, fileHeader.Name)
				if skipBaseDir == "" {
					info := newFileInfo(localFilename, fileHeader.Size, fileHeader.Mode, timeHeader.Mtime, timeHeader.Atime)
					accepted, err := acceptFn(info)
					if err != nil {
						return fmt.Errorf("error from accessFn: err=%s", err)
					}
					if !accepted {
						continue
					}
					err = copyFileBodyFromRemote(s, localFilename, timeHeader, fileHeader)
					if err != nil {
						return err
					}
				} else {
					err = s.CopyFileBodyTo(fileHeader, ioutil.Discard)
					if err != nil {
						return err
					}
				}
			case okMsg:
				// do nothing
			}
		}
		return nil
	})
}

func isSubdirectory(basepath, targetpath string) (bool, error) {
	rel, err := filepath.Rel(basepath, targetpath)
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(rel, ".."+string([]rune{filepath.Separator})), nil
}

type sinkSession struct {
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

func newSinkSession(client *ssh.Client, remoteSrcPath string, remoteSrcIsDir bool, scpPath string, recursive, updatesPermission bool) (*sinkSession, error) {
	s := &sinkSession{
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

	cmd := s.scpPath + " " + string(opt) + " " + escapeShellArg(s.remoteSrcPath)
	err = s.session.Start(cmd)
	if err != nil {
		return s, err
	}

	s.sinkProtocol, err = newSinkProtocol(s.stdin, s.stdout)
	return s, err
}

func (s *sinkSession) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Close()
}

func (s *sinkSession) Wait() error {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Wait()
}

func runSinkSession(client *ssh.Client, remoteSrcPath string, remoteSrcIsDir bool, scpPath string, recursive, updatesPermission bool, handler func(s *sinkSession) error) error {
	s, err := newSinkSession(client, remoteSrcPath, remoteSrcIsDir, scpPath, recursive, updatesPermission)
	defer s.Close()
	if err != nil {
		return err
	}

	err = handler(s)
	if err != nil {
		return err
	}

	return s.Wait()
}
