package http2

import (
	"io"
	"sync"

	"github.com/indigo-web/indigo/internal/bit"
)

const (
	wstate0 uint8 = iota
	wstateFrameread
	wstateEndstream
	wstateNoData
)

// worker is an assignation of a goroutine to a particular request.
type worker struct {
	State    bit.Map8
	ID       uint32
	h2       *HTTP2
	Mu       sync.Mutex
	C        chan []byte
	Mailbox  [][]byte
	framelen uint32
	padlen   uint32
}

func newWorker(id uint32, h2 *HTTP2) *worker {
	return &worker{
		ID:      id,
		h2:      h2,
		C:       make(chan []byte),
		Mailbox: make([][]byte, 0, h2.cfg.HTTP2.MailboxPrealloc),
	}
}

func (w *worker) Fetch() ([]byte, error) {
	if w.State.Is(wstateNoData) {
		return nil, io.EOF
	}

	if !w.State.Is(wstate0) {
		w.State.Clear()
		w.State.Set(wstate0)
		w.Mu.Lock()
		w.h2.client.AddSource(w.Mailbox, w.C)
	}

begin:
	if !w.State.Is(wstateFrameread) {
		w.State.Set(wstateFrameread)

		frame, err := w.h2.Process(w)
		if err != nil {
			return nil, err
		}

		switch frame.Type {
		case DATA, CONTINUATION:
			/*
				DATA Frame {
				  Length (24),
				  Type (8) = 0x00,

				  Unused Flags (4),
				  PADDED Flag (1),
				  Unused Flags (2),
				  END_STREAM Flag (1),

				  Reserved (1),
				  Stream Identifier (31),

				  [Pad Length (8)],
				  Data (..),
				  Padding (..2040),
				}
			*/

			w.framelen = frame.Length
			if frame.Is(FPADDED) {
				b, err := w.h2.client.ReadByte()
				if err != nil {
					return nil, err
				}

				w.padlen = uint32(b)
				w.framelen -= uint32(b) + 1
			}
			if frame.Is(FENDSTREAM) {
				w.State.Set(wstateEndstream)
			}
		// todo: handle RST_STREAM
		// todo: handle HEADERS
		default:
			// todo how exactly to error when I get a different frame?
			return nil, &Error{Stream: false, Code: PROTOCOLERROR}
		}
	}

	if w.framelen > 0 {
		data, err := w.h2.client.ReadAtMost(w.framelen)
		w.framelen -= uint32(len(data))

		return data, err
	}

	if err := w.h2.client.Skip(w.padlen); err != nil {
		return nil, err
	}

	w.State.Unset(wstateFrameread)

	if w.State.Is(wstateEndstream) {
		// todo peek the next frame, it might be HEADERS carrying trailer
		w.Mu.Unlock()
		return nil, io.EOF
	}

	goto begin
}
