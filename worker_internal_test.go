package frankenphp

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRestartWorkersForceKillsStuckThread verifies the drain path does
// not hang when a worker is stuck in a blocking PHP call (sleep, etc.).
// macOS has no realtime signals so we can't unblock sleep() there; skip.
func TestRestartWorkersForceKillsStuckThread(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "windows" {
		t.Skipf("force-kill cannot interrupt blocking syscalls on %s", runtime.GOOS)
	}

	prev := drainGracePeriod
	drainGracePeriod = 500 * time.Millisecond
	t.Cleanup(func() { drainGracePeriod = prev })

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	require.NoError(t, Init(
		WithWorkers("sleep-worker", testDataDir+"worker-sleep.php", 1),
		WithNumThreads(2),
	))
	t.Cleanup(Shutdown)

	// Marker file the worker touches right before sleep(); per-run path
	// so a stale file from a prior test can't fool the poll below.
	markerFile := filepath.Join(t.TempDir(), "sleep-worker-in-sleep")

	// Worker handles the request, then sleep(60). Recorder lets us
	// assert post-sleep code never runs (would indicate the VM interrupt
	// didn't fire and only drainChan got picked up).
	recorder := httptest.NewRecorder()
	served := make(chan struct{})
	go func() {
		defer close(served)
		req := httptest.NewRequest("GET", "http://example.com/worker-sleep.php", nil)
		req.Header.Set("Sleep-Marker", markerFile)
		fr, err := NewRequestWithContext(req, WithRequestDocumentRoot(testDataDir, false))
		if err != nil {
			return
		}
		_ = ServeHTTP(recorder, fr)
	}()

	// Confirm the worker is parked in sleep() before triggering the
	// restart, so we exercise the force-kill path and not drainChan.
	require.Eventually(t, func() bool {
		_, err := os.Stat(markerFile)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "worker never entered sleep()")

	start := time.Now()
	RestartWorkers()
	elapsed := time.Since(start)

	// Test grace period (500ms) + 3s slack for signal dispatch, VM tick, restart loop.
	const budget = 4 * time.Second
	assert.Less(t, elapsed, budget, "drain must force-kill the stuck thread within the grace period")

	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("server request goroutine did not complete after drain")
	}
	assert.NotContains(t, recorder.Body.String(), "should not reach",
		"VM interrupt was never observed; sleep returned naturally")
}
