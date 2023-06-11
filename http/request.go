package http

import (
	"context"
	"github.com/indigo-web/indigo/internal/unreader"
	"io"
	"net"

	"github.com/indigo-web/indigo/http/headers"
	// I don't know why, but otherwise GoLand cries about unused import, even if it's used
	"github.com/indigo-web/indigo/http/method"
	"github.com/indigo-web/indigo/http/proto"
	"github.com/indigo-web/indigo/http/query"
)

type (
	onBodyCallback     func([]byte) error
	onCompleteCallback func() error
	BodyReader         interface {
		Init(*Request)
		Read() ([]byte, error)
	}
)

type (
	Params = map[string]string

	Path struct {
		String   string
		Params   Params
		Query    query.Query
		Fragment Fragment
	}

	Fragment = string
)

// Request struct represents http request
// About headers manager see at http/headers/headers.go:Manager
// Headers attribute references at that one that lays in manager
type Request struct {
	body             BodyReader
	conn             net.Conn
	Remote           net.Addr
	Ctx              context.Context
	Headers          *headers.Headers
	response         Response
	Path             Path
	bodyBuff         []byte
	ContentLength    int
	TransferEncoding headers.TransferEncoding
	Method           method.Method
	Upgrade          proto.Proto
	Proto            proto.Proto
	wasHijacked      bool
	clearParamsMap   bool
}

// NewRequest returns a new instance of request object and body gateway
// Must not be used externally, this function is for internal purposes only
// HTTP/1.1 as a protocol by default is set because if first request from user
// is invalid, we need to render a response using request method, but appears
// that default method is a null-value (proto.Unknown)
func NewRequest(
	hdrs *headers.Headers, query query.Query, response Response, conn net.Conn, body BodyReader,
	paramsMap Params, disableParamsMapClearing bool,
) *Request {
	request := &Request{
		Path: Path{
			Params: paramsMap,
			Query:  query,
		},
		Proto:          proto.HTTP11,
		Headers:        hdrs,
		Remote:         conn.RemoteAddr(),
		conn:           conn,
		body:           body,
		Ctx:            context.Background(),
		response:       response,
		clearParamsMap: !disableParamsMapClearing,
	}

	return request
}

// OnBody is a low-level interface accessing a request body. It takes onBody callback that is
// being called every time a piece of body is read (note: even a single byte can be passed).
// In case error returned, it'll be returned from OnBody method. In case onBody never did return
// an error, onComplete will be called when the body will be finished. This callback also can
// return an error that'll be returned from OnBody method - for example, in case body's hash sum
// is invalid
func (r *Request) OnBody(onBody onBodyCallback, onComplete onCompleteCallback) error {
	for {
		piece, err := r.body.Read()
		switch err {
		case nil:
		case io.EOF:
			return onComplete()
		default:
			return err
		}

		if err = onBody(piece); err != nil {
			return err
		}
	}
}

// Body is a high-level function that wraps OnBody, and the only it does is reading
// pieces of body into the buffer that is a nil by default, but may grow and will stay
// as big as it grew until the disconnect
func (r *Request) Body() ([]byte, error) {
	if !r.HasBody() {
		return nil, nil
	}

	if r.bodyBuff == nil {
		r.bodyBuff = make([]byte, r.ContentLength)
	}

	r.bodyBuff = r.bodyBuff[:0]

	err := r.OnBody(func(b []byte) error {
		r.bodyBuff = append(r.bodyBuff, b...)
		return nil
	}, func() error {
		return nil
	})

	return r.bodyBuff, err
}

// Reader returns io.Reader for request body. This method may be called multiple times,
// but reading from multiple readers leads to Undefined Behaviour
func (r *Request) Reader() io.Reader {
	return newBodyIOReader(r.body)
}

// HasBody returns not actual "whether request contains a body", but a possibility.
// So result only depends on whether content-length is more than 0, or chunked
// transfer encoding is enabled
func (r *Request) HasBody() bool {
	return r.ContentLength > 0 || r.TransferEncoding.Chunked
}

// Hijack the connection. Request body will be implicitly read (so if you need it you
// should read it before) all the body left. After handler exits, the connection will
// be closed, so the connection can be hijacked only once
func (r *Request) Hijack() (net.Conn, error) {
	if err := r.resetBody(); err != nil {
		return nil, err
	}

	r.wasHijacked = true

	return r.conn, nil
}

// WasHijacked returns true or false, depending on whether was a connection hijacked
func (r *Request) WasHijacked() bool {
	return r.wasHijacked
}

// Clear resets request headers and reads body into nowhere until completed.
// It is implemented to clear the request object between requests
func (r *Request) Clear() (err error) {
	r.Path.Fragment = ""
	r.Path.Query.Set(nil)
	r.Ctx = context.Background()
	r.response = r.response.Clear()

	if err = r.resetBody(); err != nil {
		return err
	}

	r.ContentLength = 0
	r.TransferEncoding = headers.TransferEncoding{}
	r.Upgrade = proto.Unknown

	if r.clearParamsMap && len(r.Path.Params) > 0 {
		for k := range r.Path.Params {
			delete(r.Path.Params, k)
		}
	}

	return nil
}

// resetBody just reads the body until its end
func (r *Request) resetBody() error {
	for {
		_, err := r.body.Read()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
	}
}

// RespondTo returns a response object of request
func RespondTo(request *Request) Response {
	return request.response
}

// bodyIOReader is an implementation of io.Reader for request body
type bodyIOReader struct {
	unreader *unreader.Unreader
	reader   BodyReader
}

func newBodyIOReader(reader BodyReader) bodyIOReader {
	return bodyIOReader{
		unreader: new(unreader.Unreader),
		reader:   reader,
	}
}

func (b bodyIOReader) Read(buff []byte) (n int, err error) {
	data, err := b.unreader.PendingOr(b.reader.Read)
	copy(buff, data)
	n = len(data)

	if len(buff) < len(data) {
		b.unreader.Unread(data[len(buff):])
		n = len(buff)
	}

	return n, err
}

func (b bodyIOReader) WriteTo(w io.Writer) (n int64, err error) {
	for {
		data, err := b.reader.Read()
		switch err {
		case nil:
		case io.EOF:
			return n, nil
		default:
			return 0, err
		}

		n1, err := w.Write(data)
		n += int64(n1)
	}
}
