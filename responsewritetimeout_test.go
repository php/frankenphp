package frankenphp_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// TestResponseWriteTimeoutDoesNotTruncateSlowButSteadyTransfer proves a large
// write isn't punished merely for taking longer than the timeout to finish,
// only for actually stalling. SetWriteDeadline bounds whichever Write() call
// it wraps, not idle time within it, and with output_buffering=0 PHP hands an
// entire echo() to go_ub_write in one call: without resetting the deadline
// per chunk, a slow-but-alive client downloading a large response would be
// truncated exactly like a dead one once the whole transfer merely takes
// longer than the timeout.
func TestResponseWriteTimeoutDoesNotTruncateSlowButSteadyTransfer(t *testing.T) {
	iniDir := t.TempDir()
	require.NoError(t, os.WriteFile(iniDir+"/php.ini", []byte("output_buffering=0\n"), 0o600))
	t.Setenv("PHPRC", iniDir+"/php.ini")

	require.NoError(t, frankenphp.Init())
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r,
			frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false),
			frankenphp.WithResponseWriteTimeout(500*time.Millisecond),
		)
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(w, req))
	}

	ts := newRawServer(t, handler)
	defer ts.Close()

	conn, err := net.Dial("tcp", ts.Addr())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Shrink the client's receive window so the server's writes actually
	// backpressure on our read pace below, instead of the whole 2 MiB
	// silently fitting in default OS socket buffers with no real stall.
	require.NoError(t, conn.(*net.TCPConn).SetReadBuffer(32*1024))

	_, err = fmt.Fprintf(conn, "GET /large-output.php HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", ts.Addr())
	require.NoError(t, err)

	// Drip-feed reads: slow enough that the full 2 MiB transfer takes well
	// over the 500ms write timeout, but frequent enough that no single
	// 64 KiB write chunk (responseWriteChunkSize) ever actually stalls.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	buf := make([]byte, 16*1024)
	total := 0
	start := time.Now()
	for {
		n, err := conn.Read(buf)
		total += n
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	elapsed := time.Since(start)

	require.Greater(t, elapsed, 500*time.Millisecond, "the test must actually take longer than the write timeout to be meaningful")
	require.Greater(t, total, 2*1024*1024, "the full response must arrive uninterrupted despite taking longer than the write timeout")
}

// TestResponseWriteTimeout proves that WithResponseWriteTimeout bounds a
// stalled reader: the client never reads the response, so the server's socket
// send buffer fills up and the underlying Write() blocks. Without the option
// the PHP thread would block in ub_write forever, since
// frankenphp_force_kill_thread cannot interrupt a write parked in Go's
// non-blocking netpoller (see php/frankenphp#2553, php/frankenphp#2573); with
// it, the write is cut off and the handler returns.
func TestResponseWriteTimeout(t *testing.T) {
	require.NoError(t, frankenphp.Init())
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()
	done := make(chan struct{})
	handler := func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		req, err := frankenphp.NewRequestWithContext(r,
			frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false),
			frankenphp.WithResponseWriteTimeout(300*time.Millisecond),
		)
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(w, req))
	}

	ts := newRawServer(t, handler)
	defer ts.Close()

	conn, err := net.Dial("tcp", ts.Addr())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Shrink the client's receive window so the server's send buffer fills
	// after only a handful of writes instead of relying on OS defaults.
	require.NoError(t, conn.(*net.TCPConn).SetReadBuffer(1024))

	_, err = fmt.Fprintf(conn, "GET /slow-write.php HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", ts.Addr())
	require.NoError(t, err)

	// Never read the response: that's the stalled reader.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the write timeout did not release the PHP thread")
	}
}
