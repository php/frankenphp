package frankenphp_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

// TestModuleMaxRequests verifies that regular (non-worker) PHP threads restart
// after reaching max_requests by checking debug logs for restart messages.
func TestModuleMaxRequests(t *testing.T) {
	const maxRequests = 5
	const totalRequests = 30

	var buf syncBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for i := 0; i < totalRequests; i++ {
			body, resp := testGet("http://example.com/index.php", handler, t)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Contains(t, body, "I am by birth a Genevese")
		}

		restartCount := strings.Count(buf.String(), "max requests reached, restarting thread")
		t.Logf("Thread restarts observed: %d", restartCount)
		assert.GreaterOrEqual(t, restartCount, 2,
			"with maxRequests=%d and %d requests on 2 threads, at least 2 restarts should occur", maxRequests, totalRequests)
	}, &testOptions{
		logger: logger,
		initOpts: []frankenphp.Option{
			frankenphp.WithNumThreads(2),
			frankenphp.WithMaxRequests(maxRequests),
		},
	})
}

// TestModuleMaxRequestsConcurrent verifies max_requests works under concurrent load
// in module mode. All requests must succeed despite threads restarting.
func TestModuleMaxRequestsConcurrent(t *testing.T) {
	const maxRequests = 10
	const totalRequests = 200

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		var wg sync.WaitGroup

		for i := 0; i < totalRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				body, resp := testGet("http://example.com/index.php", handler, t)
				assert.Equal(t, 200, resp.StatusCode)
				assert.Contains(t, body, "I am by birth a Genevese")
			}()
		}
		wg.Wait()
	}, &testOptions{
		initOpts: []frankenphp.Option{
			frankenphp.WithNumThreads(8),
			frankenphp.WithMaxRequests(maxRequests),
		},
	})
}
