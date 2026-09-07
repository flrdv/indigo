package http1

import (
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/proto"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/internal/buffer"
	"github.com/indigo-web/indigo/internal/codecutil"
	"github.com/indigo-web/indigo/internal/construct"
	"github.com/indigo-web/indigo/internal/strutil"
	"github.com/indigo-web/indigo/router"
	"github.com/indigo-web/indigo/transport"
)

type HTTP1 struct {
	*Parser
	*Body
	*serializer
	client *Client
	router router.Router
	codecs codecutil.Cache
}

func newHTTP1(
	cfg *config.Config,
	r router.Router,
	request *http.Request,
	client *Client,
	body *Body,
	codecs codecutil.Cache,
	statusBuff, headersBuff *buffer.Buffer,
	respBuff []byte,
) *HTTP1 {
	return &HTTP1{
		Parser:     NewParser(cfg, request, statusBuff, headersBuff),
		Body:       body,
		serializer: newSerializer(cfg, request, client.Conn, codecs, respBuff),
		router:     r,
		client:     client,
		codecs:     codecs,
	}
}

// New instantiates an HTTP/1 protocol suit.
func New(
	cfg *config.Config,
	r router.Router,
	client transport.Client,
	request *http.Request,
	codecs codecutil.Cache,
) *HTTP1 {
	statusBuff, headersBuff := construct.Buffers(cfg)
	reqBuff := make([]byte, cfg.HTTP1.ReadBuffer)
	c := NewClient(client, reqBuff)
	b := NewBody(c, cfg.Body)
	respBuff := make([]byte, 0, cfg.NET.WriteBufferSize.Default)

	return newHTTP1(cfg, r, request, c, b, codecs, statusBuff, headersBuff, respBuff)
}

func (h *HTTP1) ServeOnce() (ok bool) {
	return h.serve(true)
}

func (h *HTTP1) Serve() {
	h.serve(false)
}

func (h *HTTP1) serve(once bool) (ok bool) {
	client := h.client
	request := h.Parser.request

	for {
		data, err := client.Fetch()
		if err != nil {
			// read-error most probably means deadline exceeding. Just notify the user in
			// this case and return.
			h.router.OnError(request, status.ErrCloseConnection)
			return false
		}

		// todo check whether we've got an HTTP/2 preface.
		done, bodydata, err := h.Parse(data)
		if err != nil {
			resp := respond(request, h.router.OnError(request, err))
			_ = h.Write(request.Protocol, resp)
			return false
		}

		if h.Parser.cfg.NET.Protocols&request.Protocol == 0 {
			resp := respond(request, h.router.OnError(request, status.ErrHTTPVersionNotSupported))
			_ = h.Write(request.Protocol, resp)
			return false
		}

		if !done {
			if once {
				return true
			}

			continue
		}

		client.Pushback(bodydata)
		request.Body.Reset(request)
		h.Body.Reset(request)

		transferEncoding := request.TransferEncoding
		if !validateTransferEncodingTokens(transferEncoding) {
			resp := respond(request, h.router.OnError(request, status.ErrUnsupportedEncoding))
			_ = h.Write(request.Protocol, resp)
			return false
		}

		if len(transferEncoding) > 0 {
			// get rid of the trailing chunked encoding as it is already built-in.
			if err = h.applyDecoders(transferEncoding[:len(transferEncoding)-1]); err != nil {
				// even if the connection is going to be upgraded in advance, the error happened with the
				// request prior to upgrade.
				resp := respond(request, h.router.OnError(request, err))
				_ = h.Write(request.Protocol, resp)
				return false
			}
		}

		if err = h.applyDecoders(request.ContentEncoding); err != nil {
			resp := respond(request, h.router.OnError(request, err))
			_ = h.Write(request.Protocol, resp)
			return false
		}

		version := request.Protocol
		if request.Upgrade != proto.Unknown && proto.HTTP1&request.Upgrade != 0 {
			h.Upgrade()
			version = request.Upgrade
		}

		resp := respond(request, h.router.OnRequest(request))

		if request.Hijacked() {
			// in case the connection was hijacked, we must not intrude after, so fail fast
			return false
		}

		if err = h.Write(version, resp); err != nil {
			// considering any write errors could occur due to broken connection, it makes
			// thereby no sense to try to write any error back. Moreover, there could be an
			// already sent data, which would overlay and result in a complete mess at the
			// client side.
			h.router.OnError(request, status.ErrCloseConnection)
			return false
		}

		if err = request.Body.Discard(); err != nil {
			resp = h.router.OnError(request, status.ErrCloseConnection)
			_ = h.Write(request.Protocol, resp)
			return false
		}

		if !isKeepAlive(version, request) {
			h.router.OnError(request, status.ErrCloseConnection)
			return true
		}

		if once {
			return true
		}

		request.Reset()
	}
}

func isKeepAlive(protocol proto.Protocol, req *http.Request) bool {
	switch protocol {
	case proto.HTTP10:
		return strutil.CmpFoldSafe(req.Connection, "keep-alive")
	case proto.HTTP11:
		// in case of HTTP/1.1, keep-alive may be only disabled
		return !strutil.CmpFoldSafe(req.Connection, "close")
	default:
		// we are responsible for HTTP/1 only. All others are most probably a consequence
		// of some kind of bug.
		return false
	}
}

func validateTransferEncodingTokens(tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}

	for _, token := range tokens[:len(tokens)-1] {
		if token == "chunked" {
			return false
		}
	}

	return tokens[len(tokens)-1] == "chunked"
}

func (h *HTTP1) applyDecoders(tokens []string) error {
	request := h.Parser.request
	bufferSize := h.Parser.cfg.HTTP1.ReadBuffer

	for i := len(tokens); i > 0; i-- {
		c := h.codecs.Get(tokens[i-1])
		if c == nil {
			return status.ErrUnsupportedEncoding
		}

		if err := c.ResetDecompressor(request.Body.Fetcher, bufferSize); err != nil {
			return status.ErrInternalServerError
		}

		request.Body.Fetcher = c
	}

	return nil
}

// respond ensures the passed resp is not nil.
func respond(req *http.Request, resp *http.Response) *http.Response {
	if resp != nil {
		return resp
	}

	return http.Respond(req)
}
