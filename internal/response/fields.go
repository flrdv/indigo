package response

import (
	"github.com/indigo-web/indigo/http/cookie"
	"github.com/indigo-web/indigo/http/mime"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/kv"
	"io"
)

const DefaultContentType = mime.HTML

type Fields struct {
	Code         status.Code
	Status       status.Status
	Encoding     string
	ContentType  mime.MIME
	Charset      mime.Charset
	CharsetSet   bool
	Stream       io.Reader
	StreamSize   int64
	BufferedBody []byte
	Headers      []kv.Pair
	Cookies      []cookie.Cookie
}

func (f *Fields) Clear() {
	f.Code = status.OK
	f.Status = ""
	f.Encoding = ""
	f.ContentType = DefaultContentType
	f.Charset = mime.Unset
	f.CharsetSet = false
	f.Stream = nil
	f.StreamSize = -1
	f.BufferedBody = nil
	f.Headers = f.Headers[:0]
	f.Cookies = f.Cookies[:0]
}
