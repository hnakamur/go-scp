package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	cmd := exec.Command("scp", "-t", "/tmp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = func() error {
		defer stdin.Close()

		s, err := newSource(stdin, stdout)
		if err != nil {
			return err
		}

		mode := os.FileMode(0644)
		filename := "test1"
		content := "content1\n"
		err = s.writeFile(mode, int64(len(content)), filename, bytes.NewBufferString(content))
		if err != nil {
			return err
		}

		mode = os.FileMode(0406)
		filename = "test2"
		content = ""
		err = s.writeFile(mode, int64(len(content)), filename, bytes.NewBufferString(content))
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

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

func (s *source) writeFile(mode os.FileMode, size int64, name string, body io.Reader) error {
	_, err := fmt.Fprintf(s.remIn, "C%#4o %d %s\n", mode, size, name)
	if err != nil {
		return fmt.Errorf("failed to write scp file header: err=%s", err)
	}
	_, err = io.Copy(s.remIn, body)
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
	err = s.readReply()
	if err != nil {
		return err
	}

	return nil
}

type SCPError struct {
	msg   string
	fatal bool
}

func (e *SCPError) Error() string { return e.msg }
func (e *SCPError) Fatal() bool   { return e.fatal }

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
	return &SCPError{
		msg:   string(line),
		fatal: b == replyFatalError,
	}
}
