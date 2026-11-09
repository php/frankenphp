package frankenphp

import (
	"strings"
	"testing"
)

var responseHeaderKeyResult, responseHeaderValueResult string

func BenchmarkSplitRawHeader(b *testing.B) {
	for _, raw := range []string{"Content-Type: application/json", "X-Request-Id: abcdef0123456789", "Access-Control-Allow-Credentials: true", strings.Repeat("X", 128) + ": value", "missing separator"} {
		b.Run(raw, func(b *testing.B) {
			ptr := toUnsafeChar(raw)
			b.ReportAllocs()
			for b.Loop() {
				responseHeaderKeyResult, responseHeaderValueResult = splitRawHeader(ptr, len(raw))
			}
		})
	}
}
