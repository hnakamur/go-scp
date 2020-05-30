package scp

import "golang.org/x/crypto/ssh"

// SCP is the type for the SCP client.
type SCP struct {
	client *ssh.Client
	// Alternate scp command. If not set, scp is used. This can be used
	// to call scp via sudo by setting it to "sudo scp"
	SCPCommand string
}

// NewSCP creates the SCP client.
// It is caller's responsibility to call Dial for ssh.Client before
// calling NewSCP and call Close for ssh.Client after using SCP.
func NewSCP(client *ssh.Client) *SCP {
	return &SCP{
		client: client,
	}
}
