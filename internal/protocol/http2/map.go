package http2

import (
	"math/bits"
)

// hmap keeps track of assigned workers/goroutines.
//
// hmap is a simple open-addressing hashmap. It addresses using n lower bits of a stream ID (an uint32),
// where n is log2 of map capacity (map capacity is aligned to the nearest power of two). Insertion and
// lookup are straightforward: start at `stream_id % map_capacity` and advance linearly until an unoccupied
// slot appears.
type hmap struct {
	len   uint32
	cap   uint32
	slots []*worker
}

func newMap(capacity uint32) hmap {
	return hmap{
		cap:   capacity,
		slots: make([]*worker, roundToNextPowerOfTwo(capacity)),
	}
}

func (h *hmap) Assign(worker *worker) (ok bool) {
	if h.len >= h.cap {
		return false
	}

	mask := h.mask()
	idx := worker.ID & mask
	entry := h.slots[idx]

	// just in case: the loop is guaranteed to halt! Due to the early-exit
	for ; entry != nil; idx++ {
		entry = h.slots[idx&mask]
	}

	h.len++
	h.slots[idx&mask] = worker
	return true
}

func (h *hmap) Get(key uint32) *worker {
	if _, w := h.find(key); w != nil {
		return w
	}

	return nil
}

func (h *hmap) Delete(key uint32) {
	if idx, _ := h.find(key); idx != -1 {
		h.len--
		h.slots[key] = nil
	}
}

// find iterates over the entire map as a ring buffer and seeks for the matching slot.
func (h *hmap) find(key uint32) (int, *worker) {
	k := key
	mask := h.mask()

	for i := 0; i < len(h.slots); i++ {
		slot := h.slots[k&mask]
		if slot != nil && slot.ID == key {
			return int(k & mask), slot
		}
		k++
	}

	return -1, nil
}

func (h *hmap) mask() uint32 {
	// can't use h.cap because the cap field isn't rounded up.
	return uint32(len(h.slots)) - 1
}

func roundToNextPowerOfTwo(num uint32) uint32 {
	if isPowerOfTwo(num) {
		return num
	}

	return 1 << bits.Len(uint(num))
}

func isPowerOfTwo(num uint32) bool {
	return num&(num-1) == 0
}
