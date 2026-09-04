package http2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkMap(b *testing.B) {
	const capacity = 128
	workers := []*worker{
		{ID: 15},
		{ID: 16},
		{ID: 65},
		{ID: 120},
	}
	indices := []uint32{15, 16, 65, 120}

	diymap := newMap(capacity)
	stdmap := make(map[uint32]*worker)

	for _, w := range workers {
		diymap.Assign(w)
		stdmap[w.ID] = w
	}

	b.Run("lookup", func(b *testing.B) {
		for i := range b.N {
			_ = diymap.Get(indices[i&4])
		}
	})

	b.Run("std", func(b *testing.B) {
		for i := range b.N {
			_, _ = stdmap[indices[i&4]]
		}
	})
}

func TestMap(t *testing.T) {
	value := func(id uint32) *worker {
		return &worker{ID: id}
	}

	t.Run("lookup", func(t *testing.T) {
		m := newMap(16)

		require.True(t, m.Assign(value(12)))

		require.NotNil(t, m.Get(12))
		require.Nil(t, m.Get(11))
	})

	t.Run("delete", func(t *testing.T) {
		m := newMap(16)
		require.True(t, m.Assign(value(12)))
		m.Delete(12)
		require.Nil(t, m.Get(12))
	})

	t.Run("collisions", func(t *testing.T) {
		m := newMap(16)
		for i := range 15 {
			require.True(t, m.Assign(value(uint32(i+1))))
		}

		require.True(t, m.Assign(value(0xf7)))
		require.Equal(t, uint32(0xf7), m.Get(0xf7).ID)
	})

	t.Run("size limit", func(t *testing.T) {
		m := newMap(16)
		for i := range 16 {
			require.True(t, m.Assign(value(uint32(i+1))))
		}

		require.False(t, m.Assign(value(0xf7)))
	})
}
