package middleware

import (
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/router/inbuilt/types"
)

// HTTPSOnly redirects all http requests to https. In case no Host header is provided,
// 400 Bad Request will be returned without calling the actual handler.
//
// Note: it causes 1 (one) allocation
func HTTPSOnly(next types.Handler, req *http.Request) *http.Response {
	if req.IsTLS {
		return next(req)
	}

	host := req.Headers.Value("host")
	if len(host) == 0 {
		return http.Error(req, status.ErrBadRequest).
			WithBody("the request lacks Host header value")
	}

	return req.Respond().
		WithCode(status.MovedPermanently).
		WithHeader("Location", "https://"+host+req.Path)
}