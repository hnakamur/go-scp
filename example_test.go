package scp_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	scp "github.com/hnakamur/go-scp"
	sshd "github.com/hnakamur/go-sshd"
	"golang.org/x/crypto/ssh"
)

func Example() {
	generateTestSshdKey := func() ([]byte, error) {
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

	newTestSshdServer := func() (*sshd.Server, net.Listener, error) {
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

	s, l, err := newTestSshdServer()
	if err != nil {
		log.Fatalf("fail to create test sshd server; %s", err)
	}
	defer s.Close()
	go s.Serve(l)

	config := &ssh.ClientConfig{
		User:            testSshdUser,
		Auth:            []ssh.AuthMethod{ssh.Password(testSshdPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", l.Addr().String(), config)
	if err != nil {
		log.Fatalf("failed to dial to ssh server: %s", err)
	}
	defer client.Close()

	localDir, err := ioutil.TempDir("", "go-scp-ExampleSendFile-local")
	if err != nil {
		log.Fatalf("fail to get tempdir; %s", err)
	}
	defer os.RemoveAll(localDir)

	remoteDir, err := ioutil.TempDir("", "go-scp-ExampleSendFile-remote")
	if err != nil {
		log.Fatalf("fail to get tempdir; %s", err)
	}
	defer os.RemoveAll(remoteDir)

	localName := "test1.dat"
	remoteName := "dest.dat"
	localPath := filepath.Join(localDir, localName)
	remotePath := filepath.Join(remoteDir, remoteName)
	content := []byte("Hello, SCP\n")
	err = ioutil.WriteFile(localPath, content, 0644)
	if err != nil {
		log.Fatalf("fail to write file; %s", err)
	}

	err = scp.NewSCP(client).SendFile(localPath, remotePath)
	if err != nil {
		log.Fatalf("fail to send file; %s", err)
	}

	data, err := ioutil.ReadFile(remotePath)
	if err != nil {
		log.Fatalf("fail to read file; %s", err)
	}

	fmt.Printf("%s", data)

	// Output: Hello, SCP
}
