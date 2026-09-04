package http1

import (
	"github.com/indigo-web/indigo/transport"
)

type Client struct {
	transport.Client

	buff    []byte
	pending []byte
}

func NewClient(underlying transport.Client, buff []byte) *Client {
	return &Client{
		Client: underlying,
		buff:   buff,
	}
}

func (c *Client) Fetch() ([]byte, error) {
	if len(c.pending) > 0 {
		pending := c.pending
		c.pending = nil

		return pending, nil
	}

	n, err := c.Client.Read(c.buff)
	return c.buff[:n], err
}

func (c *Client) Pushback(data []byte) {
	c.pending = data
}
