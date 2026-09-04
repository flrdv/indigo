package http2

import (
	"encoding/binary"
	"math"
)

//go:generate stringer -type=FrameType
type FrameType uint8

const (
	DATA         FrameType = 0x00
	HEADERS      FrameType = 0x01
	PRIORITY     FrameType = 0x02
	RSTSTREAM    FrameType = 0x03
	SETTINGS     FrameType = 0x04
	PUSHPROMISE  FrameType = 0x05
	PING         FrameType = 0x06
	GOAWAY       FrameType = 0x07
	WINDOWUPDATE FrameType = 0x08
	CONTINUATION FrameType = 0x09
)

// FrameOctets is the number of bytes transmitted physically (not a single idea
// how is the struct packed)
const FrameOctets = 9

type Frame struct {
	Type   FrameType
	Flags  uint8
	Length uint32
	Stream uint32
}

func FrameFrom(data [FrameOctets]byte) Frame {
	return Frame{
		Length: uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2]),
		Type:   FrameType(data[3]),
		Flags:  data[4],
		Stream: binary.BigEndian.Uint32(data[5:9]) & unsetUpperBit,
	}
}

const (
	FPRIORITY   uint8 = 0b00100000
	FPADDED     uint8 = 0b00001000
	FENDHEADERS uint8 = 0b00000100
	FENDSTREAM  uint8 = 0b00000001
	FACK        uint8 = 0b00000001
)

// Is tests whether the passed flag is set.
func (f Frame) Is(flag byte) bool {
	return f.Flags&flag != 0
}

// AltersState tells whether the frame alters a connection's state.
func (f Frame) AltersState() bool {
	// https://datatracker.ietf.org/doc/html/rfc9113#section-4.2-4:
	// An endpoint MUST send an error code of FRAME_SIZE_ERROR if a frame exceeds
	// the size defined in SETTINGS_MAX_FRAME_SIZE, exceeds any limit defined for
	// the frame type, or is too small to contain mandatory frame data. A frame
	// size error in a frame that could alter the state of the entire connection
	// MUST be treated as a connection error (Section 5.4.1); this includes any
	// frame carrying a field block (Section 4.3) (that is, HEADERS, PUSH_PROMISE,
	// and CONTINUATION), a SETTINGS frame, and any frame with a stream identifier of 0.

	switch f.Type {
	case HEADERS, CONTINUATION, SETTINGS:
		return true
	}

	return f.Stream == 0
}

func (f Frame) Serialize() [FrameOctets]byte {
	return [FrameOctets]byte{
		byte(f.Length >> 16),
		byte(f.Length >> 8),
		byte(f.Length),
		byte(f.Type),
		f.Flags,
		byte(f.Stream >> 24),
		byte(f.Stream >> 16),
		byte(f.Stream >> 8),
		byte(f.Stream),
	}
}

type ErrorCode = uint32

const (
	NOERROR            ErrorCode = 0x00
	PROTOCOLERROR      ErrorCode = 0x01
	INTERNALERROR      ErrorCode = 0x02
	FLOWCONTROLERROR   ErrorCode = 0x03
	SETTINGSTIMEOUT    ErrorCode = 0x04
	STREAMCLOSED       ErrorCode = 0x05
	FRAMESIZEERROR     ErrorCode = 0x06
	REFUSEDSTREAM      ErrorCode = 0x07
	CANCEL             ErrorCode = 0x08
	COMPRESSIONERROR   ErrorCode = 0x09
	CONNECTERROR       ErrorCode = 0x0a
	ENHANCEYOURCALM    ErrorCode = 0x0b
	INADEQUATESECURITY ErrorCode = 0x0c
	HTTP11REQUIRED     ErrorCode = 0x0d
)

type Error struct {
	Stream bool
	Code   uint32
}

func (e Error) Error() string {
	if e.Stream {
		return "stream error"
	}

	return "connection error"
}

const (
	SHEADERTABLESIZE      uint16 = 0x01
	SENABLEPUSH           uint16 = 0x02
	SMAXCONCURRENTSTREAMS uint16 = 0x03
	SINITIALWINDOWSIZE    uint16 = 0x04
	SMAXFRAMESIZE         uint16 = 0x05
	SMAXHEADERLISTSIZE    uint16 = 0x06
)

type Settings [0x07]uint32

var defaultSettings = Settings{
	SHEADERTABLESIZE:      4096,
	SENABLEPUSH:           0,
	SMAXCONCURRENTSTREAMS: 128,
	SINITIALWINDOWSIZE:    1<<16 - 1,
	SMAXFRAMESIZE:         1 << 14,
	SMAXHEADERLISTSIZE:    math.MaxUint32,
}
