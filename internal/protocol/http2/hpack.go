package http2

import (
	"slices"

	"github.com/flrdv/uf"
	"github.com/indigo-web/indigo/kv"
)

type Tag struct {
	offset, len int
}

type Pair struct {
	Key, Value Tag
}

func (p Pair) Len() int {
	return p.Key.len + p.Value.len
}

type storage struct {
	cached   bool   // tells whether the boundary contents are still valid
	head     int    // where the most recent entry in data ends
	boundary []byte // due to ring-buffer-nature of data, some value might be halved.
	data     []byte
}

func newStorage(size uint32) storage {
	return storage{
		data: make([]byte, size),
	}
}

func (s *storage) Write(data string) Tag {
	tag := Tag{offset: s.head}

	n := copy(s.data[s.head:], data)
	if n < len(data) {
		// if the string is boundary, invalidate the s.boundary contents
		n += copy(s.data, data[n:])
		s.cached = false
	}

	if s.head += n; s.head >= len(s.data) {
		// faster than modulo arithmetic. Would be nicer to always have the size as a power of 2 though.
		s.head -= len(s.data)
	}

	tag.len = n
	return tag
}

func (s *storage) Read(tag Tag) string {
	if len(s.data)-tag.offset >= tag.len {
		return uf.B2S(s.data[tag.offset : tag.offset+tag.len])
	}

	if s.cached {
		return uf.B2S(s.boundary)
	}

	s.boundary = slices.Grow(s.boundary[:0], tag.len)
	s.boundary = append(s.boundary, s.data[tag.offset:]...)
	s.boundary = append(s.boundary, s.data[:tag.offset+tag.len-len(s.data)]...)
	s.cached = true

	return uf.B2S(s.boundary)
}

func (s *storage) Cap() uint32 {
	return uint32(len(s.data))
}

type Table struct {
	cap     uint32 // the maximal space size in bytes
	len     uint32 // the overall occupied space
	head    uint32 // queue's most recent entry
	tail    uint32 // queue's oldest entry
	storage storage
	queue   []Pair
}

func NewTable(capacity uint32) Table {
	return Table{
		cap:     capacity,
		storage: newStorage(capacity),
		queue:   make([]Pair, topQueueSize(capacity)),
	}
}

const entryOverhead = 32

// Insert a new pair into the dynamic table.
func (t *Table) Insert(key, value string) {
	length := uint32(len(key) + len(value) + entryOverhead)

	for t.len+length > t.cap && !t.isEmpty() {
		t.evict()
	}

	if length > t.cap {
		// entries bigger than the table's capacity aren't considered erroneous. They simply leave the table empty.
		return
	}

	t.queue[t.head] = Pair{
		Key:   t.storage.Write(key),
		Value: t.storage.Write(value),
	}

	t.len += length
	if t.head++; t.head >= uint32(len(t.queue)) {
		t.head = 0
	}
}

func (t *Table) isEmpty() bool {
	return t.head == t.tail
}

func (t *Table) evict() {
	t.len -= uint32(t.queue[t.tail].Len()) + entryOverhead
	if t.tail++; t.tail >= uint32(len(t.queue)) {
		t.tail = 0
	}
}

func (t *Table) Read(client *limitedClient) (kv.Pair, error) {
	const (
		indexed                byte = 0x80
		indexedLiteral         byte = 0x40
		dynamicTableSizeUpdate byte = 0x20
		neverIndexedLiteral    byte = 0x10
	)

	b, err := client.ReadByte()
	if err != nil {
		return kv.Pair{}, err
	}

	switch {
	case b&indexed != 0:
		// full key-value pair in the decoder table. Index = IntRepr 7+. Index 0 = decoding error
		index, err := t.readIntRepr(client, b, 7)
		if err != nil {
			return kv.Pair{}, err
		}

		if index == 0 {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		pair, ok := t.Decode(index)
		if !ok {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		return pair, nil
	case b&indexedLiteral != 0:
		pair, err := t.readIndexedField(client, b)
		if err == nil {
			t.Insert(pair.Key, pair.Value)
		}

		return pair, err
	case b&dynamicTableSizeUpdate != 0:
		newsize, err := t.readIntRepr(client, b, 5)
		if err != nil {
			return kv.Pair{}, err
		}

		if newsize > t.cap {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		t.Resize(newsize)
		return kv.Pair{}, nil
	case b&neverIndexedLiteral != 0:
		// todo is the client willing to know whether some field is never-indexed?
		return t.readIndexedField(client, b)
	default:
		// Literal Header Field without Indexing
		return t.readIndexedField(client, b)
	}
}

func (t *Table) readIndexedField(client *limitedClient, intro byte) (kv.Pair, error) {
	index, err := t.readIntRepr(client, intro, 6)
	if err != nil {
		return kv.Pair{}, err
	}

	var key, value string

	if index == 0 {
		key, err = t.readLiteral(client)
		if err != nil {
			return kv.Pair{}, err
		}
	} else {
		entry, ok := t.Decode(index)
		if !ok {
			return kv.Pair{}, &Error{false, COMPRESSIONERROR}
		}

		key = entry.Key
	}

	value, err = t.readLiteral(client)

	return kv.Pair{Key: key, Value: value}, err
}

func (t *Table) readLiteral(client *limitedClient) (string, error) {
	/*
	     0   1   2   3   4   5   6   7
	   +---+---+---+---+---+---+---+---+
	   | H |    String Length (7+)     |
	   +---+---------------------------+
	   |  String Data (Length octets)  |
	   +-------------------------------+
	*/

	intro, err := client.ReadByte()
	if err != nil {
		return "", err
	}

	length, err := t.readIntRepr(client, intro, 7)
	if err != nil {
		return "", err
	}

	// todo: limit the maximal string length

	literal := make([]byte, length)
	if err = client.ReadFull(literal); err != nil {
		return "", err
	}

	if intro&0x80 != 0 {
		buff := make([]byte, 0, length*2)
		decompressed, ok := Decompress(literal, buff)
		if !ok {
			return "", &Error{true, COMPRESSIONERROR}
		}

		literal = decompressed
	}

	return uf.B2S(literal), nil
}

func (t *Table) readIntRepr(client *limitedClient, prefix byte, prefixlen int) (num uint32, err error) {
	mask := byte(1)<<prefixlen - 1
	prefix &= mask
	if prefix < mask {
		return uint32(prefix), nil
	}

	shift := 0
	for range 3 {
		b, err := client.ReadByte()
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

// Decode returns a pair corresponding the given index.
func (t *Table) Decode(index uint32) (kv.Pair, bool) {
	if index <= uint32(len(StaticTable)) {
		return StaticTable[index-1], true
	}

	return t.decodeDyn(index - uint32(len(StaticTable)) - 1)
}

func (t *Table) decodeDyn(index uint32) (kv.Pair, bool) {
	if index >= uint32(t.Len()) {
		return kv.Pair{}, false
	}

	idx := int(t.head) - int(index) - 1
	if idx < 0 {
		idx = len(t.queue) + idx
	}

	pair := t.queue[idx]

	return kv.Pair{
		Key:   t.storage.Read(pair.Key),
		Value: t.storage.Read(pair.Value),
	}, true
}

func (t *Table) Resize(newsize uint32) {
	if t.storage.Cap() < newsize {
		t.migrate(newsize)
		return
	}

	for t.cap = newsize; t.len > t.cap; {
		t.evict()
	}
}

func (t *Table) migrate(newsize uint32) {
	table := NewTable(newsize)
	for i := t.Len(); i >= 0; i-- {
		if i >= len(t.queue) {
			i = 0
		}

		pair, _ := t.decodeDyn(uint32(i))
		table.Insert(pair.Key, pair.Value)
	}

	*t = table
}

// Len returns a number of entries in the table, not the total size in bytes occupied.
func (t *Table) Len() int {
	l := int(t.head) - int(t.tail)
	if l < 0 {
		return len(t.queue) + l
	}

	return l
}

// topQueueSize returns the maximal possible queue size in order to avoid extra allocations. At the cost of
// slight memory overuse, of course.
func topQueueSize(size uint32) uint32 {
	// A key cannot be empty - so it occupies at least 1 byte.
	// A value can be empty - doesn't contribute.
	// => the queue must hold 1 free spot per 33 bytes of occupied space to ensure no reallocations.
	// Dividing the length by 32 yields extra spots in the queue, but that's fine. Add one more element to avoid
	// the corner case of tiny sizes and always have at least one single entry in the queue.
	return size>>5 + 1
}

var StaticTable = [...]kv.Pair{
	{":authority", ""},
	{":method", "GET"},
	{":method", "POST"},
	{":path", "/"},
	{":path", "/index.html"},
	{":scheme", "http"},
	{":scheme", "https"},
	{":status", "200"},
	{":status", "204"},
	{":status", "206"},
	{":status", "304"},
	{":status", "400"},
	{":status", "404"},
	{":status", "500"},
	{"accept-charset", ""},
	{"accept-encoding", "gzip, deflate"},
	{"accept-language", ""},
	{"accept-ranges", ""},
	{"accept", ""},
	{"access-control-allow-origin", ""},
	{"age", ""},
	{"allow", ""},
	{"authorization", ""},
	{"cache-control", ""},
	{"content-disposition", ""},
	{"content-encoding", ""},
	{"content-language", ""},
	{"content-length", ""},
	{"content-location", ""},
	{"content-range", ""},
	{"content-type", ""},
	{"cookie", ""},
	{"date", ""},
	{"etag", ""},
	{"expect", ""},
	{"expires", ""},
	{"from", ""},
	{"host", ""},
	{"if-match", ""},
	{"if-modified-since", ""},
	{"if-none-match", ""},
	{"if-range", ""},
	{"if-unmodified-since", ""},
	{"last-modified", ""},
	{"link", ""},
	{"location", ""},
	{"max-forwards", ""},
	{"proxy-authenticate", ""},
	{"proxy-authorization", ""},
	{"range", ""},
	{"referer", ""},
	{"refresh", ""},
	{"retry-after", ""},
	{"server", ""},
	{"set-cookie", ""},
	{"strict-transport-security", ""},
	{"transfer-encoding", ""},
	{"user-agent", ""},
	{"vary", ""},
	{"via", ""},
	{"www-authenticate", ""},
}
