package inbuilt

import (
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/method"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/router"
	"github.com/indigo-web/indigo/router/inbuilt/internal/radix"
	"github.com/indigo-web/indigo/router/inbuilt/uri"
)

var _ router.Builder = new(Router)

// Router is a built-in routing entity. It provides support for all the methods defined in
// the methods package, including shortcuts for those. It also supports dynamic routing
// (enabled automatically if dynamic path template is registered; otherwise more performant
// static-routing implementation is used). It also provides custom error handlers for any
// HTTP error that may occur during parsing the request or the routing of it by itself.
// By default, TRACE requests are supported (if no handler is attached, the request will be
// automatically processed), OPTIONS (including server-wide ones) and 405 Method Not Allowed
// errors in compliance with their HTTP semantics.
type Router struct {
	isRoot      bool
	prefix      string
	mutators    []Mutator
	middlewares []Middleware
	registrar   *registrar
	children    []*Router
	errHandlers errorHandlers
}

// New constructs a new instance of inbuilt router
func New() *Router {
	return &Router{
		isRoot:      true,
		registrar:   newRegistrar(),
		errHandlers: newErrorHandlers(),
	}
}

// runtimeRouter is the actual router that'll be running. The reason to separate Router from runtimeRouter
// is the fact, that there is a lot of data that is used only at registering/initialization stage.
type runtimeRouter struct {
	mutators    []Mutator
	traceBuff   []byte
	tree        *radix.Node[endpoint]
	routesMap   routesMap
	errHandlers errorHandlers
	isStatic    bool
}

func (r *Router) Build() router.Router {
	r.applyErrorHandlersMiddlewares()

	if err := r.prepare(); err != nil {
		panic(err)
	}

	isDynamic := r.registrar.IsDynamic()
	var (
		rmap routesMap
		tree *radix.Node[endpoint]
	)
	if isDynamic {
		tree = r.registrar.AsRadixTree()
	} else {
		rmap = r.registrar.AsMap()
	}

	return &runtimeRouter{
		mutators:    r.mutators,
		tree:        tree,
		routesMap:   rmap,
		errHandlers: r.errHandlers,
		isStatic:    !isDynamic,
	}
}

// OnRequest processes the request
func (r *runtimeRouter) OnRequest(request *http.Request) *http.Response {
	r.runMutators(request)

	// TODO: should path normalization be implemented as a mutator?
	request.Path = uri.Normalize(request.Path)

	return r.onRequest(request)
}

func (r *runtimeRouter) onRequest(request *http.Request) *http.Response {
	var (
		e     endpoint
		found bool
	)

	if r.isStatic {
		e, found = r.routesMap[request.Path]
	} else {
		e, found = r.tree.Lookup(request.Path, request.Vars)
	}

	if !found {
		return r.onError(request, status.ErrNotFound)
	}

	handler := getHandler(request.Method, e.methods)
	if handler == nil {
		request.Env.AllowedMethods = e.allow

		return r.onError(request, status.ErrMethodNotAllowed)
	}

	return handler(request)
}

// OnError uses a user-defined error handler, otherwise default http.Error
func (r *runtimeRouter) OnError(request *http.Request, err error) *http.Response {
	r.runMutators(request)

	return r.onError(request, err)
}

func (r *runtimeRouter) onError(request *http.Request, err error) *http.Response {
	if request.Method == method.TRACE && err == status.ErrMethodNotAllowed {
		r.traceBuff = renderHTTPRequest(request, r.traceBuff)

		return traceResponse(request.Respond(), r.traceBuff)
	}

	httpErr, ok := err.(status.HTTPError)
	if !ok {
		return http.Code(request, status.InternalServerError)
	}

	handler := r.retrieveErrorHandler(httpErr.Code)
	if handler == nil {
		// not using http.Error(request, err) in performance purposes, as in this case
		// it would try under the hood to unwrap the error again, however we did this already
		return request.Respond().
			Code(httpErr.Code).
			String(httpErr.Message)
	}

	request.Env.Error = err

	return handler(request)
}

func (r *runtimeRouter) runMutators(request *http.Request) {
	for _, mutator := range r.mutators {
		mutator(request)
	}
}

func (r *runtimeRouter) retrieveErrorHandler(code status.Code) Handler {
	handler, found := r.errHandlers[code]
	if !found {
		return r.errHandlers[AllErrors]
	}

	return handler
}

func (r *Router) applyErrorHandlersMiddlewares() {
	for code, handler := range r.errHandlers {
		r.errHandlers[code] = compose(handler, r.middlewares)
	}
}

// getHandler looks up for a handler in the methodsMap. In case request method is HEAD, however
// no matching handler is found, a handler for corresponding GET request will be retrieved
func getHandler(reqMethod method.Method, mlut methodLUT) Handler {
	handler := mlut[reqMethod]
	if handler == nil && reqMethod == method.HEAD {
		return getHandler(method.GET, mlut)
	}

	return handler
}

// TODO: implement responding on such requests with a global list of all the available methods
func isServerWideOptions(req *http.Request) bool {
	return req.Method == method.OPTIONS && req.Path == "*"
}
