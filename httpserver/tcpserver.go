package httpserver

import (
	"net"
)

/*
TCP server is a first layer in the web-server, it is responsible for all the low-level activity directly
with sockets. All the received data it provides to
*/

const ReadBytesPerOnce = 2048 // may be even decreased to 1024

type (
	connHandler func(net.Conn)
	dataHandler func([]byte) error
)

func StartTCPServer(sock net.Listener, handleConn connHandler) error {
	for {
		conn, err := sock.Accept()

		if err != nil {
			// high-level api anyway will handle this error
			// and restart tcp server
			return err
		}

		go handleConn(conn)
	}
}

func DefaultConnHandler(conn net.Conn, handleData dataHandler) {
	defer conn.Close()
	buff := make([]byte, ReadBytesPerOnce)

	for {
		n, err := conn.Read(buff)

		if n == 0 || err != nil || handleData(buff[:n]) != nil {
			return
		}
	}
}
