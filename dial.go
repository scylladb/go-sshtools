// Copyright (C) 2017 ScyllaDB

package sshtools

import (
	"context"
	"net"

	"golang.org/x/crypto/ssh"
)

// DialContextFunc creates SSH connection to host with a given address.
//
// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only),
// "udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4"
// (IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and
// "unixpacket". For more info see net.Dial.
//
// For TCP and UDP networks, the addr has the form "host:port".
// The host must be a literal IP address, or a host name that can be
// resolved to IP addresses.
// The port must be a literal port number or a service name.
// If the host is a literal IPv6 address it must be enclosed in square
// brackets, as in "[2001:db8::1]:80" or "[fe80::1%zone]:80".
type DialContextFunc func(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)

// ContextDialer returns DialContextFunc based on dialer to make net connections.
func ContextDialer(dialer *net.Dialer) DialContextFunc {
	return contextDialer{dialer}.DialContext
}

type contextDialer struct {
	dialer *net.Dialer
}

func (d contextDialer) DialContext(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := d.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	var client *ssh.Client
	var wait = make(chan struct{})
	go func() {
		sshConn, ch, rs, sshErr := ssh.NewClientConn(conn, addr, config)
		if sshErr == nil {
			client = ssh.NewClient(sshConn, ch, rs)
		}
		err = sshErr
		close(wait)
	}()

	select {
	// write to (client, err) happens-before <-wait succeed
	case <-wait:
		if err != nil {
			_ = conn.Close()
		}
		return client, err
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	}
}
