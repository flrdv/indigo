package http2

import (
	"encoding/binary"
	"io"
	"math"
	"net"
	"sync/atomic"
)

type Serializer struct {
	window atomic.Uint32
	mu     mutex
	conn   net.Conn
}

func NewSerializer(conn net.Conn) Serializer {
	return Serializer{
		mu:   newMutex(),
		conn: conn,
	}
}

func (s *Serializer) Write(typ FrameType, flags uint8, stream uint32, payload []byte) error {
	headers := Frame{
		Type:   typ,
		Flags:  flags,
		Length: uint32(len(payload)),
		Stream: stream,
	}.ToBytes()

	buff := make([]byte, len(headers)+len(payload))
	copy(buff, headers[:])
	copy(buff[len(headers):], payload)

	s.mu.Acquire()
	defer s.mu.Release()
	_, err := s.conn.Write(buff)
	return err
}

func (s *Serializer) WriteStream(stream uint32, r io.Reader) error {
	type Sized interface {
		Len() int
	}

	payload, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	return s.Write(DATA, FENDSTREAM, stream, payload)
}

func (s *Serializer) WriteError(stream uint32, err *Error) error {
	var payload [4]byte
	binary.BigEndian.PutUint32(payload[:0], err.Code)

	return s.Write(RSTSTREAM, 0, stream, payload[:])
}

/*
When a worker wants to respond, it must acquire the connection for writing. Sometimes, one wants
to offload a lot of consequent frames like DATA, which might block the connection indefinitely.
Therefore, I reinvented mutex here for this sole purpose, but with chan instead of futex.
*/

// mutex is a handoff version of ordinary sync.Mutex. It allows others to hop in while
// others are running a long-term transaction. I.e. while uploading a file, we would also
// like to hop in to respond to a PING or upload another file simultaneously, thereby
// avoiding HoL-block.
type mutex struct {
	writers atomic.Uint32
	c       chan struct{}
}

func newMutex() mutex {
	return mutex{
		c: make(chan struct{}),
	}
}

func (m *mutex) Acquire() {
	if m.writers.Add(1) > 1 {
		// temporarily locked
		<-m.c
	}
}

func (m *mutex) Interrupt() {
	if m.writers.Load() > 1 {
		// others are willing to slip in
		m.c <- struct{}{}
		<-m.c
	}
}

func (m *mutex) Release() {
	if m.writers.Add(math.MaxUint32) > 0 {
		// pass the connection to all other writers that are still in action.
		m.c <- struct{}{}
	}
}
