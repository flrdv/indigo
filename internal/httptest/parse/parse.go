package parse

import (
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/internal/construct"
	"github.com/indigo-web/indigo/internal/protocol/http1"
	"github.com/indigo-web/indigo/transport"
	"github.com/indigo-web/indigo/transport/dummy"
)

func HTTP11Request(data string) (*http.Request, error) {
	cfg := config.Default()
	request := construct.Request(cfg, dummy.NewNop())

	b1, b2 := construct.Buffers(cfg)
	parser := http1.NewParser(cfg, request, b1, b2)

	_, bodydata, err := parser.Parse([]byte(data))
	if err != nil {
		return nil, err
	}

	client := transport.NewClient(dummy.New(bodydata), 0)
	h1client := http1.NewClient(client, make([]byte, 1024))
	h1body := http1.NewBody(h1client, cfg.Body)
	request.Body = http.NewBody(h1body)
	request.Body.Reset(request)
	h1body.Reset(request)

	return request, nil
}
