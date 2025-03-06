package huffman

import (
	"github.com/stretchr/testify/require"
	"strconv"
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

func BenchmarkDecompressCommon(b *testing.B) {
	shortSrc := strings.Repeat("abcd", 64/len("abcd"))
	short := Compress(shortSrc, []byte{})
	mediumSrc := strings.Repeat("abcd", 4096/len("abcd"))
	medium := Compress(mediumSrc, []byte{})
	longSrc := strings.Repeat("abcd", 65536/len("abcd"))
	long := Compress(longSrc, []byte{})
	out := make([]byte, 0, len(longSrc))

	b.Run(strconv.Itoa(len(shortSrc)), func(b *testing.B) {
		b.SetBytes(int64(len(short)))
		b.ResetTimer()

		for range b.N {
			_, _ = Decompress(short, out)
		}
	})

	b.Run(strconv.Itoa(len(mediumSrc)), func(b *testing.B) {
		b.SetBytes(int64(len(medium)))
		b.ResetTimer()

		for range b.N {
			_, _ = Decompress(medium, out)
		}
	})

	b.Run(strconv.Itoa(len(longSrc)), func(b *testing.B) {
		b.SetBytes(int64(len(long)))
		b.ResetTimer()

		for range b.N {
			_, _ = Decompress(long, out)
		}
	})
}

func BenchmarkCompressCommon(b *testing.B) {
	short := strings.Repeat("abcd", 64/len("abcd"))
	medium := strings.Repeat("abcd", 4096/len("abcd"))
	long := strings.Repeat("abcd", 65536/len("abcd"))
	out := make([]byte, 0, len(long))

	b.Run(strconv.Itoa(len(short)), func(b *testing.B) {
		b.SetBytes(int64(len(short)))
		b.ResetTimer()

		for range b.N {
			_ = Compress(short, out)
		}
	})

	b.Run(strconv.Itoa(len(medium)), func(b *testing.B) {
		b.SetBytes(int64(len(medium)))
		b.ResetTimer()

		for range b.N {
			_ = Compress(medium, out)
		}
	})

	b.Run(strconv.Itoa(len(long)), func(b *testing.B) {
		b.SetBytes(int64(len(long)))
		b.ResetTimer()

		for range b.N {
			_ = Compress(long, out)
		}
	})
}
