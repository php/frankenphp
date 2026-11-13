package frankenphp_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingBody returns the first bytes of a body, then err instead of io.EOF, like a body cut short.
type failingBody struct {
	r   io.Reader
	err error
}

func (b *failingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if errors.Is(err, io.EOF) {
		return n, b.err
	}

	return n, err
}

func (b *failingBody) Close() error { return nil }

// cutBody delivers only the first n bytes of request bodies, then fails with err.
func cutBody(n int64, err error, handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = &failingBody{r: io.LimitReader(r.Body, n), err: err}
		handler(w, r)
	}
}

// The script must not run on a body whose read failed, whoever reads it.
func TestRequestBodyIncomplete(t *testing.T) {
	for _, failure := range []struct {
		name   string
		err    error
		status int
	}{
		{"unexpected EOF", io.ErrUnexpectedEOF, http.StatusBadRequest},
		{"connection reset", &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}, http.StatusBadRequest},
		{"deadline", os.ErrDeadlineExceeded, http.StatusRequestTimeout},
	} {
		t.Run(failure.name, func(t *testing.T) {
			runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
				cut := cutBody(1024, failure.err, handler)

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
					for _, chunked := range []bool{false, true} {
						t.Run(fmt.Sprintf("%s %q chunked=%t", tc.method, tc.contentType, chunked), func(t *testing.T) {
							marker := filepath.Join(t.TempDir(), "marker")
							w := httptest.NewRecorder()
							cut(w, newBodyRequest(t, tc.method, "http://example.com/read-input-then-touch.php?marker="+marker, tc.contentType, 4096, chunked))

							assert.Equal(t, failure.status, w.Code)
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
			}, &testOptions{nbParallelRequests: 1, initOpts: []frankenphp.Option{frankenphp.WithNumThreads(1)}})
		})
	}
}

// A transport that ends the body before the announced Content-Length without an error (HTTP/3 with
// quic-go accepts such a stream) must not deliver the received bytes as the complete body.
func TestRequestBodyShorterThanContentLength(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		cut := cutBody(1024, io.EOF, handler)

		for _, tc := range []struct {
			method, contentType string
			readAtStartup       bool
		}{
			{"POST", "application/x-www-form-urlencoded", true},
			{"POST", "multipart/form-data", true},
			{"PUT", "application/json", false},
		} {
			t.Run(tc.method+" "+tc.contentType, func(t *testing.T) {
				marker := filepath.Join(t.TempDir(), "marker")
				req := newBodyRequest(t, tc.method, "http://example.com/read-input-then-touch.php?marker="+marker, tc.contentType, 4096, false)
				require.Positive(t, req.ContentLength)

				w := httptest.NewRecorder()
				cut(w, req)

				assert.Equal(t, http.StatusBadRequest, w.Code)
				waitForThread(t, handler)
				assert.NoFileExists(t, marker)
				if tc.readAtStartup {
					assert.NoFileExists(t, marker+"-caught")
				} else {
					assert.FileExists(t, marker+"-caught")
				}
			})
		}

		t.Run("complete body", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "marker")
			w := httptest.NewRecorder()
			handler(w, newBodyRequest(t, "PUT", "http://example.com/read-input-then-touch.php?marker="+marker, "application/json", 4096, false))

			assert.Equal(t, http.StatusOK, w.Code)
			assert.FileExists(t, marker)
		})
	}, &testOptions{nbParallelRequests: 1, initOpts: []frankenphp.Option{frankenphp.WithNumThreads(1)}})
}

// request_parse_body() reads the body like php://input: a failed read must throw, not parse a prefix.
func TestRequestParseBodyIncomplete(t *testing.T) {
	if frankenphp.Version().VersionID < 80400 {
		t.Skip("request_parse_body() requires PHP 8.4")
	}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for _, failure := range []struct {
			name   string
			err    error
			status int
		}{
			{"unexpected EOF", io.ErrUnexpectedEOF, http.StatusBadRequest},
			{"deadline", os.ErrDeadlineExceeded, http.StatusRequestTimeout},
		} {
			for _, contentType := range []string{"application/x-www-form-urlencoded", "multipart/form-data"} {
				t.Run(failure.name+" "+contentType, func(t *testing.T) {
					marker := filepath.Join(t.TempDir(), "marker")
					w := httptest.NewRecorder()
					cutBody(1024, failure.err, handler)(w, newBodyRequest(t, "PUT", "http://example.com/parse-body-then-touch.php?marker="+marker, contentType, 4096, false))

					assert.Equal(t, failure.status, w.Code)
					waitForThread(t, handler)
					assert.NoFileExists(t, marker)
					assert.FileExists(t, marker+"-caught")
				})
			}
		}
	}, &testOptions{nbParallelRequests: 1, initOpts: []frankenphp.Option{frankenphp.WithNumThreads(1)}})
}

// A multipart text field cut short is invisible to PHP: php://input is empty and the field holds a valid prefix.
func TestRequestBodyIncompleteMultipartTextField(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		require.NoError(t, mw.WriteField("name", "demo"))
		require.NoError(t, mw.WriteField("amount", "12345.33"))
		require.NoError(t, mw.Close())
		// cut inside "12345.33"
		cutAt := int64(bytes.Index(body.Bytes(), []byte("12345.33")) + 2)

		marker := filepath.Join(t.TempDir(), "marker")
		req := httptest.NewRequest("POST", "http://example.com/read-input-then-touch.php?marker="+marker, &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		cutBody(cutAt, io.ErrUnexpectedEOF, handler)(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		waitForThread(t, handler)
		assert.NoFileExists(t, marker)
	}, &testOptions{nbParallelRequests: 1, initOpts: []frankenphp.Option{frankenphp.WithNumThreads(1)}})
}

func TestWorkerRequestBodyIncomplete(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		cut := cutBody(1024, io.ErrUnexpectedEOF, handler)

		for _, tc := range []struct {
			method, contentType string
			truncated           bool
			status              int
			expected            string
		}{
			{"POST", "", false, http.StatusOK, "count=1 finally=0 read=4096"},
			// rejected before the callback runs
			{"POST", "application/x-www-form-urlencoded", true, http.StatusBadRequest, ""},
			// the callback throws while reading it
			{"PUT", "", true, http.StatusBadRequest, ""},
			// the worker wasn't restarted
			{"POST", "", false, http.StatusOK, "count=3 finally=2 read=4096"},
		} {
			w := httptest.NewRecorder()
			req := newBodyRequest(t, tc.method, "http://example.com/worker-read-input.php", tc.contentType, 4096, false)
			if tc.truncated {
				cut(w, req)
			} else {
				handler(w, req)
			}

			assert.Equal(t, tc.status, w.Code)
			if tc.expected == "" {
				assert.NotContains(t, w.Body.String(), "count=")
			} else {
				assert.Equal(t, tc.expected, w.Body.String())
			}
		}
	}, &testOptions{workerScript: "worker-read-input.php", nbWorkers: 1, nbParallelRequests: 1})
}

// testdataRoot returns the absolute path of testdata/.
func testdataRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	return cwd + "/testdata/"
}

// newTestdataServer serves testdata/ through FrankenPHP over a real listener and reports when each request is done.
// The handler runs outside the test goroutine: it must use assert, never require (t.FailNow).
func newTestdataServer(t *testing.T) (*rawServer, <-chan struct{}) {
	t.Helper()

	root := testdataRoot(t)
	done := make(chan struct{}, 1)
	ts := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		defer func() { done <- struct{}{} }()

		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(root, false))
		if !assert.NoError(t, err) {
			return
		}
		_ = frankenphp.ServeHTTP(w, req)
	})
	t.Cleanup(ts.Close)

	return ts, done
}

// A client that closes its side before sending the announced body, over a real connection.
func TestRequestBodyClientClosed(t *testing.T) {
	require.NoError(t, frankenphp.Init(frankenphp.WithNumThreads(1)))
	defer frankenphp.Shutdown()

	for _, tc := range []struct {
		name, headers, body string
	}{
		{"content-length", "Content-Type: application/x-www-form-urlencoded\r\nContent-Length: 25\r\n", "name=demo&amount=12"},
		{"chunked", "Content-Type: application/x-www-form-urlencoded\r\nTransfer-Encoding: chunked\r\n", "13\r\nname=demo&amount=12\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, done := newTestdataServer(t)
			marker := filepath.Join(t.TempDir(), "marker")

			conn, err := net.Dial("tcp", ts.Addr())
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()

			_, err = fmt.Fprintf(conn, "POST /read-input-then-touch.php?marker=%s HTTP/1.1\r\nHost: %s\r\n%sConnection: close\r\n\r\n%s",
				marker, ts.Addr(), tc.headers, tc.body)
			require.NoError(t, err)
			tcp, ok := conn.(*net.TCPConn)
			require.True(t, ok, "expected a TCP connection")
			require.NoError(t, tcp.CloseWrite())

			require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
			resp, err := io.ReadAll(conn)
			require.NoError(t, err)

			<-done
			assert.Contains(t, string(resp), "400 Bad Request")
			assert.NoFileExists(t, marker)
		})
	}
}

// An HTTP/2 stream reset by the client while sending its body.
func TestRequestBodyStreamResetHTTP2(t *testing.T) {
	require.NoError(t, frankenphp.Init(frankenphp.WithNumThreads(1)))
	defer frankenphp.Shutdown()

	root := testdataRoot(t)
	done := make(chan int, 1)
	bodyStarted := make(chan struct{})
	var once sync.Once
	addr, client := newH2CServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.Body = &signalingBody{ReadCloser: r.Body, onRead: func() { once.Do(func() { close(bodyStarted) }) }}
		rec := httptest.NewRecorder()
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(root, false))
		if !assert.NoError(t, err) {
			done <- 0

			return
		}
		_ = frankenphp.ServeHTTP(rec, req)
		done <- rec.Code
	})

	marker := filepath.Join(t.TempDir(), "marker")
	pr, pw := io.Pipe()
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/read-input-then-touch.php?marker="+marker, pr)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = 25

	go func() {
		_, _ = pw.Write([]byte("name=demo&amount=12"))
		// reset only once the PHP thread reads the body
		select {
		case <-bodyStarted:
		case <-time.After(5 * time.Second):
		}
		_ = pw.CloseWithError(errors.New("client gave up"))
	}()

	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}

	select {
	case code := <-done:
		assert.Equal(t, http.StatusBadRequest, code)
	case <-time.After(5 * time.Second):
		t.Fatal("the request was not handled")
	}
	assert.NoFileExists(t, marker)
}
