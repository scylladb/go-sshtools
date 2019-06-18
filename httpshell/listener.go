// Copyright (C) 2017 ScyllaDB

package httpshell

import (
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
)

// Listener is a net.Listener that accepts only a single connection that uses
// the given reader and writer. It's intended to be used with http.Server,
// it can then expose http.Handlers over all sorts of transports.
//
// After accepting the first connection any calls to Accept will block until
// the connection is closed, then they will end immediately with io.EOF error.
// This is needed to block http.Server main loop and avoid termination of
// the golden connection.
type Listener struct {
	w    io.Writer
	r    io.ReadCloser
	done chan struct{}
}

func NewListener(w io.Writer, r io.ReadCloser) *Listener {
	return &Listener{w: w, r: r}
}

func (l *Listener) Accept() (net.Conn, error) {
	if l.done != nil {
		// Block the http.Server main loop and wait for the connection to end
		<-l.done
		return nil, io.EOF
	}

	// Return the connection consuming the reader and writer.
	l.done = make(chan struct{})
	return &conn{
		w:    l.w,
		r:    l.r,
		done: l.done,
	}, nil
}

func (l *Listener) Close() error {
	return errors.New("agent: closing Listener is not supported")
}

func (l *Listener) Addr() net.Addr {
	return nilAddr
}

// conn is a net.Conn that uses given reader and writer.
// It should be only used by Listener.
type conn struct {
	w    io.Writer
	r    io.ReadCloser
	done chan struct{}
}

func (c *conn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

func (c *conn) Write(b []byte) (n int, err error) {
	return c.w.Write(b)
}

func (c *conn) Close() error {
	defer func() {
		if c.done != nil {
			close(c.done)
			c.done = nil
		}
	}()
	return c.r.Close()
}

func (*conn) LocalAddr() net.Addr {
	return nilAddr
}

func (*conn) RemoteAddr() net.Addr {
	return nilAddr
}

func (*conn) SetDeadline(t time.Time) error {
	return errors.New("agent: deadline not supported")
}

func (*conn) SetReadDeadline(t time.Time) error {
	return errors.New("agent: deadline not supported")
}

func (*conn) SetWriteDeadline(t time.Time) error {
	return errors.New("agent: deadline not supported")
}

// addr is a mock net.Addr.
type addr string

func (addr) Network() string {
	return "tcp"
}

func (a addr) String() string {
	return string(a)
}

var nilAddr = addr("127.0.0.1")
