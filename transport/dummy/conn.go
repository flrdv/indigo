package dummy

import (
	"io"
	"net"
	"time"
)

type NopConn struct {
}

func NewNop() NopConn {
	return NopConn{}
}

func (n NopConn) Read(b []byte) (int, error) {
	return len(b), nil
}

func (n NopConn) Write(b []byte) (int, error) {
	return len(b), nil
}

func (n NopConn) Close() error {
	return nil
}

func (n NopConn) LocalAddr() net.Addr {
	return nil
}

func (n NopConn) RemoteAddr() net.Addr {
	return nil
}

func (n NopConn) SetDeadline(time.Time) error {
	return nil
}

func (n NopConn) SetReadDeadline(time.Time) error {
	return nil
}

func (n NopConn) SetWriteDeadline(time.Time) error {
	return nil
}

type Conn struct {
	closed  bool
	loop    bool
	noWrite bool
	ptr     int
	pending []byte
	Written []byte
	read    [][]byte
}

func New(read ...[]byte) *Conn {
	return &Conn{
		read: read,
	}
}

func (c *Conn) Fetch() (chunk []byte, err error) {
	if c.closed {
		return nil, io.EOF
	}

	if len(c.pending) > 0 {
		chunk, c.pending = c.pending, nil
		return chunk, nil
	}

	if len(c.read) == 0 {
		return nil, io.EOF
	}

	if c.ptr >= len(c.read) {
		if !c.loop {
			return nil, io.EOF
		}

		c.ptr = 0
	}

	c.ptr++
	return c.read[c.ptr-1], nil
}

func (c *Conn) Read(b []byte) (n int, err error) {
	data, err := c.Fetch()
	if err != nil {
		return 0, err
	}

	n = copy(b, data)
	c.pending = data[n:]
	return n, nil
}

func (c *Conn) PreviewRead() []byte {
	if len(c.read) == 0 {
		return nil
	}

	if len(c.pending) > 0 {
		return c.pending
	}

	ptr := c.ptr

	if ptr >= len(c.read) {
		if !c.loop {
			return nil
		}

		ptr = 0
	}

	return c.read[ptr]
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}

	if !c.noWrite {
		c.Written = append(c.Written, b...)
	}

	return len(b), nil
}

func (c *Conn) Close() error {
	c.closed = true
	return nil
}

func (c *Conn) LocalAddr() net.Addr {
	return nil
}

func (c *Conn) RemoteAddr() net.Addr {
	return nil
}

func (c *Conn) SetDeadline(time.Time) error {
	return nil
}

func (c *Conn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *Conn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *Conn) Loop() *Conn {
	c.loop = true
	return c
}

func (c *Conn) Reset() {
	*c = Conn{
		loop: c.loop,
		read: c.read,
	}
}

func (c *Conn) NoWrite() *Conn {
	c.noWrite = true
	return c
}
