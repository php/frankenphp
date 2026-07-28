package frankenphp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// blockingResponseWriter simulates a client whose connection is stalled: any
// Write() call after headers are sent blocks forever. This reproduces a PHP
// thread stuck inside go_ub_write with no way for FrankenPHP's force-kill
// (which only interrupts the Zend VM at the next opcode boundary, see
// frankenphp_force_kill_thread) to unstick it, since the thread has already
// left PHP execution and is parked in a Go-level network write.
type blockingResponseWriter struct {
	header  http.Header
	started chan struct{}
	once    sync.Once
}

func newBlockingResponseWriter() *blockingResponseWriter {
	return &blockingResponseWriter{header: make(http.Header), started: make(chan struct{})}
}

func (w *blockingResponseWriter) Header() http.Header { return w.header }

func (w *blockingResponseWriter) WriteHeader(int) {}

func (w *blockingResponseWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	select {} // block forever, like a write to a dead/stalled TCP connection
}

// TestRebootDoesNotHangOnUnkillableThread reproduces the hang described on
// https://github.com/php/frankenphp/pull/2570#issuecomment-5099392587 and
// https://github.com/php/frankenphp/issues/2553: a thread genuinely stuck in
// a blocking write (e.g. to a stalled HTTP/2 client) can't be interrupted by
// force-kill, so rebootAllThreads()'s unbounded post-force-kill wait blocks
// forever, wedging the whole pool (scalingMu never released, isRebooting
// never cleared) rather than just losing the one stuck thread.
func TestRebootDoesNotHangOnUnkillableThread(t *testing.T) {
	require.NoError(t, frankenphp.Init(frankenphp.WithNumThreads(2), frankenphp.WithMaxThreads(2)))
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()

	w := newBlockingResponseWriter()
	go func() {
		req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
		fr, err := frankenphp.NewRequestWithContext(req, frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false))
		if err != nil {
			t.Errorf("NewRequestWithContext: %v", err)
			return
		}
		// echo.php-style output: any script that writes output triggers go_ub_write
		_ = frankenphp.ServeHTTP(w, fr)
	}()

	select {
	case <-w.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the stuck request never started writing")
	}

	// give go_ub_write a moment to actually be blocked in Write()
	time.Sleep(100 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		frankenphp.RestartWorkers()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("RestartWorkers() hung forever on a thread stuck in a blocking write (force-kill can't interrupt it)")
	}

	// the pool must still be usable after giving up on the stuck thread: a
	// fresh request on a *new* writer must complete normally.
	body, _ := testGet(fmt.Sprintf("http://example.com/index.php?i=%d", 1), func(rw http.ResponseWriter, r *http.Request) {
		fr, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false))
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(rw, fr))
	}, t)
	require.Contains(t, body, "I am by birth a Genevese")
}
