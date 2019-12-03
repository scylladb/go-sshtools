// Copyright (C) 2017 ScyllaDB

package httpshell

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/scylladb/go-sshtools"
)

// Dialer allows for proxying connections over SSH. It can be used with HTTP
// client to allow communication with a HTTP shell using Listener and serving
// HTTP request over stdin and stdout.
type Dialer struct {
	config sshtools.Config
	dial   sshtools.DialContextFunc
	logger sshtools.Logger

	// OnDial is a listener that may be set to track openning SSH connection to
	// the remote host. It is called for both successful and failed trials.
	OnDial func(host string, err error)
	// OnConnClose is a listener that may be set to track closing of SSH
	// connection.
	OnConnClose func(host string)
}

func NewDialer(config sshtools.Config, dial sshtools.DialContextFunc, logger sshtools.Logger) *Dialer {
	return &Dialer{
		config: config,
		dial:   dial,
		logger: logger,
	}
}

// DialContext to addr HOST:PORT establishes an SSH connection to HOST and then
// sends request to the SSH shell.
func (p *Dialer) DialContext(ctx context.Context, network, addr string) (conn net.Conn, err error) {
	host, _, _ := net.SplitHostPort(addr) // nolint: errcheck

	defer func() {
		if p.OnDial != nil {
			p.OnDial(host, err)
		}
	}()

	p.logger.Println("Connecting to remote host...", "host", host)

	client, err := p.dial(ctx, network, net.JoinHostPort(host, fmt.Sprint(p.config.Port)), &p.config.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "ssh: dial failed")
	}

	p.logger.Println("Starting session", "host", host)

	keepaliveDone := make(chan struct{})
	free := func() {
		close(keepaliveDone)
		if p.OnConnClose != nil {
			p.OnConnClose(host)
		}
		p.logger.Println("Connection closed", "host", host)
	}

	pconn, err := newProxyConn(client, &logStderr{host: host, logger: p.logger}, free)
	if err != nil {
		_ = client.Close()
		return nil, errors.Wrap(err, "ssh: failed to connect")
	}

	p.logger.Println("Connected!", "host", host)

	// Init SSH keepalive if needed
	if p.config.KeepaliveEnabled() {
		p.logger.Println("Starting ssh KeepAlives", "host", host)
		go sshtools.KeepAlive(client, p.config.ServerAliveInterval, p.config.ServerAliveCountMax, keepaliveDone)
	}

	return pconn, nil
}

type logStderr struct {
	host   string
	logger sshtools.Logger
}

func (w *logStderr) Write(p []byte) (n int, err error) {
	w.logger.Println("host", w.host, "stderr", string(p))
	return len(p), nil
}
