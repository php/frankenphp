package frankenphp

import (
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zaptest"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"123", ":id"},
		{"/*123/456/asd", "/*123/:id/asd"},
		{"/users/", "/users"},
		{"/product/550e8400-e29b-41d4-a716-446655440000/", "/product/:uuid"},
		{"/not/a/uuid/550e8400-e29b-41d4-a716-44665544000Z/", "/not/a/uuid/550e8400-e29b-41d4-a716-44665544000Z"},
		{"/page/asdfghjk-lkjhgfdsasdfghjkjhgf-dsasdfghjkjhgfdsasdf-ghjkjhgfdsasdfghjkjhgfdsasdfghjkjhgf", "/page/:slug"},
	}

	for _, test := range tests {
		normalizedPath := normalizePath(test.input)
		assert.Equal(t, test.expected, normalizedPath, "normalizePath(%q) = %q; want %q", test.input, normalizedPath, test.expected)
	}
}

func assertGetRequest(t *testing.T, url string, expectedBodyContains string, opts ...RequestOption) {
	t.Helper()
	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	req, err := NewRequestWithContext(r, opts...)
	assert.NoError(t, err)
	assert.NoError(t, ServeHTTP(w, req))
	assert.Contains(t, w.Body.String(), expectedBodyContains)
}

func TestTunnelLowLatencyRequest_worker(t *testing.T) {
	assert.NoError(t, Init(
		WithWorkers("worker", "testdata/sleep.php", 1),
		WithNumThreads(2),
		WithMaxThreads(3),
		WithLogger(slog.New(zapslog.NewHandler(zaptest.NewLogger(t).Core()))),
	))
	defer Shutdown()
	opt := WithWorkerName("worker")
	wg := sync.WaitGroup{}

	// record request path as slow, manipulate thresholds to make it easy to trigger
	slowRequestThreshold = 1 * time.Millisecond
	slowThreadPercentile = 0
	scaleWorkerThread(getWorkerByName("worker")) // the scaled thread should be low-latency only
	assertGetRequest(t, "/slow/123/path?sleep=5", "slept for 5 ms", opt)
	assertGetRequest(t, "/slow/123/path?sleep=5", "slept for 5 ms", opt)

	// send 2 blocking requests that occupy all threads
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			assertGetRequest(t, "/slow/123/path?sleep=500", "slept for 500 ms", opt)
			wg.Done()
		}()
	}
	time.Sleep(time.Millisecond * 100) // enough time to receive the requests

	// send a low latency request, it should not be blocked by the slow requests
	start := time.Now()
	assertGetRequest(t, "/fast/123/path?sleep=0", "slept for 0 ms", opt)
	duration := time.Since(start)

	assert.Less(t, duration.Milliseconds(), int64(100), "the low latency request should not be blocked by the slow requests")

	// wait to avoid race conditions across tests
	wg.Wait()
}

func TestTunnelLowLatencyRequest_module(t *testing.T) {
	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2),
		WithLogger(slog.New(zapslog.NewHandler(zaptest.NewLogger(t).Core()))),
	))
	defer Shutdown()
	wg := sync.WaitGroup{}

	// record request path as slow, manipulate thresholds to make it easy to trigger
	slowRequestThreshold = 1 * time.Millisecond
	slowThreadPercentile = 0
	scaleRegularThread() // the scaled thread should be low-latency only
	assertGetRequest(t, "/testdata/sleep.php?sleep=5", "slept for 5 ms")
	assertGetRequest(t, "/testdata/sleep.php?sleep=5", "slept for 5 ms")

	// send 2 blocking requests that occupy all threads
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			assertGetRequest(t, "/testdata/sleep.php?sleep=500", "slept for 500 ms")
			wg.Done()
		}()
	}
	time.Sleep(time.Millisecond * 100) // enough time to receive the requests

	// send a low latency request, it should not be blocked by the slow requests
	start := time.Now()
	assertGetRequest(t, "/testdata/sleep.php/fastpath/?sleep=0", "slept for 0 ms")
	duration := time.Since(start)

	assert.Less(t, duration.Milliseconds(), int64(100), "the low latency request should not be blocked by the slow requests")

	// wait to avoid race conditions across tests
	wg.Wait()
}
