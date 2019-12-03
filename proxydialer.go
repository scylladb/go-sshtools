// Copyright (C) 2019 ScyllaDB

package sshtools

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
)

// ProxyDialer is a dialler that allows for proxying connections over SSH.
type ProxyDialer struct {
	config Config
	dial   DialContextFunc

	OnDial      func(host string, err error)
	OnConnClose func(host string)
}

// NewProxyDialer creates an instance of a ProxyDialer that is capable of
// setting up an ssh tunnels.
func NewProxyDialer(config Config, dial DialContextFunc) *ProxyDialer {
	return &ProxyDialer{config: config, dial: dial}
}

// DialContext dials to addr (HOST:PORT) and establishes an SSH connection
// to HOST and then proxies the connection to localhost:PORT.
func (pd *ProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var tun Tunnel

	host, port, err := net.SplitHostPort(addr)
	defer func() {
		if pd.OnDial != nil {
			pd.OnDial(host, err)
		}
	}()

	tun.client, err = pd.dial(ctx, network,
		net.JoinHostPort(host, fmt.Sprint(pd.config.Port)), &pd.config.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "ssh: dial failed")
	}

	for _, h := range []string{"0.0.0.0", host} {
		// This is a local dial and should not hang but if it does http client
		// would end up with "context deadline exceeded" error.
		// To be fixed when used with something else then http client.
		tun.Conn, err = tun.client.Dial(network, net.JoinHostPort(h, port))
		if err == nil {
			break
		}
	}
	if err != nil {
		_ = tun.client.Close()
		return nil, errors.Wrap(err, "ssh: remote dial failed")
	}

	tun.free = func() {
		if pd.OnConnClose != nil {
			pd.OnConnClose(host)
		}
	}

	if pd.config.KeepaliveEnabled() {
		tun.done = make(chan struct{})
		go KeepAlive(tun.client, pd.config.ServerAliveInterval, pd.config.ServerAliveCountMax, tun.done)
	}

	return tun, nil
}
