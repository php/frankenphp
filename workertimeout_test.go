package frankenphp

import (
	"bytes"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const timeoutTestScript = "worker-timeout-sleep.php"

// requiresKillSignal skips tests that rely on interrupting a blocking syscall
// (sleep). Only Linux/FreeBSD ship the realtime kill signal used by the
// force-kill primitive; elsewhere the watchdog can only set the VM-interrupt
// flag, which never unblocks sleep().
func requiresKillSignal(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		t.Skipf("worker timeout cannot interrupt blocking syscalls on %s", runtime.GOOS)
	}
}

// lockedBuffer is a goroutine-safe io.Writer for capturing logs emitted from
// the watchdog goroutine.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func initTimeoutWorker(t *testing.T, timeout time.Duration, numThreads, numWorkers int, logger *slog.Logger) string {
	t.Helper()

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	opts := []Option{
		WithNumThreads(numThreads),
		WithWorkers("timeout-worker", testDataDir+timeoutTestScript, numWorkers, WithWorkerTimeout(timeout)),
	}
	if logger != nil {
		opts = append(opts, WithLogger(logger))
	}

	require.NoError(t, Init(opts...))
	t.Cleanup(Shutdown)

	return testDataDir
}

// serveTimeoutRequest issues one request to the timeout worker. It is safe to
// call from a non-test goroutine (it never calls t.FailNow via require).
func serveTimeoutRequest(testDataDir, marker string, sleepSeconds int) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest("GET", "http://example.com/"+timeoutTestScript, nil)
	if marker != "" {
		req.Header.Set("Sleep-Marker", marker)
	}
	req.Header.Set("Sleep-Seconds", strconv.Itoa(sleepSeconds))

	fr, err := NewRequestWithContext(req, WithRequestDocumentRoot(testDataDir, false))
	if err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()

	return rec, ServeHTTP(rec, fr)
}

func newTimeoutTestLogger() (*slog.Logger, *lockedBuffer) {
	buf := &lockedBuffer{}

	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})), buf
}

// TestWorkerTimeout_InterruptsSlowRequest spins up a worker with a 1s timeout,
// sends a request whose handler sleeps far longer, and asserts the request is
// interrupted well within budget and that the worker recovers afterwards.
func TestWorkerTimeout_InterruptsSlowRequest(t *testing.T) {
	requiresKillSignal(t)

	logger, buf := newTimeoutTestLogger()
	testDataDir := initTimeoutWorker(t, time.Second, 2, 1, logger)

	// Per-run marker the worker touches right before sleep(), so we only start
	// the budget once the worker is provably parked in sleep().
	marker := filepath.Join(t.TempDir(), "in-sleep")

	var rec *httptest.ResponseRecorder
	done := make(chan struct{})
	go func() {
		defer close(done)
		rec, _ = serveTimeoutRequest(testDataDir, marker, 60)
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "worker never entered sleep()")

	// Timeout (1s) + slack for signal dispatch, VM tick and the restart loop.
	const budget = 5 * time.Second
	select {
	case <-done:
	case <-time.After(budget):
		t.Fatal("request was not interrupted within the worker timeout budget")
	}

	assert.NotContains(t, rec.Body.String(), "completed",
		"interrupted request must not have produced the post-sleep output")
	assert.Contains(t, buf.String(), "worker request timeout",
		"watchdog should have logged the interruption")

	// The worker must restart and serve the next request normally.
	require.Eventually(t, func() bool {
		rec2, _ := serveTimeoutRequest(testDataDir, "", 0)
		return rec2.Code == 200 && strings.Contains(rec2.Body.String(), "completed")
	}, 5*time.Second, 50*time.Millisecond, "worker did not recover after timeout")
}

// TestWorkerTimeout_InterruptsBlockingSocketRead verifies the watchdog aborts a
// request blocked in a socket read - the case that the VM-interrupt flag alone
// cannot handle and that the fd shutdown exists for. The worker connects to a
// listener that accepts but never replies, so fread() parks in ppoll exactly
// like a slow DB query.
func TestWorkerTimeout_InterruptsBlockingSocketRead(t *testing.T) {
	requiresKillSignal(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	var (
		mu    sync.Mutex
		conns []net.Conn
	)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c) // hold open, never write
			mu.Unlock()
		}
	}()
	defer func() {
		mu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()
	}()

	logger, buf := newTimeoutTestLogger()
	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"
	require.NoError(t, Init(
		WithNumThreads(2),
		WithLogger(logger),
		WithWorkers("sock-worker", testDataDir+"worker-blocking-read.php", 1, WithWorkerTimeout(time.Second)),
	))
	t.Cleanup(Shutdown)

	req := httptest.NewRequest("GET", "http://example.com/worker-blocking-read.php", nil)
	req.Header.Set("Upstream-Addr", ln.Addr().String())
	fr, err := NewRequestWithContext(req, WithRequestDocumentRoot(testDataDir, false))
	require.NoError(t, err)
	rec := httptest.NewRecorder()

	start := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = ServeHTTP(rec, fr)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("blocking socket read was not aborted by the worker timeout")
	}

	assert.Less(t, time.Since(start), 6*time.Second)
	assert.Contains(t, buf.String(), "worker request timeout")
	assert.NotContains(t, rec.Body.String(), "read returned",
		"fread should have been interrupted, not returned")
}

// TestWorkerTimeout_DoesNotFireOnFastRequest verifies the watchdog is cancelled
// for a request that finishes before the timeout and never interrupts the thread.
func TestWorkerTimeout_DoesNotFireOnFastRequest(t *testing.T) {
	logger, buf := newTimeoutTestLogger()
	testDataDir := initTimeoutWorker(t, 5*time.Second, 2, 1, logger)

	rec, err := serveTimeoutRequest(testDataDir, "", 0)
	require.NoError(t, err)
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "completed")

	// A cancelled watchdog must never fire, even given a moment to misbehave.
	time.Sleep(200 * time.Millisecond)
	assert.NotContains(t, buf.String(), "worker request timeout")
}

// TestWorkerTimeout_Disabled verifies that WorkerTimeout = 0 disables the
// watchdog: a slow request runs to natural completion, uninterrupted.
func TestWorkerTimeout_Disabled(t *testing.T) {
	logger, buf := newTimeoutTestLogger()
	testDataDir := initTimeoutWorker(t, 0, 2, 1, logger)

	start := time.Now()
	rec, err := serveTimeoutRequest(testDataDir, "", 1)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, time.Since(start), time.Second, "request must have slept its full duration")
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "completed")
	assert.NotContains(t, buf.String(), "worker request timeout")
}

// TestWorkerTimeout_PoolDoesNotCrossSignals fires one stuck request per thread
// in a pool and asserts every request is interrupted within budget. If a signal
// were delivered to the wrong thread, at least one request would hang past the
// budget (and another thread would be killed twice), so the all-interrupted
// outcome proves each thread received its own interrupt.
func TestWorkerTimeout_PoolDoesNotCrossSignals(t *testing.T) {
	requiresKillSignal(t)

	const pool = 5
	testDataDir := initTimeoutWorker(t, time.Second, pool+1, pool, nil)

	var wg sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, pool)
	for i := 0; i < pool; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = serveTimeoutRequest(testDataDir, "", 60)
		}(i)
	}

	completed := make(chan struct{})
	go func() {
		wg.Wait()
		close(completed)
	}()

	// Timeout (1s) + generous slack for 5 concurrent restarts.
	const budget = 8 * time.Second
	select {
	case <-completed:
	case <-time.After(budget):
		t.Fatal("not all stuck requests were interrupted; a signal may have hit the wrong thread")
	}

	for i := 0; i < pool; i++ {
		assert.NotContains(t, results[i].Body.String(), "completed",
			"request %d completed instead of being interrupted", i)
	}

	// The whole pool must recover and serve fast follow-up requests.
	require.Eventually(t, func() bool {
		rec, _ := serveTimeoutRequest(testDataDir, "", 0)
		return rec.Code == 200 && strings.Contains(rec.Body.String(), "completed")
	}, 5*time.Second, 50*time.Millisecond, "pool did not recover after concurrent timeouts")
}
