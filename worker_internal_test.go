package frankenphp

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrainWorkerRequests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped, resume := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	t.Cleanup(func() { release(); Shutdown() })
	require.NoError(t, Init(
		WithContext(ctx), WithWorkerRequestDrainTimeout(0),
		WithNumThreads(2), WithMaxThreads(2), WithMaxWaitTime(time.Second),
		WithWorkers("retired", testDataPath+"/worker-with-counter.php", 1,
			WithWorkerOnShutdown(func(int) { close(stopped); <-resume }),
		),
	))
	w := workersByName["retired"]
	request := httptest.NewRequest("POST", "http://localhost/worker-with-counter.php", strings.NewReader("payload"))
	response := httptest.NewRecorder()
	fc, err := newContextFromRequest(request, response, fallbackServer, WithRequestDocumentRoot(testDataPath, false))
	require.NoError(t, err)
	require.Same(t, w, fc.worker)
	shutdownDone := make(chan struct{})
	go func() { defer close(shutdownDone); Shutdown() }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("worker did not reach shutdown")
	}
	result := make(chan error, 1)
	go func() { result <- w.handleRequest(fc) }()
	require.Eventually(t, func() bool { return w.queuedRequests.Load() == 1 }, time.Second, time.Millisecond)
	release()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("PHP shutdown did not finish")
	}
	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrNotRunning)
	case <-time.After(time.Second):
		t.Fatal("retired worker request was not released")
	}
	require.Zero(t, w.queuedRequests.Load())
	require.Empty(t, response.Body.String())
	body, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	require.Equal(t, "payload", string(body))
}

func TestWorkerRequestDrainerExpires(t *testing.T) {
	w := &worker{requestChan: make(chan *frankenPHPContext)}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		w.drainRequests(context.Background(), 10*time.Millisecond)
	}()
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("worker queue drainer exceeded its configured timeout")
	}
}

// TestRestartWorkersForceKillsStuckThread verifies the drain path does
// not hang when a worker is stuck in a blocking PHP call (sleep, etc.).
// macOS has no realtime signals so we can't unblock sleep() there; skip.
func TestRestartWorkersForceKillsStuckThread(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "windows" {
		t.Skipf("force-kill cannot interrupt blocking syscalls on %s", runtime.GOOS)
	}

	prev := rebootGracePeriod
	rebootGracePeriod = 500 * time.Millisecond
	t.Cleanup(func() { rebootGracePeriod = prev })

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

// Init() must not return, nor the runtime tear SAPI/TSRM down, while a worker
// that failed to boot is still inside its shutdown handler
func TestInitJoinsAThreadStuckInStartupTeardown(t *testing.T) {
	t.Cleanup(Shutdown)

	var failedThread *phpThread
	held := make(chan struct{})
	release := make(chan struct{})
	initDone := make(chan error, 1)

	go func() {
		initDone <- Init(
			WithNumThreads(2),
			WithWorkers("held-failing-worker", testDataPath+"/failing-worker.php", 1,
				WithWorkerMaxFailures(0),
				WithWorkerOnShutdown(func(threadIndex int) {
					failedThread = phpThreads[threadIndex]
					close(held)
					<-release
				}),
			),
		)
	}()

	select {
	case <-held:
	case err := <-initDone:
		t.Fatalf("Init returned before the failed worker thread started its teardown: %v", err)
	}

	select {
	case err := <-initDone:
		t.Fatalf("Init returned while the failed worker thread was still shutting down (state: %s, error: %v)", failedThread.state.Name(), err)
	case <-time.After(500 * time.Millisecond):
	}

	require.True(t, failedThread.state.Is(state.ShuttingDown), "thread should still be shutting down")
	close(release)

	assert.Error(t, <-initDone, "a worker failing to boot must fail Init")
	assert.True(t, failedThread.state.Is(state.Reserved), "the thread must have exited and been reclaimed before Init returned, got: "+failedThread.state.Name())
}
