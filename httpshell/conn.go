// Copyright (C) 2017 ScyllaDB

package httpshell

import (
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

// proxyConn is a net.Conn that writes to the SSH shell stdin and reads from
// the SSH shell stdout.
type proxyConn struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader

	free func()
}

// newProxyConn opens a new session and start the shell. When the connection is
// closed the client is closed and the free function is called.
func newProxyConn(client *ssh.Client, stderr io.Writer, free func()) (*proxyConn, error) {
	// Open new session to the agent
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	// Get a pipe to stdin so that we can send data down
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	// Get a pipe to stdout so that we can get responses back
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Set stderr
	session.Stderr = stderr

	// Start the shell on the other side
	if err := session.Shell(); err != nil {
		return nil, err
	}

	return &proxyConn{
		client:  client,
		session: session,
		stdin:   stdin,
		stdout:  stdout,
		free:    free,
	}, nil
}

func (conn *proxyConn) Read(b []byte) (n int, err error) {
	return conn.stdout.Read(b)
}

func (conn *proxyConn) Write(b []byte) (n int, err error) {
	return conn.stdin.Write(b)
}

func (conn *proxyConn) Close() error {
	var err error
	err = multierr.Append(err, conn.session.Close())
	err = multierr.Append(err, conn.client.Close())
	if conn.free != nil {
		conn.free()
	}
	return err
}

func (conn *proxyConn) LocalAddr() net.Addr {
	return conn.client.LocalAddr()
}

func (conn *proxyConn) RemoteAddr() net.Addr {
	return conn.client.RemoteAddr()
}

func (*proxyConn) SetDeadline(t time.Time) error {
	return errors.New("ssh: deadline not supported")
}

func (*proxyConn) SetReadDeadline(t time.Time) error {
	return errors.New("ssh: deadline not supported")
}

func (*proxyConn) SetWriteDeadline(t time.Time) error {
	return errors.New("ssh: deadline not supported")
}
