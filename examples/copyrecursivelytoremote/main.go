package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/hnakamur/scp"

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

	destDir := "/tmp/hoge"
	err = runCommand(client, fmt.Sprintf("mkdir -p %s", destDir))
	if err != nil {
		return err
	}

	srcDir := filepath.Join(os.Getenv("HOME"), "gocode", "src", "github.com", "hnakamur", "scp")
	return scp.CopyRecursivelyToRemote(client, srcDir, destDir, nil)
	//return scp.CopyRecursivelyToRemote(client, srcDir, destDir, func(dir string, info os.FileInfo) (bool, error) {
	//	return dir != srcDir || info.Name() != ".git", nil
	//})
}

func runCommand(client *ssh.Client, cmd string) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	return sess.Run(cmd)
}

func sshAgent() (ssh.AuthMethod, error) {
	agentSock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(agentSock).Signers), nil
}
