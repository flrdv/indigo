package inbuilt

import (
	"context"
	"github.com/indigo-web/indigo/http"
	"strings"
	"testing"

	"github.com/indigo-web/indigo/http/method"
)

func BenchmarkRouter_OnRequest_Static(b *testing.B) {
	raw := New()

	GETRootRequest := getRequest(method.GET, "/")
	raw.Get(GETRootRequest.Path, http.Respond)
	longURIRequest := getRequest(method.GET, "/"+strings.Repeat("a", 65534))
	raw.Get(longURIRequest.Path, http.Respond)
	mediumURIRequest := getRequest(method.GET, "/"+strings.Repeat("a", 50))
	raw.Get(mediumURIRequest.Path, http.Respond)
	unknownURIRequest := getRequest(method.GET, "/"+strings.Repeat("b", 65534))
	unknownMethodRequest := getRequest(method.POST, longURIRequest.Path)

	emptyCtx := context.Background()

	r := raw.Build()

	b.Run("GET root", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r.OnRequest(GETRootRequest)
			GETRootRequest.Ctx = emptyCtx
		}
	})

	b.Run("GET long uri", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r.OnRequest(longURIRequest)
			longURIRequest.Ctx = emptyCtx
		}
	})

	b.Run("GET medium uri", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r.OnRequest(mediumURIRequest)
			mediumURIRequest.Ctx = emptyCtx
		}
	})

	b.Run("unknown uri", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r.OnRequest(unknownURIRequest)
			unknownURIRequest.Ctx = emptyCtx
		}
	})

	b.Run("unknown method", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r.OnRequest(unknownMethodRequest)
			unknownMethodRequest.Ctx = emptyCtx
		}
	})
}
