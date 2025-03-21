package huffman

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestHuffman(t *testing.T) {
	test := func(t *testing.T, str string) {
		// test reversibility. Both functions must decline each other and produce
		// identity-function, otherwise can be considered incompatible, which is in
		// fact a failed test.
		decompressed, ok := Decompress(Compress(str, nil), []byte{})
		require.True(t, ok)
		require.Equal(t, str, string(decompressed))
	}

	t.Run("single frequent letter", func(t *testing.T) {
		test(t, "a")
	})

	t.Run("single infrequent letter", func(t *testing.T) {
		test(t, "\x00")
	})

	t.Run("short string", func(t *testing.T) {
		test(t, "abcdef")
	})

	t.Run("long string", func(t *testing.T) {
		test(t, strings.Repeat("abcdef", 100))
	})

	t.Run("long string of infrequent chars", func(t *testing.T) {
		test(t, strings.Repeat("\x00\xfa\xfb\xfc\xfd", 100))
	})

	t.Run("invalid code", func(t *testing.T) {
		// single bit in the end is zero
		_, ok := Decompress([]byte{0b11111111, 0b11111111, 0b11111001, 0b10111011}, []byte{})
		require.False(t, ok)

		// has no free bits at all
		_, ok = Decompress([]byte{0b00011000, 0b11000110, 0b00111000, 0b11100011}, []byte{})
		require.True(t, ok)
	})
}

func Benchmark(b *testing.B) {
	short := strings.Repeat("a!$\r", 64/4)
	shortCompressed := Compress(short, []byte{})
	medium := strings.Repeat("a!$\n", 4096/4)
	mediumCompressed := Compress(medium, []byte{})
	long := strings.Repeat("a!$\n", 65536/4)
	longCompressed := Compress(long, []byte{})
	out := make([]byte, 0, len(long))

	b.Run("ok", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = newPtrTree()
		}
	})

	b.Run("compress 64", func(b *testing.B) {
		b.SetBytes(int64(len(short)))
		b.ResetTimer()

		for range b.N {
			_ = Compress(short, out)
		}
	})

	b.Run("decompress 64", func(b *testing.B) {
		b.SetBytes(int64(len(short)))
		b.ResetTimer()

		for range b.N {
			_, _ = Decompress(shortCompressed, out)
		}
	})

	b.Run("compress 4096", func(b *testing.B) {
		b.SetBytes(int64(len(medium)))
		b.ResetTimer()

		for range b.N {
			_ = Compress(medium, out)
		}
	})

	b.Run("decompress 4096", func(b *testing.B) {
		b.SetBytes(int64(len(medium)))
		b.ResetTimer()

		for range b.N {
			_, _ = Decompress(mediumCompressed, out)
		}
	})

	b.Run("compress 65536", func(b *testing.B) {
		b.SetBytes(int64(len(long)))
		b.ResetTimer()

		for range b.N {
			_ = Compress(long, out)
		}
	})

	b.Run("decompress 65536", func(b *testing.B) {
		b.SetBytes(int64(len(long)))
		b.ResetTimer()

		for range b.N {
			_, _ = Decompress(longCompressed, out)
		}
	})
}
