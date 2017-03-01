// +build !windows

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
	"sort"
	"testing"
	"time"

	scp "github.com/hnakamur/go-scp"
	sshd "github.com/hnakamur/go-sshd"
	"golang.org/x/crypto/ssh"
)

func TestSendFile(t *testing.T) {
	s, l, err := newTestSshdServer()
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

	t.Run("Random sized file", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendFile-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendFile-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		localName := "test1.dat"
		remoteName := "dest.dat"
		localPath := filepath.Join(localDir, localName)
		remotePath := filepath.Join(remoteDir, remoteName)
		err = generateRandomFile(localPath)
		if err != nil {
			t.Fatalf("fail to generate local file; %s", err)
		}

		err = scp.NewSCP(c).SendFile(localPath, remotePath)
		if err != nil {
			t.Errorf("fail to CopyFileToRemote; %s", err)
		}
		sameFileInfoAndContent(t, remoteDir, localDir, remoteName, localName)
	})

	t.Run("Empty file", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendFile-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendFile-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		localName := "test1.dat"
		remoteName := "dest.dat"
		localPath := filepath.Join(localDir, localName)
		remotePath := filepath.Join(remoteDir, remoteName)
		err = generateRandomFileWithSize(localPath, 0)
		if err != nil {
			t.Fatalf("fail to generate local file; %s", err)
		}

		err = scp.NewSCP(c).SendFile(localPath, remotePath)
		if err != nil {
			t.Errorf("fail to SendFile; %s", err)
		}
		sameFileInfoAndContent(t, remoteDir, localDir, remoteName, localName)
	})

	t.Run("Dest is existing dir", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendFile-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendFile-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		localName := "test1.dat"
		localPath := filepath.Join(localDir, localName)
		err = generateRandomFile(localPath)
		if err != nil {
			t.Fatalf("fail to generate local file; %s", err)
		}

		err = scp.NewSCP(c).SendFile(localPath, remoteDir)
		if err != nil {
			t.Errorf("fail to CopyFileToRemote; %s", err)
		}
		sameDirTreeContent(t, localDir, remoteDir)
	})
}

func TestSendDir(t *testing.T) {
	s, l, err := newTestSshdServer()
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

	t.Run("dest dir not exist", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendDir-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		entries := []fileInfo{
			{name: "foo", maxSize: testMaxFileSize, mode: 0644},
			{name: "bar", maxSize: testMaxFileSize, mode: 0600},
			{name: "baz", isDir: true, mode: 0755,
				entries: []fileInfo{
					{name: "foo", maxSize: testMaxFileSize, mode: 0400},
					{name: "hoge", maxSize: testMaxFileSize, mode: 0602},
					{name: "emptyDir", isDir: true, mode: 0500},
				},
			},
		}
		err = generateRandomFiles(localDir, entries)
		if err != nil {
			t.Fatalf("fail to generate local files; %s", err)
		}

		remoteDestDir := filepath.Join(remoteDir, "dest")
		err = scp.NewSCP(c).SendDir(localDir, remoteDestDir, nil)
		if err != nil {
			t.Errorf("fail to SendDir; %s", err)
		}
		sameDirTreeContent(t, localDir, remoteDestDir)
	})

	t.Run("dest dir exists", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendDir-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		entries := []fileInfo{
			{name: "foo", maxSize: testMaxFileSize, mode: 0644},
			{name: "bar", maxSize: testMaxFileSize, mode: 0600},
			{name: "baz", isDir: true, mode: 0755,
				entries: []fileInfo{
					{name: "foo", maxSize: testMaxFileSize, mode: 0400},
					{name: "hoge", maxSize: testMaxFileSize, mode: 0602},
					{name: "emptyDir", isDir: true, mode: 0500},
				},
			},
		}
		err = generateRandomFiles(localDir, entries)
		if err != nil {
			t.Fatalf("fail to generate local files; %s", err)
		}

		err = scp.NewSCP(c).SendDir(localDir, remoteDir, nil)
		if err != nil {
			t.Errorf("fail to SendDir; %s", err)
		}
		localDirBase := filepath.Base(localDir)
		remoteDestDir := filepath.Join(remoteDir, localDirBase)
		sameDirTreeContent(t, localDir, remoteDestDir)
	})

	t.Run("send only files case #1 dest dir exists", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendDir-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		entries := []fileInfo{
			{name: "01_file", maxSize: testMaxFileSize, mode: 0644},
			{name: "02_file", maxSize: testMaxFileSize, mode: 0600},
			{name: "03_dir", isDir: true, mode: 0755,
				entries: []fileInfo{
					{name: "01_file", maxSize: testMaxFileSize, mode: 0600},
					{name: "02_file", maxSize: testMaxFileSize, mode: 0600},
				},
			},
		}
		err = generateRandomFiles(localDir, entries)
		if err != nil {
			t.Fatalf("fail to generate local files; %s", err)
		}

		err = scp.NewSCP(c).SendDir(localDir, remoteDir, func(parentDir string, info os.FileInfo) (bool, error) {
			current := filepath.Join(parentDir, info.Name())
			return localDir == current || (localDir == parentDir && !info.IsDir()), nil
		})
		if err != nil {
			t.Errorf("fail to SendDir; %s", err)
		}
		localDirBase := filepath.Base(localDir)
		remoteDestDir := filepath.Join(remoteDir, localDirBase)
		err = os.RemoveAll(filepath.Join(localDir, "03_dir"))
		if err != nil {
			t.Errorf("fail to remove directory; %s", err)
		}
		sameDirTreeContent(t, localDir, remoteDestDir)
	})

	t.Run("send only files case #2 dest dir exists", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendDir-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		entries := []fileInfo{
			{name: "01_dir", isDir: true, mode: 0755,
				entries: []fileInfo{
					{name: "01_file", maxSize: testMaxFileSize, mode: 0600},
					{name: "02_file", maxSize: testMaxFileSize, mode: 0600},
				},
			},
			{name: "02_file", maxSize: testMaxFileSize, mode: 0644},
			{name: "03_file", maxSize: testMaxFileSize, mode: 0600},
		}
		err = generateRandomFiles(localDir, entries)
		if err != nil {
			t.Fatalf("fail to generate local files; %s", err)
		}

		err = scp.NewSCP(c).SendDir(localDir, remoteDir, func(parentDir string, info os.FileInfo) (bool, error) {
			current := filepath.Join(parentDir, info.Name())
			return localDir == current || (localDir == parentDir && !info.IsDir()), nil
		})
		if err != nil {
			t.Errorf("fail to SendDir; %s", err)
		}
		localDirBase := filepath.Base(localDir)
		remoteDestDir := filepath.Join(remoteDir, localDirBase)
		err = os.RemoveAll(filepath.Join(localDir, "01_dir"))
		if err != nil {
			t.Errorf("fail to remove directory; %s", err)
		}
		sameDirTreeContent(t, localDir, remoteDestDir)
	})
	t.Run("send only files case #3 dest dir not exists", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendDir-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)
		remoteDir = filepath.Join(remoteDir, "dest")

		entries := []fileInfo{
			{name: "01_file", maxSize: testMaxFileSize, mode: 0644},
			{name: "02_file", maxSize: testMaxFileSize, mode: 0600},
			{name: "03_dir", isDir: true, mode: 0755,
				entries: []fileInfo{
					{name: "01_file", maxSize: testMaxFileSize, mode: 0600},
					{name: "02_file", maxSize: testMaxFileSize, mode: 0600},
				},
			},
		}
		err = generateRandomFiles(localDir, entries)
		if err != nil {
			t.Fatalf("fail to generate local files; %s", err)
		}

		err = scp.NewSCP(c).SendDir(localDir, remoteDir, func(parentDir string, info os.FileInfo) (bool, error) {
			current := filepath.Join(parentDir, info.Name())
			return localDir == current || (localDir == parentDir && !info.IsDir()), nil
		})
		if err != nil {
			t.Errorf("fail to SendDir; %s", err)
		}

		err = os.RemoveAll(filepath.Join(localDir, "03_dir"))
		if err != nil {
			t.Errorf("fail to remove directory; %s", err)
		}
		sameDirTreeContent(t, localDir, remoteDir)
	})

	t.Run("send only files case #4 dest dir not exists", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestSendDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestSendDir-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)
		remoteDir = filepath.Join(remoteDir, "dest")

		entries := []fileInfo{
			{name: "01_dir", isDir: true, mode: 0755,
				entries: []fileInfo{
					{name: "01_file", maxSize: testMaxFileSize, mode: 0600},
					{name: "02_file", maxSize: testMaxFileSize, mode: 0600},
				},
			},
			{name: "02_file", maxSize: testMaxFileSize, mode: 0644},
			{name: "03_file", maxSize: testMaxFileSize, mode: 0600},
		}
		err = generateRandomFiles(localDir, entries)
		if err != nil {
			t.Fatalf("fail to generate local files; %s", err)
		}

		err = scp.NewSCP(c).SendDir(localDir, remoteDir, func(parentDir string, info os.FileInfo) (bool, error) {
			current := filepath.Join(parentDir, info.Name())
			return localDir == current || (localDir == parentDir && !info.IsDir()), nil
		})
		if err != nil {
			t.Errorf("fail to SendDir; %s", err)
		}

		err = os.RemoveAll(filepath.Join(localDir, "01_dir"))
		if err != nil {
			t.Errorf("fail to remove directory; %s", err)
		}
		sameDirTreeContent(t, localDir, remoteDir)
	})
}

var (
	testMaxFileSize  = int64(1024 * 1024)
	testSshdUser     = "user1"
	testSshdPassword = "password1"
	testSshdShell    = "sh"
)

func newTestSshdServer() (*sshd.Server, net.Listener, error) {
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

type fileInfo struct {
	name    string
	isDir   bool
	mode    os.FileMode
	maxSize int64
	entries []fileInfo
}

func generateRandomFiles(dir string, entries []fileInfo) error {
	for _, entry := range entries {
		if filepath.Base(entry.name) != entry.name {
			return fmt.Errorf("fileInfo name must not contain path separator; %s", entry.name)
		}
		path := filepath.Join(dir, entry.name)
		if entry.isDir {
			err := os.MkdirAll(path, entry.mode)
			if err != nil {
				return err
			}
			err = generateRandomFiles(path, entry.entries)
			if err != nil {
				return err
			}
		} else {
			err := generateRandomFileWithMaxSizeAndMode(path, entry.maxSize, entry.mode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func generateRandomFile(filename string) error {
	return generateRandomFileWithMaxSizeAndMode(filename, testMaxFileSize, 0644)
}

func generateRandomFileWithMaxSizeAndMode(filename string, maxSize int64, mode os.FileMode) error {
	size, err := generateRandomFileSize(maxSize)
	if err != nil {
		return err
	}

	return generateRandomFileWithSizeAndMode(filename, size, mode)
}

func generateRandomFileWithSize(filename string, size int64) error {
	return generateRandomFileWithSizeAndMode(filename, size, 0666)
}

func generateRandomFileWithSizeAndMode(filename string, size int64, mode os.FileMode) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := io.LimitReader(rand.Reader, size)
	_, err = io.Copy(file, reader)
	return err
}

func generateRandomFileSize(maxSize int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(maxSize))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

func sameDirTreeContent(t *testing.T, gotDir, wantDir string) bool {
	gotNames, err := filepath.Glob(filepath.Join(gotDir, "*"))
	if err != nil {
		t.Fatalf("failed to glob under dir %s; %s", gotDir, err)
	}
	wantNames, err := filepath.Glob(filepath.Join(wantDir, "*"))
	if err != nil {
		t.Fatalf("failed to glob under dir %s; %s", wantDir, err)
	}
	if len(gotNames) != len(wantNames) {
		t.Errorf("unmatch entry count. got:%d, want:%d", len(gotNames), len(wantNames))
		return false
	}
	sort.Strings(gotNames)
	sort.Strings(wantNames)
	for i := 0; i < len(gotNames); i++ {
		gotName := filepath.Base(gotNames[i])
		wantName := filepath.Base(wantNames[i])
		gotPath := filepath.Join(gotDir, gotName)
		wantPath := filepath.Join(wantDir, wantName)
		gotFileInfo, err := os.Stat(gotPath)
		if err != nil {
			t.Fatalf("cannot stat file; %s", err)
			return false
		}
		wantFileInfo, err := os.Stat(wantPath)
		if err != nil {
			t.Fatalf("cannot stat file; %s", err)
			return false
		}

		same := sameDirOrFile(t, gotDir, wantDir, gotFileInfo, wantFileInfo)
		if !same {
			return false
		}

		if gotFileInfo.IsDir() {
			same = sameDirTreeContent(t, filepath.Join(gotDir, gotName), filepath.Join(wantDir, wantName))
			if !same {
				return false
			}
		}
	}

	return false
}

func sameDirOrFile(t *testing.T, gotDir, wantDir string, gotFileInfo, wantFileInfo os.FileInfo) bool {
	if !sameFileInfo(t, gotDir, wantDir, gotFileInfo, wantFileInfo) {
		return false
	}
	if gotFileInfo.IsDir() {
		return true
	}
	return sameFileContent(t, gotDir, wantDir, gotFileInfo.Name(), wantFileInfo.Name())
}

func sameFileInfoAndContent(t *testing.T, gotDir, wantDir, gotFilename, wantFilename string) bool {
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
	return sameFileInfo(t, gotDir, wantDir, gotFileInfo, wantFileInfo) &&
		sameFileContent(t, gotDir, wantDir, gotFilename, wantFilename)
}

func sameFileInfo(t *testing.T, gotDir, wantDir string, gotFileInfo, wantFileInfo os.FileInfo) bool {
	same := true
	if gotFileInfo.Size() != wantFileInfo.Size() {
		t.Errorf("unmatch size. wantDir:%s; gotFileInfo:%d; wantFileInfo:%d", wantDir, gotFileInfo.Size(), wantFileInfo.Size())
		same = false
	}
	if gotFileInfo.Mode() != wantFileInfo.Mode() {
		t.Errorf("unmatch mode. wantDir:%s; gotFileInfo:%s; wantFileInfo:%s", wantDir, gotFileInfo.Mode(), wantFileInfo.Mode())
		same = false
	}
	gotModTime := gotFileInfo.ModTime().Truncate(time.Second)
	wantModTime := wantFileInfo.ModTime().Truncate(time.Second)
	if gotModTime != wantModTime {
		t.Errorf("unmatch modification time. wantDir:%s; gotFileInfo:%s; wantFileInfo:%s", wantDir, gotModTime, wantModTime)
		same = false
	}
	if gotFileInfo.IsDir() != wantFileInfo.IsDir() {
		t.Errorf("unmatch isDir. wantDir:%s; gotFileInfo:%v; wantFileInfo:%v", wantDir, gotFileInfo.IsDir(), wantFileInfo.IsDir())
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
