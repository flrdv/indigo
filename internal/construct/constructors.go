package construct

import (
	"net"

	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/internal/buffer"
	"github.com/indigo-web/indigo/kv"
)

func Request(cfg *config.Config, conn net.Conn) *http.Request {
	headers := kv.NewPrealloc(cfg.Headers.Number.Default)
	params := kv.NewPrealloc(cfg.URI.ParamsPrealloc)
	vars := kv.New()
	request := http.NewRequest(cfg, http.NewResponse(), conn, headers, params, vars)

	return request
}

func Buffers(s *config.Config) (statusBuff, headersBuff *buffer.Buffer) {
	return buffer.New(s.URI.RequestLineSize.Default, s.URI.RequestLineSize.Maximal),
		buffer.New(s.Headers.Space.Default, s.Headers.Space.Maximal)
}
