package main

import (
	"bytes"
	"io/ioutil"
	"net"
	"os"
	"time"

	"bitbucket.org/hnakamur/scp"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	auth, err := sshAgent()
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{auth},
	}

	client, err := ssh.Dial("tcp", "10.155.92.21:22", config)
	if err != nil {
		return err
	}
	defer client.Close()

	destDir := "/tmp"
	copier := func(s *scp.Source) error {
		mode := os.FileMode(0644)
		filename := "test1"
		content := "content1\n"
		modTime := time.Date(2006, 1, 2, 15, 04, 05, 678901000, time.Local)
		accessTime := time.Date(2018, 8, 31, 23, 59, 58, 999999000, time.Local)
		fi := scp.NewFileInfo(filename, int64(len(content)), mode, modTime, accessTime)
		err = s.WriteFile(fi, ioutil.NopCloser(bytes.NewBufferString(content)))
		if err != nil {
			return err
		}

		di := scp.NewDirInfo("test2", os.FileMode(0755), time.Time{}, time.Time{})
		err = s.StartDirectory(di)
		if err != nil {
			return err
		}

		di = scp.NewDirInfo("sub", os.FileMode(0750), time.Time{}, time.Time{})
		err = s.StartDirectory(di)
		if err != nil {
			return err
		}

		mode = os.FileMode(0604)
		filename = "test2"
		content = ""
		fi = scp.NewFileInfo(filename, int64(len(content)), mode, time.Time{}, time.Time{})
		err = s.WriteFile(fi, ioutil.NopCloser(bytes.NewBufferString(content)))
		if err != nil {
			return err
		}

		err = s.EndDirectory()
		if err != nil {
			return err
		}

		return s.EndDirectory()
	}
	return scp.NewSource(client, destDir, "", true, true).CopyToRemote(copier)
}

func sshAgent() (ssh.AuthMethod, error) {
	agentSock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(agentSock).Signers), nil
}
