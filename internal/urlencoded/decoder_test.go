package urlencoded

import (
	"github.com/indigo-web/indigo/http/status"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func testDecode(t *testing.T, decode func([]byte) ([]byte, error)) {
	t.Run("no escaping", func(t *testing.T) {
		str := []byte("/hello")
		decoded, err := Decode(str)
		require.NoError(t, err)
		require.Equal(t, "/hello", string(decoded))
	})

	t.Run("corners", func(t *testing.T) {
		str := []byte("%2fhello%2f")
		decoded, err := Decode(str)
		require.NoError(t, err)
		require.Equal(t, "/hello/", string(decoded))
	})

	t.Run("multiple consecutive", func(t *testing.T) {
		str := []byte("%2f%20hello")
		decoded, err := Decode(str)
		require.NoError(t, err)
		require.Equal(t, "/ hello", string(decoded))
	})

	t.Run("incomplete sequence", func(t *testing.T) {
		str := []byte("%2")
		_, err := Decode(str)
		require.EqualError(t, err, status.ErrURLDecoding.Error())
	})

	t.Run("invalid code", func(t *testing.T) {
		str := []byte("%2j")
		_, err := Decode(str)
		require.EqualError(t, err, status.ErrURLDecoding.Error())
	})

	t.Run("4kb slightly escaped", func(t *testing.T) {
		str := []byte("/" + disperse("%5f", "a", 10, 4095))
		decoded, err := Decode(str)
		require.NoError(t, err)
		want := "/" + strings.Repeat("_"+strings.Repeat("a", 10), 4095/len("%5f"+strings.Repeat("a", 10)))
		require.Equal(t, want, string(decoded))
		require.Equal(t, 4096, cap(decoded))
	})
}

func TestDecode(t *testing.T) {
	testDecode(t, Decode)
}

func TestLazyDecode(t *testing.T) {
	testDecode(t, func(bytes []byte) ([]byte, error) {
		data, _, err := LazyDecode(bytes, nil)
		return data, err
	})
}

func BenchmarkDecode(b *testing.B) {
	bench := func(b *testing.B, segment string) {
		str := []byte("/" + strings.Repeat(segment, 4095/len(segment)))
		b.SetBytes(int64(len(str)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = Decode(str)
		}
	}

	b.Run("4kb unescaped", func(b *testing.B) {
		bench(b, "a")
	})

	b.Run("4kb slightly escaped", func(b *testing.B) {
		// one urlencoded part per 10 decoded characters
		bench(b, "%5faaaaaaaaa")
	})

	b.Run("4kb half escaped", func(b *testing.B) {
		bench(b, "%5fa")
	})

	b.Run("4kb only escaped", func(b *testing.B) {
		bench(b, "%5f")
	})
}

// disperse makes a string, which consists of 1:proportion substrings a and b respectfully.
// Repeating them doesn't always result in exactly desiredLen bytes
func disperse(a, b string, proportion, desiredLen int) string {
	return strings.Repeat(a+strings.Repeat(b, proportion), desiredLen/(len(a)+len(b)*proportion))
}
