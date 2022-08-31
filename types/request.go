package types

import (
	"indigo/http/headers"
	methods "indigo/http/method"
	"indigo/http/proto"
	"indigo/http/url"
	"indigo/internal"
	"net"
)

type (
	// ConnectionHijacker is for user. It returns error because it has to
	// read full request body to stop the server in defined state. And,
	// as we know, reading body may return an error
	ConnectionHijacker func() (net.Conn, error)

	// hijackConn is like an interface of httpServer method that notifies
	// core about hijacking and returns connection object
	hijackConn func() net.Conn
)

// Request struct represents http request
// About headers manager see at http/headers/headers.go:Manager
// Headers attribute references at that one that lays in manager
type Request struct {
	Method   methods.Method
	Path     string
	Query    url.Query
	Fragment string
	Proto    proto.Proto

	Headers        headers.Headers
	headersManager *headers.Manager

	body     requestBody
	bodyBuff []byte

	Hijack ConnectionHijacker
}

// NewRequest returns a new instance of request object and body gateway
// Must not be used externally, this function is for internal purposes only
// HTTP/1.1 as a protocol by default is set because if first request from user
// is invalid, we need to render a response using request method, but appears
// that default method is a null-value (proto.Unknown)
// Also url.Query is being constructed right here instead of passing from outside
// because it has only optional purposes and buff will be nil anyway
// But maybe it's better to implement DI all the way we go? I don't know, maybe
// someone will contribute and fix this
func NewRequest(manager *headers.Manager, query url.Query) (*Request, *internal.BodyGateway) {
	requestBodyStruct, gateway := newRequestBody()
	request := &Request{
		Query:          query,
		Proto:          proto.HTTP11,
		Headers:        manager.Headers,
		headersManager: manager,
		body:           requestBodyStruct,
	}

	return request, gateway
}

// OnBody is a proxy-function for r.body.Read. This method reads body in streaming
// processing mode by calling onBody on each body piece, and onComplete when body
// is over (onComplete is guaranteed to be called except situation when body is already
// read)
func (r *Request) OnBody(onBody onBodyCallback, onComplete onCompleteCallback) error {
	return r.body.Read(onBody, onComplete)
}

// Body is a high-level function that wraps OnBody, and the only it does is reading
// pieces of body into the buffer that is a nil by default, but may grow and will stay
// as big as it grew until the disconnect
func (r *Request) Body() ([]byte, error) {
	r.bodyBuff = r.bodyBuff[:0]
	err := r.body.Read(func(b []byte) error {
		r.bodyBuff = append(r.bodyBuff, b...)
		return nil
	}, func(err error) {
		// ignore error here, because it will be anyway returned from r.body.Read call
	})

	return r.bodyBuff, err
}

// Reset resets request object. It is made to clear the object between requests
func (r *Request) Reset() error {
	r.headersManager.Reset()
	r.Headers = r.headersManager.Headers

	return r.body.Reset()
}

func Hijacker(request *Request, hijacker hijackConn) func() (net.Conn, error) {
	return func() (net.Conn, error) {
		// we anyway don't need to have a body anymore. Also, without reading
		// the body until complete server will not transfer into the state
		// we need so this step is anyway compulsory
		if err := request.body.Reset(); err != nil {
			return nil, err
		}

		return hijacker(), nil
	}
}
