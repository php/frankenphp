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
