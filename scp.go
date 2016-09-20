package scp

import "golang.org/x/crypto/ssh"

type SCP struct {
	client *ssh.Client
}

func NewSCP(client *ssh.Client) *SCP {
	return &SCP{
		client: client,
	}
}
