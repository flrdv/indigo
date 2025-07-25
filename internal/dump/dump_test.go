package dump

import (
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/method"
	"github.com/indigo-web/indigo/http/proto"
	"github.com/indigo-web/indigo/internal/construct"
	"github.com/indigo-web/indigo/kv"
	"github.com/indigo-web/indigo/transport/dummy"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSerializeRequest(t *testing.T) {
	cfg := config.Default()
	client := dummy.NewClient([]byte("Hello, world!")).Once()
	request := construct.Request(cfg, client)
	request.Body = http.NewBody(cfg, client)

	request.Headers = kv.New().
		Add("hello", "world").
		Add("foo", "bar")
	request.Params = kv.New().
		Add("hello", "world").
		Add("somewhere", "there")
	request.Method = method.GET
	request.Path = "/"
	request.Protocol = proto.HTTP11
	request.ContentLength = 13

	request.Body.Reset(request)

	dumped, err := Request(request)
	require.NoError(t, err)
	want := "GET /?hello=world&somewhere=there HTTP/1.1\r\nhello: world\r\nfoo: bar\r\nContent-Length: 13\r\n\r\nHello, world!"
	require.Equal(t, want, dumped)
}
