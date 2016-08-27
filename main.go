package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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
		defer stdout.Close()

		reader := bufio.NewReader(stdout)

		filename := "test1"
		content := "content1\n"
		fmt.Fprintf(stdin, "C0649 %d %s\n", len(content), filename)
		fmt.Fprintf(stdin, "%s", content)
		fmt.Fprint(stdin, "\x00")

		b, msg, err := readReply(reader)
		if err != nil {
			return err
		}
		fmt.Printf("first b=%v, msg=%s\n", b, msg)

		for i := 0; i < 2; i++ {
			b, msg, err := readReply(reader)
			fmt.Printf("file#1 b=%v, msg=%s\n", b, msg)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if b != ok {
				return errors.New(msg)
			}
		}

		filename = "test2"
		content = "content2\n"
		fmt.Fprintf(stdin, "C0604 %d %s\n", len(content), filename)
		stdin.Write([]byte(content))
		stdin.Write([]byte{'\x00'})

		for i := 0; i < 2; i++ {
			b, msg, err := readReply(reader)
			fmt.Printf("file#2 b=%v, msg=%s\n", b, msg)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if b != ok {
				return errors.New(msg)
			}
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
	ok         = '\x00'
	warning    = '\x01'
	fatalError = '\x02'
)

func readReply(r *bufio.Reader) (b byte, msg string, err error) {
	b, err = r.ReadByte()
	if err != nil {
		return
	}
	if b == ok {
		return
	}
	if b != warning && b != fatalError {
		err = errors.New("unexpected reply type")
		return
	}
	var line []byte
	line, err = r.ReadBytes('\n')
	if err != nil {
		return
	}
	msg = string(line)
	return
}
