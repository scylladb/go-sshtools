// Copyright (C) 2019 ScyllaDB

package sshtools

import (
	"net"

	"golang.org/x/crypto/ssh"
)

// Tunnel holds an ssh tunnel to some resource on remote machine
type Tunnel struct {
	net.Conn

	client *ssh.Client

	done chan struct{}
	free func()
}

var _ net.Conn = (*Tunnel)(nil)

// Client returns ssh client instance.
func (c Tunnel) Client() *ssh.Client {
	return c.client
}

// Close closes the connection and frees the associated resources.
func (c Tunnel) Close() error {
	defer c.free()

	if c.done != nil {
		close(c.done)
	}

	// Close closes the underlying network connection.
	return c.client.Close()
}
