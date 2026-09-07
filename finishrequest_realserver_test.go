package frankenphp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// TestFinishRequestRealHTTPServerDoesNotAbort proves the frankenphp_finish_request()
// fix holds against a real net/http.Server, not just the httptest.NewRecorder()
// harness the rest of this suite uses. A recorder-driven test never exercises
// Go's automatic cancellation of request.Context() on handler return, which
// happens the instant frankenphp_finish_request() lets ServeHTTP return -
// independent of whether the client is still connected (see the discussion on
// php/frankenphp#2569). Checking clientHasClosed() again after that point,
// instead of the state captured before the handler returned, would read true
// for this reason alone and incorrectly abort the script.
func TestFinishRequestRealHTTPServerDoesNotAbort(t *testing.T) {
	logger, buf := newTestLogger(t)
	require.NoError(t, frankenphp.Init(frankenphp.WithLogger(logger)))
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fr, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false))
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(w, fr))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/finish-request.php?i=1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "This is output 1\n", string(body))

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "reached after finish_request 1")
	}, 2*time.Second, 10*time.Millisecond,
		"a write after a normal fastcgi_finish_request() must not be treated as an aborted connection")
}
