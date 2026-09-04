package http2

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"

	"github.com/flrdv/uf"
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/method"
	"github.com/indigo-web/indigo/http/proto"
	"github.com/indigo-web/indigo/internal/construct"
	"github.com/indigo-web/indigo/internal/protocol/http2/hpack"
	"github.com/indigo-web/indigo/kv"
	"github.com/indigo-web/indigo/router"
	"github.com/indigo-web/indigo/transport"
)

const unsetUpperBit = ^uint32(0x80000000)

type HTTP2 struct {
	tlsVersion  uint16
	lastID      uint32
	allworkers  uint32
	freeworkers atomic.Uint32
	queue       chan struct{}

	cfg        *config.Config
	client     h2client
	workers    hmap
	dtable     hpack.Table
	router     router.Router
	settings   Settings
	serializer Serializer
}

func NewHTTP2(
	cfg *config.Config,
	client transport.Client,
	tlsVersion uint16,
	r router.Router,
) *HTTP2 {
	dtable, etable := hpack.NewPair(defaultSettings[SHEADERTABLESIZE])

	return &HTTP2{
		tlsVersion: tlsVersion,
		queue:      make(chan struct{}),
		cfg:        cfg,
		client:     newH2Client(client, cfg.HTTP2.WindowBuffer),
		workers:    newMap(cfg.HTTP2.MaxConcurrentStreams),
		dtable:     dtable,
		router:     r,
		settings:   defaultSettings,
		serializer: NewSerializer(client, etable),
	}
}

func (h *HTTP2) Serve() {
	if err := h.Prelude(); err != nil {
		// todo what to do kurwa
		return
	}

	h.allworkers = 1
	h.work()
}

func (h *HTTP2) Delegate() *Error {
	// todo can I really just pass data like that? Considering bumping, tails, etc.

	if h.freeworkers.Load() > 0 {
		h.freeworkers.Add(math.MaxUint32)
		h.queue <- struct{}{}
		return nil
	}

	if h.allworkers >= h.cfg.HTTP2.MaxConcurrentStreams {
		return &Error{true, REFUSEDSTREAM}
	}

	h.allworkers++
	go h.work()
	return nil
}

func (h *HTTP2) ReacquireConn() {
	h.freeworkers.Add(1)
	<-h.queue
}

func (h *HTTP2) work() {
	me := newWorker(0, h)
	_, err := h.Process(me)
	_ = h.serializer.Write(Frame{Type: GOAWAY}, uf.S2B(err.Error()))
}

func (h *HTTP2) Prelude() error {
	if err := h.readPreface(); err != nil {
		// todo do we need to notify the router at all?
		return err
	}

	frame, err := h.readFrame()
	if err != nil {
		//s.router.OnError(s.request, status.ErrCloseConnection)
		return err
	}

	if frame.Type != SETTINGS {
		//s.router.OnError(s.request, status.ErrBadRequest)
		return err
	}

	if err = h.readSettings(frame); err != nil {
		// todo notify router about the error
		return err
	}

	settings := Frame{
		Type:   SETTINGS,
		Flags:  0,
		Length: 0,
		Stream: 0,
	}
	return h.serializer.Write(settings, nil)
}

// Process is the real hero. It receives all the incoming frames and handles them.
//
// The returned error always contains the most recent frame that has caused the error.
//
// The returned frame hints the caller what to do. If Worker.ID is 0, then such frames
// won't be returned at all and the incoming request is handled in-place. Otherwise, the caller
// should find out who is going to handle the request. DATA frames are either returned (if
// belonging to the caller) or rerouted to an assigned stream.
func (h *HTTP2) Process(me *worker) (Frame, error) {
	request := construct.Request(h.cfg, h.client.Conn)
	request.Protocol = proto.HTTP2
	request.Env.Encryption = h.tlsVersion

	for {
		frame, err := h.readFrame()
		if err != nil {
			return frame, err
		}

		if frame.Length > h.settings[SMAXFRAMESIZE] {
			if !frame.AltersState() {
				if err = h.client.Skip(frame.Length); err != nil {
					return frame, err
				}

				continue
			}

			return frame, &Error{Stream: false, Code: FRAMESIZEERROR}
		}

		switch frame.Type {
		case DATA, CONTINUATION:
			switch frame.Stream {
			case 0:
				return frame, &Error{Stream: false, Code: PROTOCOLERROR}
			case me.ID:
				return frame, nil
			default:
				w := h.workers.Get(frame.Stream)
				if w == nil {
					// todo maybe differentiate also with STREAM_CLOSED? If frame.Stream <= h.lastID
					return frame, &Error{false, PROTOCOLERROR}
				}

				if err = h.rerouteData(w, frame); err != nil {
					return frame, err
				}
			}
		case HEADERS:
			if frame.Stream <= h.lastID {
				return frame, &Error{false, PROTOCOLERROR}
			}

			if me.ID != 0 {
				return frame, nil
			}

			for {
				if err = h.readHeaders(frame, request); err != nil {
					return frame, err
				}

				if frame.Is(FENDHEADERS) {
					break
				}

				frame, err = h.readFrame()
				if err != nil {
					return frame, err
				}

				if frame.Type != CONTINUATION {
					// after receiving HEADERS frame, until END_HEADERS only CONTINUATION frames are allowed.
					return frame, &Error{false, PROTOCOLERROR}
				}
			}

			h.lastID = frame.Stream
			me.ID = frame.Stream
			h.workers.Assign(me)

			if err := h.Delegate(); err != nil {
				h.workers.Delete(me.ID)
				me.ID = 0

				// todo continue processing the stream. Just don't call the handler.
				if err := h.serializer.WriteError(frame.Stream, err); err != nil {
					return Frame{}, err
				}

				continue
			}

			request.Body = http.NewBody(me)
			response := h.router.OnRequest(request)
			h.workers.Delete(me.ID)
			me.ID = 0

			_ = response // todo send the response lol

			headers := Frame{
				Type:   HEADERS,
				Flags:  FENDHEADERS,
				Stream: frame.Stream,
			}
			data := Frame{
				Type:   DATA,
				Flags:  FENDSTREAM,
				Stream: frame.Stream,
			}

			datastream := response.Expose().Stream
			if datastream == nil {
				headers.Flags |= FENDSTREAM
			}

			if err = h.serializer.Write(headers, []byte{136}); err != nil {
				return Frame{}, err
			}

			if datastream != nil {
				if err = h.serializer.WriteStream(data, response.Expose().Stream); err != nil {
					return Frame{}, err
				}
			}

			request.Reset()
			if err = request.Body.Discard(); err != nil {
				return Frame{}, err
			}

			h.ReacquireConn()
			/*
				h.writeFrame(HEADERS, FENDHEADERS, 1, frame.Stream)
				h.Client.Write([]byte{136})
				h.writeFrame(DATA, FENDSTREAM, 17, frame.Stream)
				h.Client.Write([]byte("Hello via HTTP/2!"))
				fmt.Println("the stream is over. Responded.")
			*/
		case PRIORITY:
			// deprecated; skip
			if err = h.readPriority(frame); err != nil {
				return frame, err
			}
		case RSTSTREAM:
			// todo PROBLEM: must be ready to process frames on this stream, but also discard them?
			if frame.Stream == 0 {
				return frame, &Error{Stream: false, Code: PROTOCOLERROR}
			}

			// todo if there's a worker dedicated, set its flag to "RESET_STREAM"
		case SETTINGS:
			if err = h.readSettings(frame); err != nil {
				return frame, err
			}
		case PING:
			if frame.Is(FACK) {
				if err = h.client.Skip(frame.Length); err != nil {
					return frame, err
				}

				continue
			}

			data, err := h.readPing(frame)
			if err != nil {
				return frame, err
			}

			pingback := Frame{
				Type:   PING,
				Flags:  FACK,
				Length: 8,
				Stream: frame.Stream,
			}
			if err = h.serializer.Write(pingback, data[:]); err != nil {
				return frame, err
			}
		case GOAWAY:
			lastStream, errcode, debug, err := h.readGoAway(frame)
			if err != nil {
				return frame, err
			}

			fmt.Println("going away!", lastStream, errcode, strconv.Quote(debug))
		case WINDOWUPDATE:
			increment, err := h.readWindowUpdate(frame)
			if err != nil {
				return frame, err
			}

			fmt.Println("window increment:", increment)

			_ = increment
			//panic("not implemented!")
		default:
			// https://datatracker.ietf.org/doc/html/rfc9113#section-4.1-4.4.1:
			// Implementations MUST ignore and discard frames of unknown types.
			if err = h.client.Skip(frame.Length); err != nil {
				return frame, err
			}
		}
	}
}

func (h *HTTP2) rerouteData(worker *worker, frame Frame) (err error) {
	h.client.Bump(FrameOctets)
	segment := h.client.Tail(FrameOctets)
	n := 0
	if !worker.Mu.TryLock() {
		h.rollData(worker, segment)
		return nil
	}

	h.client.Alloc()

	for uint32(n) < frame.Length {
		worker.Mu.Unlock()

		data, err := h.client.ReadAtMost(frame.Length)
		if err != nil {
			// todo CRITICAL: notify the worker about error
			//      or not? the connection is dead anyway
			return err
		}

		h.client.Bump(len(data))
		n += len(data)

		if !worker.Mu.TryLock() {
			h.rollData(worker, h.client.Tail(FrameOctets+n))
			return nil
		}
	}

	// todo i really must set a flag ENDSTREAM.
	worker.Mailbox = append(worker.Mailbox, h.client.Tail(FrameOctets+n))
	worker.Mu.Unlock()
	return nil
}

func (h *HTTP2) rollData(worker *worker, pending []byte) {
	worker.C <- pending
	<-worker.C
}

func (h *HTTP2) readFrame() (Frame, error) {
	var headers [FrameOctets]byte
	err := h.client.ReadFull(headers[:])
	return FrameFrom(headers), err
}

func (h *HTTP2) readWindowUpdate(frame Frame) (increment uint32, err error) {
	/*
		WINDOW_UPDATE Frame {
		  Length (24) = 0x04,
		  Type (8) = 0x08,

		  Unused Flags (8),

		  Reserved (1),
		  Stream Identifier (31),

		  Reserved (1),
		  Window Size Increment (31),
		}
	*/

	if frame.Length != 4 {
		return 0, &Error{false, PROTOCOLERROR}
	}

	increment, err = h.readu32be()
	increment &= unsetUpperBit
	return increment, err
}

func (h *HTTP2) readPing(frame Frame) ([8]byte, error) {
	/*
		PING Frame {
		  Length (24) = 0x08,
		  Type (8) = 0x06,

		  Unused Flags (7),
		  ACK Flag (1),

		  Reserved (1),
		  Stream Identifier (31) = 0,

		  Opaque Data (64),
		}
	*/

	const opaqueBytes = 8
	var opaqueData [opaqueBytes]byte

	if frame.Length != opaqueBytes {
		return [8]byte{}, &Error{false, PROTOCOLERROR}
	}

	err := h.client.ReadFull(opaqueData[:])
	return opaqueData, err
}

func (h *HTTP2) readRstStream(frame Frame) (code ErrorCode, err error) {
	/*
		RST_STREAM Frame {
		  Length (24) = 0x04,
		  Type (8) = 0x03,

		  Unused Flags (8),

		  Reserved (1),
		  Stream Identifier (31),

		  Error Code (32),
		}
	*/

	if frame.Length != 4 {
		return 0, &Error{false, PROTOCOLERROR}
	}

	return h.readu32be()
}

func (h *HTTP2) readGoAway(frame Frame) (lastStream, errcode uint32, debug string, err error) {
	/*
		GOAWAY Frame {
		  Length (24),
		  Type (8) = 0x07,

		  Unused Flags (8),

		  Reserved (1),
		  Stream Identifier (31) = 0,

		  Reserved (1),
		  Last-Stream-ID (31),
		  Error Code (32),
		  Additional Debug Data (..),
		}
	*/

	if frame.Stream != 0 {
		return 0, 0, "", &Error{false, PROTOCOLERROR}
	}

	// 32 bits for Last-Stream-ID and 32 bits for Error Code.
	// The rest of the length is the actual Additional Debug Data.
	const requiredFieldsLen = 4 + 4
	if frame.Length < requiredFieldsLen {
		return 0, 0, "", &Error{false, PROTOCOLERROR}
	}

	lastStream, err = h.readu32be()
	if err != nil {
		return 0, 0, "", err
	}
	lastStream &= unsetUpperBit

	errcode, err = h.readu32be()
	if err != nil {
		return 0, 0, "", err
	}

	// todo limit debug data maximal length
	debugdata := make([]byte, frame.Length-requiredFieldsLen)
	err = h.client.ReadFull(debugdata)

	return lastStream, errcode, uf.B2S(debugdata), err
}

func (h *HTTP2) readPriority(frame Frame) error {
	/*
		PRIORITY Frame {
		  Length (24) = 0x05,
		  Type (8) = 0x02,

		  Unused Flags (8),

		  Reserved (1),
		  Stream Identifier (31),

		  Exclusive (1),
		  Stream Dependency (31),
		  Weight (8),
		}
	*/

	if frame.Stream == 0 {
		return &Error{false, PROTOCOLERROR}
	}

	const priorityBytes = 5
	if frame.Length != priorityBytes {
		return &Error{true, FRAMESIZEERROR}
	}

	return h.client.Skip(priorityBytes)
}

func (h *HTTP2) readHeaders(frame Frame, request *http.Request) (err error) {
	/*
		HEADERS Frame {
		  Length (24),
		  Type (8) = 0x01,

		  Unused Flags (2),
		  PRIORITY Flag (1),
		  Unused Flag (1),
		  PADDED Flag (1),
		  END_HEADERS Flag (1),
		  Unused Flag (1),
		  END_STREAM Flag (1),

		  Reserved (1),
		  Stream Identifier (31),

		  [Pad Length (8)],
		  [Exclusive (1)],
		  [Stream Dependency (31)],
		  [Weight (8)],
		  Field Block Fragment (..),
		  Padding (..2040),
		}
	*/

	length := int(frame.Length)
	padding := uint32(0)

	if frame.Is(FPADDED) {
		b, err := h.client.ReadByte()
		if err != nil {
			return err
		}

		padding = uint32(b)
		length -= int(b) + 1
	}

	if frame.Is(FPRIORITY) {
		const priorityOctets = 5 // Exclusive + Stream Dependency + Weight = 40 bits
		if err = h.client.Skip(priorityOctets); err != nil {
			return err
		}

		length -= priorityOctets
	}

	h.client.Limit(length)
	for {
		pair, err := h.readHeader()
		switch err {
		case nil:
		case ErrLimit:
			return h.client.Skip(padding)
		default:
			return err
		}

		if pair.Empty() {
			// it might have been just the decoder table resizing. It yields no pair.
			continue
		}

		switch pair.Key {
		case ":method":
			request.Method = method.Parse(pair.Value)
			continue
		case ":path":
			request.Path = pair.Value
			continue
		case ":authority":
			pair.Key = "Host"
		case ":scheme":
			// conveys no information. It's already known whether the connection is encrypted or
			// plain-text via the http.Request.Env.Encryption field.
			continue
		}

		request.Headers.Add(pair.Key, pair.Value)
	}
}

func (h *HTTP2) readHeader() (kv.Pair, error) {
	const (
		indexed                byte = 0x80
		indexedLiteral         byte = 0x40
		dynamicTableSizeUpdate byte = 0x20
		neverIndexedLiteral    byte = 0x10
	)

	b, err := h.client.ReadByte()
	if err != nil {
		return kv.Pair{}, err
	}

	switch {
	case b&indexed != 0:
		// full key-value pair in the decoder table. Index = IntRepr 7+. Index 0 = decoding error
		index, err := h.readIntRepr(b, 7)
		if err != nil {
			return kv.Pair{}, err
		}

		if index == 0 {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		pair, ok := h.dtable.Decode(index)
		if !ok {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		return pair, nil
	case b&indexedLiteral != 0:
		pair, err := h.readIndexedField(b)
		if err == nil {
			h.dtable.Insert(pair.Key, pair.Value)
		}

		return pair, err
	case b&dynamicTableSizeUpdate != 0:
		newsize, err := h.readIntRepr(b, 5)
		if err != nil {
			return kv.Pair{}, err
		}

		if newsize > h.settings[SHEADERTABLESIZE] {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		h.dtable.Resize(newsize)
		return kv.Pair{}, nil
	case b&neverIndexedLiteral != 0:
		// todo does the user want to know whether a field is never-indexed? Or simply store it somewhere internally?
		return h.readIndexedField(b)
	default:
		// Literal Header Field without Indexing
		return h.readIndexedField(b)
	}
}

func (h *HTTP2) readIndexedField(intro byte) (kv.Pair, error) {
	index, err := h.readIntRepr(intro, 6)
	if err != nil {
		return kv.Pair{}, err
	}

	var key, value string

	if index == 0 {
		key, err = h.readLiteral()
		if err != nil {
			return kv.Pair{}, err
		}
	} else {
		entry, ok := h.dtable.Decode(index)
		if !ok {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		key = entry.Key
	}

	value, err = h.readLiteral()

	return kv.Pair{Key: key, Value: value}, err
}

func (h *HTTP2) readLiteral() (string, error) {
	/*
	     0   1   2   3   4   5   6   7
	   +---+---+---+---+---+---+---+---+
	   | H |    String Length (7+)     |
	   +---+---------------------------+
	   |  String Data (Length octets)  |
	   +-------------------------------+
	*/

	intro, err := h.client.ReadByte()
	if err != nil {
		return "", err
	}

	length, err := h.readIntRepr(intro, 7)
	if err != nil {
		return "", err
	}

	// todo: limit the maximal string length

	literal := make([]byte, length)
	if err = h.client.ReadFull(literal); err != nil {
		return "", err
	}

	if intro&0x80 != 0 {
		buff := make([]byte, 0, length*2)
		decompressed, ok := hpack.Decompress(literal, buff)
		if !ok {
			return "", &Error{true, COMPRESSIONERROR}
		}

		literal = decompressed
	}

	return uf.B2S(literal), nil
}

func (h *HTTP2) readSettings(frame Frame) error {
	/*
		SETTINGS Frame {
		  Length (24),
		  Type (8) = 0x04,

		  Unused Flags (7),
		  ACK Flag (1),

		  Reserved (1),
		  Stream Identifier (31) = 0,

		  Setting (48) ...,
		}

		Setting {
		  Identifier (16),
		  Value (32),
		}
	*/

	if frame.Flags&FACK != 0 {
		if frame.Length > 0 {
			return &Error{false, FRAMESIZEERROR}
		}
	}

	if frame.Stream != 0 {
		return &Error{false, PROTOCOLERROR}
	}

	const pair = 2 + 4
	i := frame.Length
	for ; i >= pair; i -= pair {
		var setting [pair]byte
		err := h.client.ReadFull(setting[:])
		if err != nil {
			return err
		}

		key := binary.BigEndian.Uint16(setting[:2])
		value := binary.BigEndian.Uint32(setting[2:6])

		if key >= uint16(len(h.settings)) {
			continue
		}

		h.settings[key] = value
	}

	if i > 0 {
		return &Error{false, PROTOCOLERROR}
	}

	return nil
}

func (h *HTTP2) readu32be() (uint32, error) {
	var num [4]byte
	if err := h.client.ReadFull(num[:]); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(num[:]), nil
}

func (h *HTTP2) readIntRepr(prefix byte, prefixlen int) (num uint32, err error) {
	mask := byte(1)<<prefixlen - 1
	prefix &= mask
	if prefix < mask {
		return uint32(prefix), nil
	}

	shift := 0
	for range 3 {
		b, err := h.client.ReadByte()
		if err != nil {
			return 0, err
		}

		num = (uint32(b&0x7f) << shift) | num
		if b&0x80 == 0 {
			return num + uint32(prefix), nil
		}

		shift += 7
	}

	// stop reading after 8 bytes (full uint64).
	// fixme: malformed intrepr is a protocol error? Stream or connection error?
	return 0, &Error{true, PROTOCOLERROR}
}

func (h *HTTP2) readPreface() error {
	preface := "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	const prefaceOctets = 24 // essentially len(preface), but Go won't allow to use it as a constant initializer.
	var data [prefaceOctets]byte

	if h.client.ReadFull(data[:]) != nil {
		return &Error{false, CONNECTERROR}
	}

	if uf.B2S(data[:]) != preface {
		return &Error{false, PROTOCOLERROR}
	}

	return nil
}
