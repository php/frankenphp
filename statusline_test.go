package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

func serveStatusScript(t *testing.T, script string) *http.Response {
	t.Helper()

	cwd, _ := os.Getwd()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/"+script, nil)
	fr, err := frankenphp.NewRequestWithContext(req,
		frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	require.NoError(t, frankenphp.ServeHTTP(w, fr))

	return w.Result()
}

// A status line shorter than 9 bytes (e.g. header("HTTP/")) once made
// frankenphp_send_headers read out of bounds via atoi(http_status_line + 9).
// Run under -asan to catch a regression.
func TestShortStatusLineDoesNotOverflow(t *testing.T) {
	require.NoError(t, frankenphp.Init())
	defer frankenphp.Shutdown()

	resp := serveStatusScript(t, "short-status-line.php")
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCustomStatusLineIsHonored(t *testing.T) {
	require.NoError(t, frankenphp.Init())
	defer frankenphp.Shutdown()

	resp := serveStatusScript(t, "custom-status-line.php")
	require.Equal(t, http.StatusTeapot, resp.StatusCode)
}
