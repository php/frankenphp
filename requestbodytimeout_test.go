package frankenphp_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// TestRequestBodyTimeout proves that WithRequestBodyTimeout bounds a slow-POST
// client: it announces a large Content-Length, then stalls without sending the
// body. Without the option the PHP thread would block in Body.Read until the
// connection closed; with it, the read is cut off and the request completes.
func TestRequestBodyTimeout(t *testing.T) {
	require.NoError(t, frankenphp.Init())
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r,
			frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false),
			frankenphp.WithRequestBodyTimeout(300*time.Millisecond),
		)
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(w, req))
	}

	ts := newRawServer(t, handler)
	defer ts.Close()

	conn, err := net.Dial("tcp", ts.Addr())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Announce a 1 MiB body, then send nothing: a classic slow POST.
	_, err = fmt.Fprintf(conn,
		"POST /read-input.php HTTP/1.1\r\nHost: %s\r\nContent-Type: application/octet-stream\r\nContent-Length: 1048576\r\nConnection: close\r\n\r\n",
		ts.Addr(),
	)
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	start := time.Now()
	resp, err := io.ReadAll(conn)
	elapsed := time.Since(start)
	require.NoError(t, err)

	// The 300ms idle timeout (applied at most twice by PHP's read loop) must
	// release the thread well before the client's 5s read deadline.
	require.Less(t, elapsed, 4*time.Second, "slow body must be bounded by the timeout")
	require.Contains(t, string(resp), "200 OK")
	// PHP saw an empty body: php://input read zero bytes.
	require.Contains(t, string(resp), "read=0")
}

// rawServer is a minimal HTTP server exposing its listener address so a test
// can drive it with a raw TCP connection (needed to simulate a stalled body).
type rawServer struct {
	ln  net.Listener
	srv *http.Server
}

func newRawServer(t *testing.T, handler http.HandlerFunc) *rawServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &http.Server{Handler: handler}
	go func() {
		_ = srv.Serve(ln)
	}()

	return &rawServer{ln: ln, srv: srv}
}

func (s *rawServer) Addr() string { return s.ln.Addr().String() }

func (s *rawServer) Close() {
	_ = s.srv.Close()
	_ = s.ln.Close()
}
