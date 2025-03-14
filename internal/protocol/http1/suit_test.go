package http1

import (
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/headers"
	"github.com/indigo-web/indigo/internal/requestgen"
	"github.com/indigo-web/indigo/router/simple"
	"github.com/indigo-web/indigo/transport/dummy"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func newSimpleRouter(t *testing.T, want headers.Headers) *simple.Router {
	return simple.New(func(request *http.Request) *http.Response {
		require.True(t, compareHeaders(want, request.Headers))
		return http.Respond(request)
	}, func(request *http.Request) *http.Response {
		require.Failf(t, "unexpected error", "unexpected error: %s", request.Env.Error.Error())
		return nil
	})
}

func TestServer(t *testing.T) {
	const N = 10

	t.Run("simple get", func(t *testing.T) {
		raw := []byte("GET / HTTP/1.1\r\nAccept-Encoding: identity\r\n\r\n")
		client := dummy.NewCircularClient(raw)
		server, _ := newSuit(client)
		wantHeaders := headers.New().Add("Accept-Encoding", "identity")
		server.router = newSimpleRouter(t, wantHeaders)

		for i := 0; i < N; i++ {
			require.True(t, server.ServeOnce())
		}
	})

	t.Run("5 headers", func(t *testing.T) {
		wantHeaders := requestgen.Headers(5)
		raw := requestgen.Generate(longPath, wantHeaders)
		dispersed := disperse(raw, config.Default().NET.ReadBufferSize)
		client := dummy.NewCircularClient(dispersed...)
		server, _ := newSuit(client)
		server.router = newSimpleRouter(t, wantHeaders)

		for i := 0; i < N; i++ {
			require.True(t, server.ServeOnce())
		}
	})

	t.Run("10 headers", func(t *testing.T) {
		wantHeaders := requestgen.Headers(10)
		raw := requestgen.Generate(longPath, wantHeaders)
		dispersed := disperse(raw, config.Default().NET.ReadBufferSize)
		client := dummy.NewCircularClient(dispersed...)
		server, _ := newSuit(client)
		server.router = newSimpleRouter(t, wantHeaders)

		for i := 0; i < N; i++ {
			require.True(t, server.ServeOnce())
		}
	})

	t.Run("50 headers", func(t *testing.T) {
		wantHeaders := requestgen.Headers(50)
		raw := requestgen.Generate(longPath, wantHeaders)
		dispersed := disperse(raw, config.Default().NET.ReadBufferSize)
		client := dummy.NewCircularClient(dispersed...)
		server, _ := newSuit(client)
		server.router = newSimpleRouter(t, wantHeaders)

		for i := 0; i < N; i++ {
			for j := 0; j < len(dispersed); j++ {
				require.True(t, server.ServeOnce())
			}
		}
	})

	t.Run("heavily escaped", func(t *testing.T) {
		wantHeaders := requestgen.Headers(20)
		raw := requestgen.Generate(strings.Repeat("%20", 500), wantHeaders)
		dispersed := disperse(raw, config.Default().NET.ReadBufferSize)
		client := dummy.NewCircularClient(dispersed...)
		server, _ := newSuit(client)
		server.router = newSimpleRouter(t, wantHeaders)

		for i := 0; i < N; i++ {
			for j := 0; j < len(dispersed); j++ {
				require.True(t, server.ServeOnce())
			}
		}
	})
}

func TestPOST(t *testing.T) {
	const N = 10

	t.Run("POST hello world", func(t *testing.T) {
		raw := []byte("POST / HTTP/1.1\r\nContent-Length: 13\r\n\r\nHello, world!")
		client := dummy.NewCircularClient(disperse(raw, config.Default().NET.ReadBufferSize)...)
		server, _ := newSuit(client)

		for i := 0; i < N; i++ {
			require.True(t, server.ServeOnce())
		}
	})

	t.Run("discard POST 10mib", func(t *testing.T) {
		body := strings.Repeat("a", 10_000_000)
		raw := []byte("POST / HTTP/1.1\r\nContent-Length: 10000000\r\n\r\n" + body)
		dispersed := disperse(raw, config.Default().NET.ReadBufferSize)
		client := dummy.NewCircularClient(dispersed...)
		server, _ := newSuit(client)

		for i := 0; i < N; i++ {
			for j := 0; j < len(dispersed); j++ {
				require.True(t, server.ServeOnce())
			}
		}
	})

	t.Run("discard chunked 10mib", func(t *testing.T) {
		const chunkSize = 0xfffe
		const numberOfChunks = 10_000_000 / chunkSize
		chunk := "fffe\r\n" + strings.Repeat("a", chunkSize) + "\r\n"
		chunked := strings.Repeat(chunk, numberOfChunks) + "0\r\n\r\n"
		raw := []byte("POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n" + chunked)
		dispersed := disperse(raw, config.Default().NET.ReadBufferSize)
		client := dummy.NewCircularClient(dispersed...)
		server, _ := newSuit(client)

		for i := 0; i < N; i++ {
			for j := 0; j < len(dispersed); j++ {
				require.True(t, server.ServeOnce())
			}
		}
	})
}

func compareHeaders(a, b headers.Headers) bool {
	first, second := a.Expose(), b.Expose()
	if len(first) != len(second) {
		return false
	}

	for i, pair := range first {
		if pair != second[i] {
			return false
		}
	}

	return true
}
