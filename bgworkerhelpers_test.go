package frankenphp_test

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// serveInlinePHP writes a small PHP fixture under testDataDir, serves it
// via ServeHTTP, removes it on cleanup, and returns the response body.
// The file name should not exist already; it is registered for
// t.Cleanup-driven removal.
func serveInlinePHP(t *testing.T, testDataDir, name, php string) string {
	t.Helper()
	path := testDataDir + name
	require.NoError(t, os.WriteFile(path, []byte(php), 0644))
	t.Cleanup(func() { _ = os.Remove(path) })
	return serveBody(t, testDataDir, name)
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
