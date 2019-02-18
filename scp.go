package scp

import "golang.org/x/crypto/ssh"

// SCP is the type for the SCP client.
type SCP struct {
	client *ssh.Client
}

// NewSCP creates the SCP client.
// It is caller's responsibility to call Dial for ssh.Client before
// calling NewSCP and call Close for ssh.Client after using SCP.
func NewSCP(client *ssh.Client) *SCP {
	return &SCP{
		client: client,
	}
}
