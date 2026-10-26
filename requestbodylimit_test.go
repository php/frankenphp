package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyTooLarge(t *testing.T) {
	require.NoError(t, frankenphp.Init())
	defer frankenphp.Shutdown()

	cwd, _ := os.Getwd()
	handler := func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(cwd+"/testdata/", false))
		require.NoError(t, err)
		require.NoError(t, frankenphp.ServeHTTP(w, req))
	}

	for name, size := range map[string]int{"under": 512, "over": 4096} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://example.com/read-input.php", strings.NewReader(strings.Repeat("x", size)))
			w := httptest.NewRecorder()
			handler(w, req)

			if size > 1024 {
				require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
				require.NotContains(t, w.Body.String(), "read=")
			} else {
				require.Equal(t, http.StatusOK, w.Code)
				require.Contains(t, w.Body.String(), "read=512")
			}
		})
	}
}

func TestWorkerRequestBodyTooLarge(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		// the worker must stay usable after each rejected request
		for _, size := range []int{4096, 512, 4096, 512} {
			req := httptest.NewRequest("POST", "http://example.com/worker-read-input.php", strings.NewReader(strings.Repeat("x", size)))
			req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 1024)
			w := httptest.NewRecorder()
			handler(w, req)

			if size > 1024 {
				assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
				assert.NotContains(t, w.Body.String(), "read=")
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "read=512", w.Body.String())
			}
		}
	}, &testOptions{workerScript: "worker-read-input.php", nbWorkers: 1, nbParallelRequests: 1})
}
