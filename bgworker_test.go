package frankenphp_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackgroundWorkerLifecycle boots a background worker that touches a
// sentinel file then parks on the stop pipe. It proves the bg worker runs
// (sentinel appears) and that Shutdown returns within a reasonable time.
func TestBackgroundWorkerLifecycle(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-lifecycle.sentinel")

	require.NoError(t, frankenphp.Init(
		frankenphp.WithWorkers("bg-lifecycle", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	))
	// Note: this test asserts on Shutdown timing, so it manages Shutdown
	// itself instead of using setupFrankenPHP's t.Cleanup hook.

	requireFileEventually(t, sentinel, "background worker did not touch sentinel")

	done := make(chan struct{})
	go func() {
		frankenphp.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("Shutdown did not return within 10s")
	}
}

// TestBackgroundWorkerCrashRestarts boots a worker that exit(1)s on its
// first run and touches a "restarted" sentinel on its second run. The
// sentinel proves the crash-restart loop fired.
func TestBackgroundWorkerCrashRestarts(t *testing.T) {
	tmp := t.TempDir()
	crashMarker := filepath.Join(tmp, "bg-crash.marker")
	restarted := filepath.Join(tmp, "bg-crash.restarted")

	setupFrankenPHP(t,
		frankenphp.WithWorkers("bg-crash", "testdata/bgworker/crash.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{
				"BG_CRASH_MARKER":       crashMarker,
				"BG_RESTARTED_SENTINEL": restarted,
			}),
		),
		frankenphp.WithNumThreads(2),
	)

	requireFileEventually(t, restarted, "background worker did not restart after crash")
}

// TestBackgroundWorkerWithoutHTTP confirms that a request to a script
// unrelated to the bg worker still works: the bg worker doesn't intercept
// HTTP traffic.
func TestBackgroundWorkerWithoutHTTP(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "bg-nohttp.sentinel")

	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("bg-nohttp", "testdata/bgworker/basic.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerEnv(map[string]string{"BG_SENTINEL": sentinel}),
		),
		frankenphp.WithNumThreads(2),
	)

	requireFileEventually(t, sentinel, "background worker did not touch sentinel")

	body := serveBody(t, testDataDir, "index.php")
	assert.NotEmpty(t, body, "expected non-empty body from index.php")
}
