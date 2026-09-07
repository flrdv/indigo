package http2

import (
	"io"
	"math"
	"sync"
	"sync/atomic"

	"github.com/indigo-web/indigo/transport"
)

// h2client implements higher-level features for HTTP2.
type h2client struct {
	transport.Client
	allocator

	preserved []byte
	src       readsource
}

func newH2Client(underlying transport.Client, bufferSize uint32) h2client {
	return h2client{
		Client:    underlying,
		allocator: newAllocator(bufferSize),
		src:       readsource{Exhausted: true},
	}
}

func (h *h2client) AddSource(mb [][]byte, c chan []byte) {
	h.src = readsource{
		Exhausted: false,
		C:         c,
		Mailbox:   mb,
	}
}

func (h *h2client) ReadAtMost(n uint32) ([]byte, error) {
	data, err := h.Read()

	chunk := data[:min(n, uint32(len(data)))]
	data = data[len(chunk):]
	h.Pushback(data)

	return chunk, err
}

func (h *h2client) Read() (data []byte, err error) {
	data, h.preserved = h.preserved, nil
	if len(data) == 0 {
		data, err = h.moreData()
	}

	//fmt.Println("read:", strconv.Quote(string(data)))
	return data, err
}

func (h *h2client) moreData() (data []byte, err error) {
	if h.src.Exhausted {
		buff := h.Writeable()
		n, err := h.Client.Read(buff)
		return buff[:n], err
	}

	if len(h.src.Mailbox) > 0 {
		data, h.src.Mailbox = h.src.Mailbox[0], h.src.Mailbox[1:]
		h.Free()
		return data, nil
	}

	h.src.Exhausted = true
	return <-h.src.C, nil
}

func (h *h2client) Pushback(data []byte) {
	h.preserved = data
}

func (h *h2client) Skip(n uint32) error {
	for n > 0 {
		data, err := h.Read()
		if err != nil {
			return err
		}

		delim := min(n, uint32(len(data)))
		h.Pushback(data[delim:])
		n -= delim
	}

	return nil
}

func (h *h2client) ReadByte() (b byte, err error) {
	data, err := h.Read()
	if err != nil {
		return 0, err
	}

	h.Pushback(data[1:])
	return data[0], nil
}

// ReadFull fills the whole destination buffer up to its length.
func (h *h2client) ReadFull(dst []byte) (err error) {
	for len(dst) > 0 {
		data, err := h.Read()
		n := copy(dst, data)
		dst = dst[n:]
		h.Pushback(data[n:])

		if err != nil {
			return err
		}
	}

	return nil
}

type limitedClient struct {
	limit  int
	client *h2client
}

func newLimitedClient(client *h2client, limit int) limitedClient {
	return limitedClient{
		limit:  limit,
		client: client,
	}
}

func (l *limitedClient) Remains() int {
	return l.limit
}

func (l *limitedClient) ReadByte() (byte, error) {
	if l.limit <= 0 {
		return 0, io.ErrUnexpectedEOF
	}

	l.limit--
	return l.client.ReadByte()
}

func (l *limitedClient) ReadFull(dst []byte) error {
	l.limit -= len(dst)
	if l.limit < 0 {
		return io.ErrUnexpectedEOF
	}

	return l.client.ReadFull(dst)
}

type readsource struct {
	Exhausted bool
	C         chan []byte
	Mailbox   [][]byte
}

// allocator is a basic single-region bump-allocator.
type allocator struct {
	wg      sync.WaitGroup
	entries atomic.Uint32
	region  []byte
}

func newAllocator(size uint32) allocator {
	return allocator{
		region: make([]byte, 0, size),
	}
}

func (a *allocator) Alloc() {
	a.entries.Add(1)
	a.wg.Add(1)
}

func (a *allocator) Free() {
	a.entries.Add(math.MaxUint32)
	a.wg.Done()
}

func (a *allocator) Writeable() []byte {
	if len(a.region) == cap(a.region) {
		// if the region is full, just wait for it to empty. This is supposed
		// to be an edge-case and generally be prevented via higher-level window
		// management, so this is rather a backup option.
		a.wg.Wait()
	}

	if len(a.region) > 0 && a.entries.Load() == 0 {
		// start from the beginning once no more entries are available
		a.region = a.region[:0]
	}

	return a.region[len(a.region):cap(a.region)]
}

func (a *allocator) Bump(n int) {
	a.region = a.region[0 : len(a.region)+n]
}

func (a *allocator) Tail(n int) []byte {
	off := len(a.region)
	return a.region[off-n : off]
}
