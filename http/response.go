package http

import (
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/internal/render/types"
	"github.com/indigo-web/indigo/internal/strcomp"
	"github.com/indigo-web/utils/uf"
	json "github.com/json-iterator/go"
	"io"
	"os"
)

type ResponseWriter func(b []byte) error

// IDK why 7, but let it be
const (
	defaultHeadersNumber = 7
	defaultContentType   = "text/html"
)

type Response interface {
	error
}

var _ Response = &Builder{}

type Builder struct {
	attachment  types.Attachment
	Status      status.Status
	ContentType string
	// TODO: add corresponding Content-Encoding field
	// TODO: automatically apply the encoding on a body when specified
	TransferEncoding string
	headers          []string
	Body             []byte
	Code             status.Code
}

func NewBuilder() *Builder {
	return &Builder{
		Code:        status.OK,
		headers:     make([]string, 0, defaultHeadersNumber*2),
		ContentType: defaultContentType,
	}
}

// WithCode sets a response code and a corresponding status.
// In case of unknown code, "Unknown Status Code" will be set as a status
// code. In this case you should call Status explicitly
func (b *Builder) WithCode(code status.Code) *Builder {
	b.Code = code
	return b
}

// WithStatus sets a custom status text. This text does not matter at all, and usually
// totally ignored by client, so there is actually no reasons to use this except some
// rare cases when you need to represent a response status text somewhere
func (b *Builder) WithStatus(status status.Status) *Builder {
	b.Status = status
	return b
}

// WithContentType sets a custom Content-Type header value.
func (b *Builder) WithContentType(value string) *Builder {
	b.ContentType = value
	return b
}

// WithTransferEncoding sets a custom Transfer-Encoding header value.
func (b *Builder) WithTransferEncoding(value string) *Builder {
	b.TransferEncoding = value
	return b
}

// WithHeader sets header values to a key. In case it already exists the value will
// be appended.
func (b *Builder) WithHeader(key string, values ...string) *Builder {
	switch {
	case strcomp.EqualFold(key, "content-type"):
		return b.WithContentType(values[0])
	case strcomp.EqualFold(key, "transfer-encoding"):
		return b.WithTransferEncoding(values[0])
	}

	for i := range values {
		b.headers = append(b.headers, key, values[i])
	}

	return b
}

// WithHeaders simply merges passed headers into response. Also, it is the only
// way to specify a quality marker of value. In case headers were not initialized
// before, response headers will be set to a passed map, so editing this map
// will affect response
func (b *Builder) WithHeaders(headers map[string][]string) *Builder {
	resp := b

	for k, v := range headers {
		resp = resp.WithHeader(k, v...)
	}

	return resp
}

// DiscardHeaders returns response object with no any headers set.
//
// Warning: this action is not pure. Appending new headers will cause overriding
// old ones
func (b *Builder) DiscardHeaders() *Builder {
	b.headers = b.headers[:0]
	return b
}

// WithBody sets a string as a response body. This will override already-existing
// body if it was set
func (b *Builder) WithBody(body string) *Builder {
	return b.WithBodyByte(uf.S2B(body))
}

// WithBodyByte does all the same as Body does, but for byte slices
func (b *Builder) WithBodyByte(body []byte) *Builder {
	b.Body = body
	return b
}

// Write implements io.Writer interface for response body
func (b *Builder) Write(data []byte) (n int, err error) {
	b.Body = append(b.Body, data...)

	return len(data), nil
}

// WithFile opens a file for reading, and returns a new response with attachment corresponding
// to the file FD. In case not found or any other error, it'll be directly returned.
// In case error occurred while opening the file, response builder won't be affected and stay
// clean
func (b *Builder) WithFile(path string) (*Builder, error) {
	file, err := os.Open(path)
	if err != nil {
		return b, err
	}

	stat, err := file.Stat()
	if err != nil {
		return b, err
	}

	return b.WithAttachment(file, int(stat.Size())), nil
}

// WithAttachment sets a response's attachment. In this case response body will be ignored.
// If size <= 0, then Transfer-Encoding: chunked will be used
func (b *Builder) WithAttachment(reader io.Reader, size int) *Builder {
	b.attachment = types.NewAttachment(reader, size)
	return b
}

// WithJSON receives a model (must be a pointer to the structure) and returns a new Response
// object and an error
func (b *Builder) WithJSON(model any) (*Builder, error) {
	b.Body = b.Body[:0]
	stream := json.ConfigDefault.BorrowStream(b)
	stream.WriteVal(model)
	err := stream.Flush()
	json.ConfigDefault.ReturnStream(stream)

	if err != nil {
		return b, err
	}

	return b.WithContentType("application/json"), nil
}

// WithError returns response with corresponding HTTP error code, if passed error is
// status.HTTPError. Otherwise, code is considered to be 500 Internal Server Error.
// Note: revealing error text may be dangerous
func (b *Builder) WithError(err error) *Builder {
	if http, ok := err.(status.HTTPError); ok {
		return b.
			WithCode(http.Code).
			WithBody(http.Message)
	}

	return b.
		WithCode(status.InternalServerError).
		WithBody(err.Error())
}

// Headers returns an underlying response headers
func (b *Builder) Headers() []string {
	return b.headers
}

// Attachment returns response's attachment.
//
// WARNING: do NEVER use this method in your code. It serves internal purposes ONLY
func (b *Builder) Attachment() types.Attachment {
	return b.attachment
}

func (b *Builder) Error() string {
	if b.Code.IsError() {
		return uf.B2S(b.Body)
	}

	return ""
}

// Clear discards everything was done with response object before
func (b *Builder) Clear() *Builder {
	b.Code = status.OK
	b.Status = ""
	b.ContentType = defaultContentType
	b.TransferEncoding = ""
	b.headers = b.headers[:0]
	b.Body = nil
	b.attachment = types.Attachment{}
	return b
}

func AsBuilder(request *Request, resp Response) *Builder {
	builder, ok := resp.(*Builder)
	if !ok {
		builder = request.Respond().WithError(resp)
	}

	return builder
}
