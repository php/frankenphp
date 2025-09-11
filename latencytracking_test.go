package frankenphp

import (
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zaptest"
)

func TestIsWildcardPathPart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"*123", false},
		{"users", false},
		{"550e8400-e29b-41d4-a716-446655440000", true}, // uuid
		{"550e8400-e29b-41d4-a716-44665544000Z", false},
		// very long string
		{"asdfghjklkjhgfdsasdfghjkjhgfdsasdfghjkjhgfdsasdfghjkjhgfdsasdfghjkjhgfdsasdfghjkjhgf", true},
	}

	for _, test := range tests {
		isWildcard := isWildcardPathPart(test.input)
		assert.Equal(t, test.expected, isWildcard, "isWildcard(%q) = %q; want %q", test.input, isWildcard, test.expected)
	}
}

func assertGetRequest(t *testing.T, url string, expectedBodyContains string) {
	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	req, err := NewRequestWithContext(r, WithWorkerName("worker"))
	assert.NoError(t, err)
	assert.NoError(t, ServeHTTP(w, req))
	assert.Contains(t, w.Body.String(), expectedBodyContains)
}

func TestTunnelLowLatencyRequest(t *testing.T) {
	assert.NoError(t, Init(
		WithWorkers("worker", "testdata/sleep.php", 1),
		WithNumThreads(2),
		WithMaxThreads(3),
		WithLogger(slog.New(zapslog.NewHandler(zaptest.NewLogger(t).Core()))),
	))
	defer Shutdown()

	// record request path as slow, manipulate thresholds to make it easy to trigger
	slowRequestThreshold = 1 * time.Millisecond
	slowThreadPercentile = 0
	scaleWorkerThread(getWorkerByName("worker")) // the scaled thread should be low-latency only
	assertGetRequest(t, "/slow/123/path?sleep=5", "slept for 5 ms")
	assertGetRequest(t, "/slow/123/path?sleep=5", "slept for 5 ms")

	// send 2 blocking requests that occupy all threads
	go assertGetRequest(t, "/slow/123/path?sleep=500", "slept for 500 ms")
	go assertGetRequest(t, "/slow/123/path?sleep=500", "slept for 500 ms")
	time.Sleep(time.Millisecond * 100) // enough time to receive the requests

	// send a low latency request, it should not be blocked by the slow requests
	start := time.Now()
	assertGetRequest(t, "/fast/123/path?sleep=0", "slept for 0 ms")
	duration := time.Since(start)

	assert.Less(t, duration.Milliseconds(), int64(100), "the low latency request should not be blocked by the slow requests")
}
