package phpheaders

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllCommonHeadersAreCorrect(t *testing.T) {
	fakeRequest := httptest.NewRequest("GET", "http://localhost", nil)

	for header, phpHeader := range CommonRequestHeaders {
		// verify that common and uncommon headers return the same result
		expectedPHPHeader := GetUnCommonHeader(t.Context(), header)
		assert.Equal(t, phpHeader+"\x00", expectedPHPHeader, "header is not well formed: "+phpHeader)

		// net/http will capitalize lowercase headers, verify that headers are capitalized
		fakeRequest.Header.Add(header, "foo")
		assert.Contains(t, fakeRequest.Header, header, "header is not correctly capitalized: "+header)
	}
}

// Build a list of uncommon headers for benchmarking
var uncommonHeaders = func() []string {
	headers := make([]string, 250)
	for i := range headers {
		headers[i] = fmt.Sprintf("X-Custom-Header-%d", i+1)
	}
	return headers
}()

// BenchmarkHeaderCached validates the claim that caching is ~2.5x faster
func BenchmarkHeaderCached(b *testing.B) {
	ctx := context.Background()

	// Warm up the cache with all headers
	for _, h := range uncommonHeaders {
		GetUnCommonHeader(ctx, h)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, h := range uncommonHeaders {
			_ = GetUnCommonHeader(ctx, h)
		}
	}
}

func BenchmarkHeaderUncached(b *testing.B) {
	replacer := strings.NewReplacer(" ", "_", "-", "_")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, h := range uncommonHeaders {
			_ = "HTTP_" + replacer.Replace(strings.ToUpper(h)) + "\x00"
		}
	}
}

// BenchmarkHeaderCachedParallel simulates real FrankenPHP workload:
// multiple concurrent requests, each doing a sequential loop over headers
func BenchmarkHeaderCachedParallel(b *testing.B) {
	ctx := context.Background()

	// Warm up the cache with all headers
	for _, h := range uncommonHeaders {
		GetUnCommonHeader(ctx, h)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Each "request" sequentially processes all its headers
			for _, h := range uncommonHeaders {
				_ = GetUnCommonHeader(ctx, h)
			}
		}
	})
}
