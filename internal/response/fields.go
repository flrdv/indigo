package response

import (
	"github.com/indigo-web/indigo/http/cookie"
	"github.com/indigo-web/indigo/http/mime"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/internal/types"
	"github.com/indigo-web/indigo/kv"
)

const DefaultContentType = mime.HTML

type Fields struct {
	Attachment  types.Attachment
	Headers     []kv.Pair
	Body        []byte
	Cookies     []cookie.Cookie
	Status      status.Status
	ContentType mime.MIME
	// TODO: add corresponding Content-Encoding field
	// TODO: automatically apply the encoding on a body when specified
	TransferEncoding string
	Code             status.Code
}

func (f *Fields) Clear() {
	f.Code = status.OK
	f.Status = ""
	f.ContentType = DefaultContentType
	f.TransferEncoding = ""
	f.Headers = f.Headers[:0]
	f.Body = nil
	f.Cookies = f.Cookies[:0]
	f.Attachment = types.Attachment{}
}
