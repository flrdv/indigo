package transport

import (
	"net"
	"time"

	"github.com/indigo-web/indigo/internal/timer"
)

// Client inherits net.Conn and adds automatic timeout handling.
type Client struct {
	net.Conn

	timeout time.Duration
}

func NewClient(conn net.Conn, timeout time.Duration) Client {
	return Client{
		Conn:    conn,
		timeout: timeout,
	}
}

func (c Client) Read(b []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(timer.Now().Add(c.timeout)); err != nil {
		return 0, err
	}

	return c.Conn.Read(b)
}
