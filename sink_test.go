package scp_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	scp "github.com/hnakamur/go-scp"
	sshd "github.com/hnakamur/go-sshd"
	"golang.org/x/crypto/ssh"
)

func TestCopyFileToRemote(t *testing.T) {
	remoteDir, err := ioutil.TempDir("", "go-scp-test")
	if err != nil {
		t.Fatalf("fail to get tempdir; %s", err)
	}
	defer os.RemoveAll(remoteDir)

	s, l, err := newTestSshdServer(remoteDir)
	if err != nil {
		t.Fatalf("fail to create test sshd server; %s", err)
	}
	defer s.Close()
	go s.Serve(l)

	c, err := newTestSshClient(l.Addr().String())
	if err != nil {
		t.Fatalf("fail to serve test sshd server; %s", err)
	}
	defer c.Close()

	localDir := "."
	localName := "sink_test.go"
	remoteName := "dest_file.go"
	localPath := filepath.Join(localDir, localName)
	remotePath := filepath.Join(remoteDir, remoteName)
	err = scp.CopyFileToRemote(c, localPath, remotePath)
	if err != nil {
		t.Errorf("fail to CopyFileToRemote; %s", err)
	}
	sameFileInfoAndContent(t, remoteDir, localDir, remoteName, localName, false)
}

var (
	testSshdUser     = "user1"
	testSshdPassword = "password1"
	testSshdShell    = "sh"
	testSshdKey      = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEArs5yTtou7fbBsuAUpzoeo1tDqL8eNAaINY3e/1FZuxpOSpw2
JGx5bFOgpoKErmnpNbP/0XnaqwB0axagyrz9+n9VeK7oUGzYpaLhMD0vuJcMkO19
cLFkeGy/9IYB+T18tu9U3wM2Y79fFS+2TbLFP8LDmnKbO4l8NL6b2dwgej2NX3jH
MiFaZBwPd8bjy71gt68klmCXcokzFeus0e9/3cSENlHgKH6pETYsalWS2W2jZT2I
cpe+rSC9zJScTifJrXUTc106pHcg1/lGoUP0izHu7dDomr1f3l9MqESikR57Wn08
YmtfW5suNty5qKWQE/DG1R8F+N0suKwndvZEMQIDAQABAoIBAAyYT2AjFFKM/vPW
dWZ/J4n5n2xfKNvyxImnSTP4NpDmmlGB49zH/A+0DtUwfkLa2rTF3V7IetrrU3gL
z1YMO/h6iDwDzjVNQmbcz4DcR73zFDK1Cd6+yVBr9YC2zxmYNo4vvFu9LYQOW9l4
8Je0k8W+UL2mGE034L0kJrKRy71C55rO1QYca+O0Ykmtkn4U5jyPEI2Xj19r6BHJ
43CjPdIkr80QFnJWhBgJy5j5S740ZBjuC4mdaNUS+9U/cITe4zxYetLTOBZb/BJO
M1kkGRbwj2SiDyX0l9xs7QzVUjP26DoN91ifIgKx/DYszO+8CVg0Q8uEHUBlbR0/
dPSFXIECgYEA06snJVHoiXu7GM5tTA1aXkqySckDHSYxOPzdFpLaRWoMkhhLhyDw
Z8fCF+uP8eotlewefX9NIvUazZ2XXZcJauORY3Tcy6wvKGNLW5ju5848Bwy8jT+G
sKYij5LGgt8kMlA2c/ULSQTgyIafhCgroDvLKkyoGvWkV1YQb+IKxPkCgYEA02rj
lh1MHcbcB8o2IJlyEFV/ICkytFzdJ6eI0+nt88o3kbzg92uxCYZJtEb9wX2AVZ1v
57q/r9w/krSc3VRfUDE8wJ8VCidxlXHYcN6CQILA3bsM/t2q0kuacwpN7OGEhX2+
Fj9EZ1gjTpWehIRveCBF7Fkxzq9es6CZNlYDnvkCgYAkopfo5q9XtFmipn/WTO1a
KpWHHcpzLhwQ3/soIAy1PPCmDJxt6+6QF8vpNfU5Cq4PJ8nzMKhaJ5AXDHKZWT3h
CTgtvZlFiyyyUdVGKkcXSeOr2LF9xQP76RVMQjwnhJWQO7/g/AWTAswhCOPtDMLY
PeEhFhl2aROjphq8MqRoiQKBgQCezKjJtpPXwei/iSmC7v74Od/U/lzxkNck0/g4
hHuRJJD8zMyFy8QcjVuLJ8+uqF/e7vSBMIqOw3aU8UjqDlfRWkpxvIwHJn1wbSTQ
ErHvVscbRUaLoWCPuO33/wNtLC9oPXysJTVyEofinQuGKhu4NTWQQ6bfwmX1smmi
oJTzsQKBgQCVK54GpR7fbLmA6zIYoqUmLmspipoUBKCeklHRpBdqy7iuJlXwPu7U
p61z/WNeBWrJC3fyatSWzhpWSDXLSfFkL7kJ+ebFVt8V9KZZjaSfFQNa1hUhk8rf
aHZHA1fKg5f7iECmHAW+jyaB7iRW1cSwRDs002kApRAqMUZ9IU0/Iw==
-----END RSA PRIVATE KEY-----
`
)

func newTestSshdServer(dir string) (*sshd.Server, net.Listener, error) {
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == testSshdUser && string(pass) == testSshdPassword {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	private, err := ssh.ParsePrivateKey([]byte(testSshdKey))
	if err != nil {
		return nil, nil, err
	}

	config.AddHostKey(private)

	server := sshd.NewServer(testSshdShell, config, nil)
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, nil, err
	}
	return server, l, err
}

func newTestSshClient(addr string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: testSshdUser,
		Auth: []ssh.AuthMethod{ssh.Password(testSshdPassword)},
	}

	return ssh.Dial("tcp", addr, config)
}

func sameFileInfoAndContent(t *testing.T, gotDir, wantDir, gotFilename, wantFilename string, compareName bool) bool {
	wantPath := filepath.Join(wantDir, wantFilename)
	wantFileInfo, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("fail to stat file %s; %s", wantPath, err)
	}
	gotPath := filepath.Join(gotDir, gotFilename)
	gotFileInfo, err := os.Stat(gotPath)
	if err != nil {
		t.Fatalf("fail to stat file %s; %s", gotPath, err)
	}
	return sameFileInfo(t, gotDir, wantDir, gotFileInfo, wantFileInfo, compareName) &&
		sameFileContent(t, gotDir, wantDir, gotFilename, wantFilename)
}

func sameFileInfo(t *testing.T, gotDir, wantDir string, got, want os.FileInfo, compareName bool) bool {
	same := true
	if compareName && got.Name() != want.Name() {
		t.Errorf("unmatch name. wantDir:%s; got:%s; want:%s", wantDir, got.Name(), want.Name())
		same = false
	}
	if got.Size() != want.Size() {
		t.Errorf("unmatch size. wantDir:%s; got:%d; want:%d", wantDir, got.Size(), want.Size())
		same = false
	}
	if got.Mode() != want.Mode() {
		t.Errorf("unmatch mode. wantDir:%s; got:%s; want:%s", wantDir, got.Mode(), want.Mode())
		same = false
	}
	gotModTime := got.ModTime().Round(time.Second)
	wantModTime := want.ModTime().Round(time.Second)
	if gotModTime != wantModTime {
		t.Errorf("unmatch modification time. wantDir:%s; got:%s; want:%s", wantDir, gotModTime, wantModTime)
		same = false
	}
	if got.IsDir() != want.IsDir() {
		t.Errorf("unmatch isDir. wantDir:%s; got:%v; want:%v", wantDir, got.IsDir(), want.IsDir())
		same = false
	}
	return same
}

func sameFileContent(t *testing.T, gotDir, wantDir, gotFilename, wantFilename string) bool {
	wantPath := filepath.Join(wantDir, wantFilename)
	wantFile, err := os.Open(wantPath)
	if err != nil {
		t.Fatalf("failed to open file %s; %s", wantPath, err)
	}
	defer wantFile.Close()

	gotPath := filepath.Join(gotDir, gotFilename)
	gotFile, err := os.Open(gotPath)
	if err != nil {
		t.Fatalf("failed to open file %s; %s", gotPath, err)
	}
	defer gotFile.Close()

	wantBuf := make([]byte, 4096)
	gotBuf := make([]byte, 4096)
	for {
		n, err := wantFile.Read(wantBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read file %s; %s", wantPath, err)
		}
		_, err = io.ReadFull(gotFile, gotBuf[:n])
		if err != nil {
			t.Fatalf("failed to read file %s; %s", gotPath, err)
		}
		if !bytes.Equal(gotBuf[:n], wantBuf[:n]) {
			t.Errorf("unmatch file content. wantDir:%s, got:%s, want:%s", wantDir, gotFilename, wantFilename)
			return false
		}
	}
	return true
}
