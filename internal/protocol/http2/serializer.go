package http2

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"sync/atomic"

	"github.com/indigo-web/indigo/internal/protocol/http2/hpack"
)

type Serializer struct {
	mu mutex

	etable hpack.Table
	conn   net.Conn
}

func NewSerializer(conn net.Conn, etable hpack.Table) Serializer {
	return Serializer{
		mu:     newMutex(),
		etable: etable,
		conn:   conn,
	}
}

func (s *Serializer) Write(frame Frame, payload []byte) error {
	fmt.Println("writing:", frame, strconv.Quote(string(payload)))

	frame.Length = uint32(len(payload))
	buff := make([]byte, FrameOctets+len(payload))
	headers := frame.Serialize()
	copy(buff, headers[:])
	copy(buff[FrameOctets:], payload)

	s.mu.Acquire()
	defer s.mu.Release()
	_, err := s.conn.Write(buff)
	return err
}

func (s *Serializer) WriteStream(frame Frame, stream io.Reader) error {
	type Sized interface {
		Len() int
	}

	payload, err := io.ReadAll(stream)
	if err != nil {
		return err
	}

	return s.Write(frame, payload)
}

func (s *Serializer) WriteError(stream uint32, err *Error) error {
	data := Frame{
		Type:   RSTSTREAM,
		Flags:  0,
		Length: 4,
		Stream: stream,
	}.Serialize()

	buff := make([]byte, FrameOctets+4)
	copy(buff, data[:])
	buff = binary.BigEndian.AppendUint32(buff[:FrameOctets], err.Code)

	s.mu.Acquire()
	defer s.mu.Release()
	_, e := s.conn.Write(buff)

	return e
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
