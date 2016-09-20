package scp_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	scp "github.com/hnakamur/go-scp"
)

func TestFetchFile(t *testing.T) {
	localDir, err := ioutil.TempDir("", "go-scp-TestFetchFile-local")
	if err != nil {
		t.Fatalf("fail to get tempdir; %s", err)
	}
	defer os.RemoveAll(localDir)

	remoteDir, err := ioutil.TempDir("", "go-scp-TestFetchFile-remote")
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

	t.Run("Random sized file", func(t *testing.T) {
		remoteName := "src.dat"
		localName := "dest.dat"
		remotePath := filepath.Join(remoteDir, remoteName)
		localPath := filepath.Join(localDir, localName)
		err = generateRandomFile(remotePath)
		if err != nil {
			t.Fatalf("fail to generate remote file; %s", err)
		}

		err = scp.FetchFile(c, remotePath, localPath)
		if err != nil {
			t.Errorf("fail to FetchFile; %s", err)
		}
		sameFileInfoAndContent(t, localDir, remoteDir, localName, remoteName)
	})

	t.Run("Empty file", func(t *testing.T) {
		remoteName := "src.dat"
		localName := "dest.dat"
		remotePath := filepath.Join(remoteDir, remoteName)
		localPath := filepath.Join(localDir, localName)
		err = generateRandomFileWithSize(remotePath, 0)
		if err != nil {
			t.Fatalf("fail to generate remote file; %s", err)
		}

		err = scp.FetchFile(c, remotePath, localPath)
		if err != nil {
			t.Errorf("fail to FetchFile; %s", err)
		}
		sameFileInfoAndContent(t, localDir, remoteDir, localName, remoteName)
	})
}

func TestFetchDir(t *testing.T) {
	localDir, err := ioutil.TempDir("", "go-scp-TestFetchDir-local")
	if err != nil {
		t.Fatalf("fail to get tempdir; %s", err)
	}
	defer os.RemoveAll(localDir)

	remoteDir, err := ioutil.TempDir("", "go-scp-TestFetchDir-remote")
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

	t.Run("copy all case 1", func(t *testing.T) {
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
		err := generateRandomFiles(remoteDir, entries)
		if err != nil {
			t.Fatalf("fail to generate remote files; %s", err)
		}

		err = scp.FetchDir(c, remoteDir, localDir, nil)
		if err != nil {
			t.Errorf("fail to FetchDir; %s", err)
		}
		sameDirTreeContent(t, remoteDir, localDir)
	})
}
