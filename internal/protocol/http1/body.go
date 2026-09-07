package http1

import (
	"io"
	"math"

	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/status"
)

type Body struct {
	maxLen        uint32
	counter       uint32
	client        *Client
	reader        func(*Body) ([]byte, error)
	chunkedParser chunkedParser
}

func NewBody(client *Client, s config.Body) *Body {
	return &Body{
		reader:        nop,
		client:        client,
		maxLen:        s.MaxSize,
		chunkedParser: newChunkedParser(),
	}
}

func (b *Body) Fetch() ([]byte, error) {
	return b.reader(b)
}

func (b *Body) Reset(request *http.Request) {
	if request.Chunked {
		b.initChunked()
		b.reader = (*Body).readChunked
	} else if request.Connection == "close" {
		b.initEOFReader()
		b.reader = (*Body).readTillEOF
	} else {
		b.initPlain(uint32(request.ContentLength))
		b.reader = (*Body).readPlain
	}
}

func (b *Body) initPlain(totalLen uint32) {
	b.counter = totalLen
}

func (b *Body) readPlain() (body []byte, err error) {
	if b.counter == 0 {
		return nil, io.EOF
	}

	if b.counter > b.maxLen {
		return nil, status.ErrBodyTooLarge
	}

	data, err := b.client.Fetch()
	if err != nil {
		return nil, err
	}

	if uint32(len(data)) >= b.counter {
		body, data = data[:b.counter], data[b.counter:]
		b.client.Pushback(data)
		b.counter = 0
		err = io.EOF
	} else {
		b.counter -= uint32(len(data))
		body = data
	}

	return body, err
}

func (b *Body) initEOFReader() {
	b.counter = 0
}

func (b *Body) readTillEOF() ([]byte, error) {
	chunk, err := b.client.Fetch()
	if b.counter > math.MaxUint32-uint32(len(chunk)) {
		return nil, status.ErrBodyTooLarge
	}

	b.counter += uint32(len(chunk))

	return chunk, err
}

func (b *Body) initChunked() {
	b.counter = 0
}

func (b *Body) readChunked() (body []byte, err error) {
	data, err := b.client.Fetch()
	if err != nil {
		return nil, err
	}

	chunk, extra, err := b.chunkedParser.Parse(data)
	switch err {
	case nil, io.EOF:
	default:
		return nil, err
	}

	if b.counter > math.MaxUint32-uint32(len(chunk)) {
		return nil, status.ErrBodyTooLarge
	}

	b.counter += uint32(len(chunk))
	b.client.Pushback(extra)

	return chunk, err
}

func nop(*Body) ([]byte, error) {
	return nil, io.EOF
}
