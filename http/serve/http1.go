package serve

import (
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/internal/codecutil"
	"github.com/indigo-web/indigo/internal/construct"
	"github.com/indigo-web/indigo/internal/protocol/http1"
	"github.com/indigo-web/indigo/router"
	"github.com/indigo-web/indigo/transport"
)

// HTTP1 setups and serves an HTTP/1.1 server until it stops. Note that the connection isn't
// automatically closed on server stop
func HTTP1(
	cfg *config.Config,
	client transport.Client,
	enc uint16,
	r router.Router,
	codecs codecutil.Cache,
) {
	request := construct.Request(cfg, client.Conn)
	request.Env.Encryption = enc
	h1 := http1.New(cfg, r, client, request, codecs)
	request.Body = http.NewBody(h1)
	h1.Serve()
}
