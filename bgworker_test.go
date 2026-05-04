package frankenphp_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

// TestBackgroundWorker drives the minimal set_vars/get_vars path end-to-end:
// a background worker publishes three values, then an HTTP request on a
// separate thread reads them back by name.
func TestBackgroundWorker(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("bg-basic", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(2),
	)

	// Give the background worker time to boot, publish, and park on the
	// stop stream. set_vars is synchronous but the first scheduling of the
	// bg worker thread is a race with Init returning, so a short wait is
	// cheaper than a ready-channel hook for this test.
	deadline := time.Now().Add(3 * time.Second)
	var out string
	for time.Now().Before(deadline) {
		out = serveBody(t, testDataDir, "bgworker/reader.php")
		if !strings.Contains(out, "MISSING") && strings.Contains(out, "has-ready-at=1") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.Contains(t, out, "message=hello from background worker")
	assert.Contains(t, out, "count=42")
	assert.Contains(t, out, "has-ready-at=1")
}

// TestBackgroundWorkerErrorPaths covers the misuse errors that don't need
// a running worker: get_vars on a nonexistent name, set_vars from outside
// a background worker, and get_worker_handle from outside a background
// worker. Runs as a non-worker request so none of the calls happen on a
// bg-worker thread.
func TestBackgroundWorkerErrorPaths(t *testing.T) {
	testDataDir := setupFrankenPHP(t, frankenphp.WithNumThreads(2))

	body := serveBody(t, testDataDir, "bgworker/errors.php")
	assert.NotContains(t, body, "FAIL", "error-path script reported a failure:\n"+body)
	assert.Contains(t, body, "OK missing:")
	assert.Contains(t, body, "OK reject-non-bg:")
	assert.Contains(t, body, "OK reject-handle:")
}
