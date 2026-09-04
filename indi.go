package indigo

import (
	"crypto/tls"

	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http/codec"
	"github.com/indigo-web/indigo/http/proto"
	"github.com/indigo-web/indigo/router"
	"github.com/indigo-web/indigo/router/inbuilt"
	"github.com/indigo-web/indigo/router/inbuilt/uri"
	"github.com/indigo-web/indigo/transport"
)

const Version = "0.18.0"

// App is just a struct with addr and shutdown channel that is currently
// not used. Planning to replace it with context.WithCancel()
type App struct {
	cfg   *config.Config
	hooks struct {
		OnStart func()
		OnBind  func(addr string)
		OnStop  func()
	}
	codecs     []codec.Codec
	orders     []transportOrder
	supervisor transport.Supervisor
}

// New returns a new App instance.
func New(addr ...string) *App {
	app := &App{
		cfg:        config.Default(),
		supervisor: transport.NewSupervisor(),
	}

	for _, a := range addr {
		app.TCP(a)
	}

	return app
}

// Tune replaces default config.
func (a *App) Tune(cfg *config.Config) *App {
	a.cfg = cfg
	return a
}

// OnStart calls the callback at the moment, when all the servers are started. However,
// it isn't strongly guaranteed that they'll be able to accept new connections immediately.
func (a *App) OnStart(cb func()) *App {
	a.hooks.OnStart = cb
	return a
}

// OnBind callback is called every time a listener is ready to accept new connections.
func (a *App) OnBind(cb func(addr string)) *App {
	a.hooks.OnBind = cb
	return a
}

// OnStop calls the callback at the moment, when all the servers are down. It's guaranteed,
// that at the moment as the callback is called, the server isn't able to accept any new connections
// and all the clients are already disconnected.
func (a *App) OnStop(cb func()) *App {
	a.hooks.OnStop = cb
	return a
}

// Disable allows the application to disable using specific HTTP versions.
func (a *App) Disable(protocol proto.Protocol) *App {
	a.cfg.NET.Protocols &^= protocol
	return a
}

// Codec appends a new codec into the list of supported.
func (a *App) Codec(codecs ...codec.Codec) *App {
	a.codecs = append(a.codecs, codecs...)
	return a
}

// TCP tells the application to bind a plain TCP listener.
func (a *App) TCP(addr string) *App {
	a.orders = append(a.orders, transportOrder{
		addr:      addr,
		transport: TCP,
	})
	return a
}

// TLS tells the application to bind a TLS listener.
func (a *App) TLS(addr string, cert tls.Certificate, other ...tls.Certificate) *App {
	certs := append([]tls.Certificate{cert}, other...)
	return a.TLSWithConfig(addr, &tls.Config{Certificates: certs})
}

// TLSWithConfig tells the application to bind a TLS listener with the provided config.
func (a *App) TLSWithConfig(addr string, cfg *tls.Config) *App {
	a.orders = append(a.orders, transportOrder{
		addr:      addr,
		transport: TLS,
		cfg:       cfg,
	})
	return a
}

// Serve starts the application. If passed nil, default inbuilt.Router is used instead.
//
// TODO: add a greeting router.
func (a *App) Serve(r router.Builder) error {
	if r == nil {
		r = inbuilt.New()
	}

	return a.run(r.Build())
}

func (a *App) run(r router.Router) error {
	if a.hooks.OnStart != nil {
		a.hooks.OnStart()
	}

	for _, order := range a.orders {
		addr := uri.Normalize(order.addr)
		if len(addr) == 0 {
			continue
		}

		var (
			cb transportCallback
			tp transport.Transport
		)

		switch order.transport {
		case TCP:
			cb, tp = tcpCallback, transport.NewTCP()
		case TLS:
			cfg := order.cfg
			cfg.NextProtos = alpnTokens(a.cfg.NET.Protocols)

			cb, tp = tlsCallback, transport.NewTLS(cfg)
		default:
			panic("BUG: unrecognized transport order")
		}

		if err := a.supervisor.Bind(addr, tp, cb(a.cfg, r, a.codecs)); err != nil {
			return err
		}

		if a.hooks.OnBind != nil {
			a.hooks.OnBind(addr)
		}
	}

	err := a.supervisor.Run(a.cfg.NET)
	if a.hooks.OnStop != nil {
		a.hooks.OnStop()
	}

	return err
}

// Stop stops the whole application immediately and waits until it _really_ stops.
func (a *App) Stop() {
	a.supervisor.Stop()
}

func alpnTokens(set proto.Protocol) []string {
	protos := make([]string, 0, 2)
	if set&proto.HTTP2 != 0 {
		protos = append(protos, "h2")
	}
	if set&proto.HTTP11 != 0 {
		protos = append(protos, "http/1.1")
	}

	return protos
}
