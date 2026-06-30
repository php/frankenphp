package frankenphp_test

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// requireFileEventually asserts that `path` appears on disk before the
// deadline. Wraps require.Eventually so call sites stay short.
func requireFileEventually(t testing.TB, path string, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 25*time.Millisecond, msgAndArgs...)
}

// setupFrankenPHP boots FrankenPHP with the given options, registers
// Shutdown as a t.Cleanup, and returns the absolute path to the testdata
// directory. Saves the boilerplate every bg-worker test repeats.
func setupFrankenPHP(t *testing.T, opts ...frankenphp.Option) (testDataDir string) {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	testDataDir = cwd + "/testdata/"
	require.NoError(t, frankenphp.Init(opts...))
	t.Cleanup(frankenphp.Shutdown)
	return
}

// serveBody runs `script` (relative to testDataDir, may include a query
// string) through FrankenPHP and returns the response body. ErrRejected is
// treated as a non-fatal outcome so worker-mode quirks don't fail tests
// that only care about the script's stdout.
func serveBody(t *testing.T, testDataDir, scriptAndQuery string, opts ...frankenphp.RequestOption) string {
	t.Helper()
	req := httptest.NewRequest("GET", "http://example.com/"+scriptAndQuery, nil)
	reqOpts := append([]frankenphp.RequestOption{
		frankenphp.WithRequestDocumentRoot(testDataDir, false),
	}, opts...)
	fr, err := frankenphp.NewRequestWithContext(req, reqOpts...)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	if err := frankenphp.ServeHTTP(w, fr); err != nil {
		require.ErrorAs(t, err, &frankenphp.ErrRejected{})
	}
	body, err := io.ReadAll(w.Result().Body)
	require.NoError(t, err)
	return string(body)
}
