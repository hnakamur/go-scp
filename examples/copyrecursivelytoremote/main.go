package main

import (
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

	destDir := "/tmp/hoge"
	err = runCommand(client, "mkdir -p "+scp.EscapeShellArg(destDir))
	if err != nil {
		return err
	}

	srcDir := filepath.Join(os.Getenv("HOME"), "gocode", "src", "bitbucket.org", "hnakamur", "scp")
	//walkFn := func(path string, info os.FileInfo, err error) error {
	//	if filepath.Base(path) == ".git" {
	//		return filepath.SkipDir
	//	}
	//	return nil
	//}
	return scp.CopyRecursivelyToRemote(client, srcDir, destDir, nil)
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
