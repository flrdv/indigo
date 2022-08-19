package router

import (
	"indigo/errors"
	"indigo/http/status"
	"indigo/types"
)

/*
This file contains core-callbacks that are called by server, so it's
like a core of the router
*/

// OnStart currently only applies default headers, but in future it will also
// apply all the middlewares onto handlers
func (d DefaultRouter) OnStart() {
	d.applyDefaultHeaders()
}

// OnRequest routes the request
func (d DefaultRouter) OnRequest(request *types.Request, respWriter types.ResponseWriter) error {
	urlMethods, found := d.routes[request.Path]
	if !found {
		return respWriter(d.renderer.Response(request.Proto, defaultNotFound))
	}

	handler, found := urlMethods[request.Method]
	if !found {
		return respWriter(d.renderer.Response(request.Proto, defaultMethodNotAllowed))
	}

	return respWriter(d.renderer.Response(request.Proto, handler(request)))
}

// OnError receives error and decides, which error handler is better to use in this case
func (d DefaultRouter) OnError(request *types.Request, respWriter types.ResponseWriter, err error) {
	var code status.Code

	switch err {
	case errors.ErrCloseConnection:
		code = status.ConnectionClose
	case errors.ErrBadRequest:
		code = status.BadRequest
	case errors.ErrTooLarge:
		code = status.RequestEntityTooLarge
	case errors.ErrHeaderFieldsTooLarge:
		code = status.RequestHeaderFieldsTooLarge
	case errors.ErrURITooLong:
		code = status.RequestURITooLong
	case errors.ErrUnsupportedProtocol:
		code = status.HTTPVersionNotSupported
	default:
		// unknown error, but for consistent behaviour we must respond with
		// something. Let it be some neutral error
		code = status.BadRequest
	}

	response := d.errHandlers[code](request)
	_ = respWriter(d.renderer.Response(request.Proto, response))
}