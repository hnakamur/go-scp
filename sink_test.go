// +build !windows

package scp_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	scp "github.com/hnakamur/go-scp"
)

func TestReceiveFile(t *testing.T) {
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
		localDir, err := ioutil.TempDir("", "go-scp-TestReceiveFile-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestReceiveFile-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		remoteName := "src.dat"
		localName := "dest.dat"
		remotePath := filepath.Join(remoteDir, remoteName)
		localPath := filepath.Join(localDir, localName)
		err = generateRandomFile(remotePath)
		if err != nil {
			t.Fatalf("fail to generate remote file; %s", err)
		}

		err = scp.NewSCP(c).ReceiveFile(remotePath, localPath)
		if err != nil {
			t.Errorf("fail to ReceiveFile; %s", err)
		}
		sameFileInfoAndContent(t, localDir, remoteDir, localName, remoteName)
	})

	t.Run("Empty file", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestReceiveFile-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestReceiveFile-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		remoteName := "src.dat"
		localName := "dest.dat"
		remotePath := filepath.Join(remoteDir, remoteName)
		localPath := filepath.Join(localDir, localName)
		err = generateRandomFileWithSize(remotePath, 0)
		if err != nil {
			t.Fatalf("fail to generate remote file; %s", err)
		}

		err = scp.NewSCP(c).ReceiveFile(remotePath, localPath)
		if err != nil {
			t.Errorf("fail to ReceiveFile; %s", err)
		}
		sameFileInfoAndContent(t, localDir, remoteDir, localName, remoteName)
	})

	t.Run("Dest is existing dir", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestReceiveFile-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestReceiveFile-remote")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(remoteDir)

		remoteName := "src.dat"
		remotePath := filepath.Join(remoteDir, remoteName)
		err = generateRandomFileWithSize(remotePath, 0)
		if err != nil {
			t.Fatalf("fail to generate remote file; %s", err)
		}

		err = scp.NewSCP(c).ReceiveFile(remotePath, localDir)
		if err != nil {
			t.Errorf("fail to ReceiveFile; %s", err)
		}
		sameDirTreeContent(t, remoteDir, localDir)
	})
}

func TestReceiveDir(t *testing.T) {
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
		localDir, err := ioutil.TempDir("", "go-scp-TestReceiveDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestReceiveDir-remote")
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
		err = generateRandomFiles(remoteDir, entries)
		if err != nil {
			t.Fatalf("fail to generate remote files; %s", err)
		}

		localDestDir := filepath.Join(localDir, "dest")
		err = scp.NewSCP(c).ReceiveDir(remoteDir, localDestDir, nil)
		if err != nil {
			t.Errorf("fail to ReceiveDir; %s", err)
		}
		sameDirTreeContent(t, remoteDir, localDestDir)
	})

	t.Run("dest dir exists", func(t *testing.T) {
		localDir, err := ioutil.TempDir("", "go-scp-TestReceiveDir-local")
		if err != nil {
			t.Fatalf("fail to get tempdir; %s", err)
		}
		defer os.RemoveAll(localDir)

		remoteDir, err := ioutil.TempDir("", "go-scp-TestReceiveDir-remote")
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
		err = generateRandomFiles(remoteDir, entries)
		if err != nil {
			t.Fatalf("fail to generate remote files; %s", err)
		}

		err = scp.NewSCP(c).ReceiveDir(remoteDir, localDir, nil)
		if err != nil {
			t.Errorf("fail to ReceiveDir; %s", err)
		}
		remoteDirBase := filepath.Base(remoteDir)
		localDestDir := filepath.Join(localDir, remoteDirBase)
		sameDirTreeContent(t, remoteDir, localDestDir)
	})
}
