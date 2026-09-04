package http1

import (
	"io"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/internal/construct"
	"github.com/indigo-web/indigo/kv"
	"github.com/indigo-web/indigo/transport"
	"github.com/indigo-web/indigo/transport/dummy"
	"github.com/stretchr/testify/require"
)

func getClient(conn net.Conn) *Client {
	return NewClient(transport.NewClient(conn, 0), make([]byte, 1024))
}

func getBody(client *Client, body config.Body) *Body {
	return NewBody(client, body)
}

func getRequestWithBody(chunked bool, body ...[]byte) (*http.Request, *Body) {
	cfg := config.Default()
	conn := dummy.New(body...)
	req := construct.Request(cfg, conn)
	b := getBody(getClient(conn), cfg.Body)
	req.Body = http.NewBody(b)

	var (
		contentLength = 0
		hdrs          http.Headers
	)

	if chunked {
		hdrs = kv.NewFromMap(map[string][]string{
			"Transfer-Encoding": {"chunked"},
		})
	} else {
		for _, b := range body {
			contentLength += len(b)
		}

		hdrs = kv.NewFromMap(map[string][]string{
			"Content-Length": {strconv.Itoa(contentLength)},
		})
	}

	req.Headers = hdrs
	req.ContentLength = contentLength
	req.Chunked = chunked
	req.Body.Reset(req)

	return req, b
}

func readall(b *Body) ([]byte, error) {
	var buff []byte

	for {
		data, err := b.Fetch()
		buff = append(buff, data...)
		switch err {
		case nil:
		case io.EOF:
			return buff, nil
		default:
			return buff, err
		}
	}
}

func TestBody(t *testing.T) {
	t.Run("zero length", func(t *testing.T) {
		req, b := getRequestWithBody(false)
		b.Reset(req)

		data, err := b.Fetch()
		require.EqualError(t, err, io.EOF.Error())
		require.Empty(t, data)
	})

	t.Run("all at once", func(t *testing.T) {
		sample := []byte("Hello, world!")
		request, b := getRequestWithBody(false, sample)
		b.Reset(request)

		actualBody, err := b.Fetch()
		require.EqualError(t, err, io.EOF.Error())
		require.Equal(t, string(sample), string(actualBody))
	})

	t.Run("consecutive data pieces", func(t *testing.T) {
		sample := [][]byte{
			[]byte("Hel"),
			[]byte("lo, "),
			[]byte("wor"),
			[]byte("ld!"),
		}
		bodyString := "Hello, world!"

		req, b := getRequestWithBody(false, sample...)
		b.Reset(req)
		actualBody, err := readall(b)
		require.NoError(t, err)
		require.Equal(t, bodyString, string(actualBody))
	})

	t.Run("distinction", func(t *testing.T) {
		const buffSize = 10
		var (
			first  = strings.Repeat("a", buffSize)
			second = strings.Repeat("b", buffSize)
		)

		conn := dummy.New([]byte(first + second))
		request := construct.Request(config.Default(), dummy.NewNop())
		request.ContentLength = buffSize
		client := getClient(conn)
		b := getBody(client, config.Default().Body)
		b.Reset(request)

		data, err := b.Fetch()
		require.Equal(t, first, string(data))
		require.EqualError(t, err, io.EOF.Error())

		data, err = b.Fetch()
		require.Empty(t, data)
		require.EqualError(t, err, io.EOF.Error())

		require.Equal(t, second, string(client.pending))
	})

	t.Run("too big plain body", func(t *testing.T) {
		data := strings.Repeat("a", 10)
		request, _ := getRequestWithBody(false, []byte(data))
		conn := dummy.New([]byte(data))
		s := config.Default().Body
		s.MaxSize = 9
		b := getBody(getClient(conn), s)
		b.Reset(request)

		_, err := readall(b)
		require.EqualError(t, err, status.ErrBodyTooLarge.Error())
	})
}

func TestBodyReader_Chunked(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		chunked := []byte("7\r\nMozilla\r\n9\r\nDeveloper\r\n7\r\nNetwork\r\n0\r\n\r\n")
		wantBody := "MozillaDeveloperNetwork"
		request, b := getRequestWithBody(true, chunked)
		b.Reset(request)

		actualBody, err := readall(b)
		require.NoError(t, err)
		require.Equal(t, wantBody, string(actualBody))
	})
}

func TestBodyReader_ConnectionClose(t *testing.T) {
	request, b := getRequestWithBody(false, []byte("Hello, "), []byte("world!"))
	request.Connection = "close"
	b.Reset(request)

	actualBody, err := readall(b)
	require.NoError(t, err)
	require.Equal(t, "Hello, world!", string(actualBody))
}
