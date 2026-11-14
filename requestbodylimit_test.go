package frankenphp_test

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// limitBody caps request bodies like Caddy's request_body max_size does.
func limitBody(limit int64, handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		handler(w, r)
	}
}

func newBodyRequest(t *testing.T, method, target, contentType string, size int, chunked bool) *http.Request {
	t.Helper()

	var body bytes.Buffer
	switch contentType {
	case "multipart/form-data":
		mw := multipart.NewWriter(&body)
		fw, err := mw.CreateFormFile("file", "file.txt")
		require.NoError(t, err)
		_, err = fw.Write(bytes.Repeat([]byte("x"), size))
		require.NoError(t, err)
		require.NoError(t, mw.Close())
		contentType = mw.FormDataContentType()
	case "application/x-www-form-urlencoded":
		body.WriteString("foo=" + strings.Repeat("x", size-len("foo=")))
	default:
		body.WriteString(strings.Repeat("x", size))
	}

	var r io.Reader = &body
	if chunked {
		// hides the length
		r = io.MultiReader(r)
	}

	req := httptest.NewRequest(method, target, r)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req
}

// waitForThread returns once the only PHP thread is done with the previous script: a 413 is sent before.
func waitForThread(t *testing.T, handler func(http.ResponseWriter, *http.Request)) {
	t.Helper()

	testGet("http://example.com/read-input.php", handler, t)
}

// The script must not run on a truncated body, whoever reads it.
func TestRequestBodyTooLarge(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		handler = limitBody(1024, handler)

		for _, tc := range []struct {
			method, contentType string
			// PHP reads the body before the script starts
			readAtStartup bool
		}{
			{"POST", "application/x-www-form-urlencoded", true},
			{"POST", "multipart/form-data", true},
			{"POST", "text/csv", true},
			{"POST", "", false},
			{"PUT", "application/json", false},
			{"PUT", "application/x-www-form-urlencoded", false},
		} {
			for _, size := range []int{512, 4096} {
				for _, chunked := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s %q %d chunked=%t", tc.method, tc.contentType, size, chunked), func(t *testing.T) {
						marker := filepath.Join(t.TempDir(), "marker")
						w := httptest.NewRecorder()
						handler(w, newBodyRequest(t, tc.method, "http://example.com/read-input-then-touch.php?marker="+marker, tc.contentType, size, chunked))

						if size <= 1024 {
							assert.Equal(t, http.StatusOK, w.Code)
							assert.FileExists(t, marker)

							return
						}

						assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
						waitForThread(t, handler)
						assert.NoFileExists(t, marker)
						if tc.readAtStartup {
							assert.NoFileExists(t, marker+"-caught")
						} else {
							assert.FileExists(t, marker+"-caught")
						}
					})
				}
			}
		}
	}, &testOptions{nbParallelRequests: 1, initOpts: []frankenphp.Option{frankenphp.WithNumThreads(1)}})
}

func TestWorkerRequestBodyTooLarge(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		handler = limitBody(1024, handler)

		for _, tc := range []struct {
			method, query, contentType string
			size, status               int
			expected                   string
		}{
			{"POST", "", "", 512, http.StatusOK, "count=1 finally=0 read=512"},
			// rejected before the callback runs
			{"POST", "", "application/x-www-form-urlencoded", 4096, http.StatusRequestEntityTooLarge, ""},
			// the callback throws while reading it
			{"PUT", "", "", 4096, http.StatusRequestEntityTooLarge, ""},
			// drained after the callback returns
			{"PUT", "?ignore-body", "", 4096, http.StatusOK, "count=3 finally=2 read=0"},
			// the worker wasn't restarted
			{"POST", "", "", 512, http.StatusOK, "count=4 finally=3 read=512"},
		} {
			w := httptest.NewRecorder()
			handler(w, newBodyRequest(t, tc.method, "http://example.com/worker-read-input.php"+tc.query, tc.contentType, tc.size, false))

			assert.Equal(t, tc.status, w.Code)
			if tc.expected == "" {
				assert.NotContains(t, w.Body.String(), "count=")
			} else {
				assert.Equal(t, tc.expected, w.Body.String())
			}
		}
	}, &testOptions{workerScript: "worker-read-input.php", nbWorkers: 1, nbParallelRequests: 1})
}

// The script starts before the end of the body, which it reads as it arrives.
func TestRequestBodyStreamed(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		marker := filepath.Join(t.TempDir(), "marker")
		body, bodyWriter := io.Pipe()
		req := httptest.NewRequest("PUT", "http://example.com/read-input-stream.php?marker="+marker, body)
		req.ContentLength = 16384

		w := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			defer close(done)
			limitBody(1<<20, handler)(w, req)
		}()

		chunk := strings.Repeat("x", 8192)
		_, err := io.WriteString(bodyWriter, chunk)
		require.NoError(t, err)
		assert.Eventually(t, func() bool {
			_, err := os.Stat(marker)

			return err == nil
		}, 5*time.Second, 10*time.Millisecond)

		_, err = io.WriteString(bodyWriter, chunk)
		require.NoError(t, err)
		require.NoError(t, bodyWriter.Close())
		<-done

		assert.Equal(t, "read=16384", w.Body.String())
	}, &testOptions{nbParallelRequests: 1})
}
