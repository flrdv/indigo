package http2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHuffman(t *testing.T) {
	test := func(str string) func(t *testing.T) {
		return func(t *testing.T) {
			// identity test. Both must cancel each other out.
			decompressed, ok := Decompress(Compress(str, nil), []byte{})
			require.True(t, ok)
			require.Equal(t, str, string(decompressed))
		}
	}

	t.Run("single frequent letter", test("a"))

	t.Run("single infrequent letter", test("\x00"))

	t.Run("short string", test("abcdef"))

	t.Run("long string", test(strings.Repeat("abcdef", 100)))

	t.Run("long string of infrequent chars", test(strings.Repeat("\x00\xfa\xfb\xfc\xfd", 100)))

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
	strN := func(n int) string {
		return strings.Repeat("a!$\n", n/4)
	}

	benchCompress := func(n int) func(b *testing.B) {
		return func(b *testing.B) {
			data := strN(n)
			out := make([]byte, 0, len(data))
			b.SetBytes(int64(len(data)))
			b.ResetTimer()

			for range b.N {
				_ = Compress(data, out[:0])
			}
		}
	}

	benchDecompress := func(n int) func(b *testing.B) {
		return func(b *testing.B) {
			compressed := Compress(strN(n), []byte{})
			out := make([]byte, 0, n)
			b.SetBytes(int64(len(compressed)))
			b.ResetTimer()

			for range b.N {
				_, _ = Decompress(compressed, out[:0])
			}
		}
	}

	b.Run("compress 64", benchCompress(64))
	b.Run("decompress 64", benchDecompress(64))

	b.Run("compress 4096", benchCompress(4096))
	b.Run("decompress 4096", benchDecompress(4096))

	b.Run("compress 65536", benchCompress(65536))
	b.Run("decompress 65536", benchDecompress(65536))
}
