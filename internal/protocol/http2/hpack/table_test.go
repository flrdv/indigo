package hpack

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/indigo-web/indigo/kv"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	t.Run("write 2 strings", func(t *testing.T) {
		s := newStorage(10)
		hello := s.Write("hello")
		world := s.Write("world")
		require.Equal(t, "hello", s.Read(hello))
		require.Equal(t, "world", s.Read(world))
	})

	t.Run("wrapped", func(t *testing.T) {
		s := newStorage(10)
		hello := s.Write("hello")
		world := s.Write("world!")
		require.Equal(t, "!ello", s.Read(hello))
		require.Equal(t, "world!", s.Read(world))
	})

	t.Run("cached wrapped", func(t *testing.T) {
		s := newStorage(10)
		hello := s.Write("hello")
		world := s.Write("world!")
		require.Equal(t, "!ello", s.Read(hello))
		require.Equal(t, "world!", s.Read(world))
		// read already cached
		require.Equal(t, "world!", s.Read(world))

		// stale the cache
		helloworld := s.Write("helloworld")
		require.Equal(t, "helloworld", s.Read(helloworld))
	})

	t.Run("copy to foregoing", func(t *testing.T) {
		s := newStorage(13)
		str := s.Read(s.Write("helloworld"))
		require.Equal(t, "helloworld", s.Read(s.Write(str)))
	})
}

func TestTable(t *testing.T) {
	wantpair := func(t *testing.T, table Table, idx uint32, key, value string) {
		pair, ok := table.Decode(idx)
		require.True(t, ok)
		require.Equal(t, kv.Pair{Key: key, Value: value}, pair)
	}

	dontwantpair := func(t *testing.T, table Table, idx uint32) {
		_, ok := table.Decode(idx)
		require.False(t, ok)
	}

	t.Run("static space", func(t *testing.T) {
		table := NewTable(0)
		pair, ok := table.Decode(1)
		require.True(t, ok)
		require.Equal(t, StaticTable[0], pair)

		pair, ok = table.Decode(61)
		require.True(t, ok)
		require.Equal(t, StaticTable[60], pair)
	})

	t.Run("insert", func(t *testing.T) {
		table := NewTable(128)
		table.Insert("hello", "world")
		wantpair(t, table, 62, "hello", "world")
		table.Insert("Slava", "Ukraini")
		wantpair(t, table, 62, "Slava", "Ukraini")
		wantpair(t, table, 63, "hello", "world")
	})

	t.Run("insert huge", func(t *testing.T) {
		table := NewTable(128)
		table.Insert("hello", "world")
		table.Insert("Slava", "Ukraini")

		table.Insert("hello", strings.Repeat("a", 1024))
		require.Equal(t, 0, table.Len())
		table.Insert("hello", "world")
		require.Equal(t, 1, table.Len())
		wantpair(t, table, 62, "hello", "world")
	})

	t.Run("get dynamic out of bounds", func(t *testing.T) {
		table := NewTable(128)
		dontwantpair(t, table, 62)
	})

	t.Run("evict oldest", func(t *testing.T) {
		table := NewTable(80)
		table.Insert("hello", "world")
		table.Insert("foo", "bar")
		table.Insert("lorem", "ipsum")

		wantpair(t, table, 62, "lorem", "ipsum")
		wantpair(t, table, 63, "foo", "bar")
		dontwantpair(t, table, 64)
	})

	t.Run("resize", func(t *testing.T) {
		t.Run("shrink", func(t *testing.T) {
			table := NewTable(128)
			table.Insert("hello", "world")
			table.Insert("Slava", "Ukraini")

			table.Resize(table.len)
			wantpair(t, table, 62, "Slava", "Ukraini")
			wantpair(t, table, 63, "hello", "world")

			table.Resize(table.len - 1)
			wantpair(t, table, 62, "Slava", "Ukraini")
			dontwantpair(t, table, 63)
		})

		t.Run("identity", func(t *testing.T) {
			table := NewTable(128)
			table.Insert("hello", "world")
			table.Insert("Slava", "Ukraini")

			original := unsafe.SliceData(table.storage.data)

			table.Resize(86)
			wantpair(t, table, 62, "Slava", "Ukraini")
			wantpair(t, table, 63, "hello", "world")
			dontwantpair(t, table, 64)

			table.Resize(128)
			wantpair(t, table, 62, "Slava", "Ukraini")
			wantpair(t, table, 63, "hello", "world")
			dontwantpair(t, table, 64)

			require.Equal(t, original, unsafe.SliceData(table.storage.data), "identity resize must not allocate")
		})

		t.Run("grow", func(t *testing.T) {
			table := NewTable(86)
			for range 4 {
				// by looping 4 times, "Slava Ukraini" wraps in the storage. Additional corner-case check.
				table.Insert("hello", "world")
				table.Insert("Slava", "Ukraini")
			}

			table.Resize(128)
			table.Insert("hallo", "welt")
			wantpair(t, table, 62, "hallo", "welt")
			wantpair(t, table, 63, "Slava", "Ukraini")
			wantpair(t, table, 64, "hello", "world")
		})
	})
}
