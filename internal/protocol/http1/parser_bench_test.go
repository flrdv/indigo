package http1

import (
	"github.com/indigo-web/indigo/config"
	"github.com/indigo-web/indigo/internal/requestgen"
	"strings"
	"testing"
)

func BenchmarkParser(b *testing.B) {
	parser, request := getParser(config.Default())

	b.Run("with 5 headers", func(b *testing.B) {
		data := requestgen.Generate(strings.Repeat("a", 500), requestgen.Headers(5))
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, _ = parser.Parse(data)
			b.ReportAllocs()
			_ = request.Reset()
		}
	})

	b.Run("with 10 headers", func(b *testing.B) {
		data := requestgen.Generate(strings.Repeat("a", 500), requestgen.Headers(10))
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, _ = parser.Parse(data)
			_ = request.Reset()
		}
	})

	b.Run("with 50 headers", func(b *testing.B) {
		data := requestgen.Generate(strings.Repeat("a", 500), requestgen.Headers(50))
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, _ = parser.Parse(data)
			_ = request.Reset()
		}
	})

	b.Run("escaped 10 headers", func(b *testing.B) {
		data := requestgen.Generate(strings.Repeat("%20", 500), requestgen.Headers(10))
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, _ = parser.Parse(data)
			_ = request.Reset()
		}
	})
}
