package transport

import (
	"crypto/tls"
	"net"
)

type TLS struct {
	TCP
	cfg *tls.Config
}

func NewTLS(cfg *tls.Config) *TLS {
	return &TLS{cfg: cfg}
}

func (t *TLS) Bind(addr string) error {
	tcp, err := bindTCP(addr)
	if err != nil {
		return err
	}

	l := tls.NewListener(tcp, t.cfg)
	t.TCP = newTCP(tlsAdapter{
		TCPListener: tcp,
		tls:         l,
	})

	return nil
}

type tlsAdapter struct {
	*net.TCPListener
	tls net.Listener
}

func (t tlsAdapter) Accept() (net.Conn, error) {
	for {
		conn, err := t.tls.Accept()
		if err != nil {
			return nil, err
		}

		if err = conn.(*tls.Conn).Handshake(); err != nil {
			_ = conn.Close()
			continue
		}

		return conn, nil
	}

}
