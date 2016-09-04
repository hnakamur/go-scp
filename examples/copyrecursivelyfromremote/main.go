package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

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

	destDir := "/tmp/foo"
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		return err
	}

	srcDir := "/tmp/hoge"
	acceptFn := func(info os.FileInfo) (bool, error) {
		accepted := info.Name() != filepath.Join(destDir, ".git")
		fmt.Printf("acceptFn info=%+v, accepted=%v\n", info, accepted)
		return accepted, nil
	}
	return scp.CopyRecursivelyFromRemote(client, srcDir, destDir, acceptFn)
}

func sshAgent() (ssh.AuthMethod, error) {
	agentSock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(agentSock).Signers), nil
}
