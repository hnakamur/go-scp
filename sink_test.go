package scp_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
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
	localDir, err := ioutil.TempDir("", "go-scp-test-local")
	if err != nil {
		t.Fatalf("fail to get tempdir; %s", err)
	}
	defer os.RemoveAll(localDir)

	remoteDir, err := ioutil.TempDir("", "go-scp-test-remote")
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

	localName := "test1.dat"
	remoteName := "dest.dat"
	localPath := filepath.Join(localDir, localName)
	remotePath := filepath.Join(remoteDir, remoteName)
	err = generateRandomFile(localPath)
	if err != nil {
		t.Fatalf("fail to generate local file; %s", err)
	}

	err = scp.CopyFileToRemote(c, localPath, remotePath)
	if err != nil {
		t.Errorf("fail to CopyFileToRemote; %s", err)
	}
	sameFileInfoAndContent(t, remoteDir, localDir, remoteName, localName, false)
}

var (
	testMaxFileSize  = big.NewInt(1024 * 1024)
	testSshdUser     = "user1"
	testSshdPassword = "password1"
	testSshdShell    = "sh"
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
	testSshdKey, err := generateTestSshdKey()
	if err != nil {
		return nil, nil, err
	}
	private, err := ssh.ParsePrivateKey(testSshdKey)
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

func generateTestSshdKey() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	pemdata := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	return pemdata, nil
}

func newTestSshClient(addr string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: testSshdUser,
		Auth: []ssh.AuthMethod{ssh.Password(testSshdPassword)},
	}

	return ssh.Dial("tcp", addr, config)
}

func generateRandomFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	size, err := generateRandomFileSize()
	if err != nil {
		return err
	}
	reader := io.LimitReader(rand.Reader, size)
	_, err = io.Copy(file, reader)
	return err
}

func generateRandomFileSize() (int64, error) {
	n, err := rand.Int(rand.Reader, testMaxFileSize)
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
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
