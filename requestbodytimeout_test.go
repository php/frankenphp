package frankenphp_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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

// TestRequestBodyTimeoutHTTP2 is the HTTP/2 counterpart of the test above: over
// HTTP/2 the deadline lands on the stream (via x/net) rather than the net.Conn,
// so this exercises the SetReadDeadline path that HTTP/1 never touches. A slow
// POST that trips the idle timeout must bound the read and return cleanly.
// The nil-dereference crash of php/frankenphp#2535 needs the writer to be used
// after its stream is finalized; see TestSetReadDeadlineRecoversFromPanic.
func TestRequestBodyTimeoutHTTP2(t *testing.T) {
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	h2s := &http2.Server{}
	srv := &http.Server{Handler: h2c.NewHandler(http.HandlerFunc(handler), h2s)}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	client := &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(_ context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}}

	// A body that never sends data: the server blocks in Body.Read until the
	// idle timeout fires. Close the writer once the request returns.
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	req, err := http.NewRequest(http.MethodPost, "http://"+ln.Addr().String()+"/read-input.php", pr)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/octet-stream")

	start := time.Now()
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	elapsed := time.Since(start)

	require.Less(t, elapsed, 4*time.Second, "slow body must be bounded by the timeout")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "read=0")
}

// TestFinishRequestThenReadBodyHTTP2 reproduces php/frankenphp#2535: a script
// that calls frankenphp_finish_request() and then reads php://input triggers
// go_read_post after the HTTP/2 responseWriter has been finalized. Setting a
// read deadline on that dead writer would dereference a nil pointer and crash
// the whole process. enable_post_data_reading=Off defers the body read until
// the explicit php://input access, i.e. after the request is finished.
func TestFinishRequestThenReadBodyHTTP2(t *testing.T) {
	iniDir := t.TempDir()
	require.NoError(t, os.WriteFile(iniDir+"/php.ini", []byte("enable_post_data_reading=Off\n"), 0o600))
	t.Setenv("PHPRC", iniDir+"/php.ini")

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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	h2s := &http2.Server{}
	srv := &http.Server{Handler: h2c.NewHandler(http.HandlerFunc(handler), h2s)}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	client := &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(_ context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}}

	req, err := http.NewRequest(http.MethodPost, "http://"+ln.Addr().String()+"/finish-then-read-input.php", strings.NewReader("hello world"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
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
