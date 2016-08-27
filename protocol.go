package scp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

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
	return &ProtocolError{
		msg:   string(line),
		fatal: b == replyFatalError,
	}
}
