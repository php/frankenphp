package frankenphp_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		formData := url.Values{"baz": {"bat"}}
		req := httptest.NewRequest("POST", "http://example.com/worker.php?foo=bar", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", strings.Clone("application/x-www-form-urlencoded"))
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), fmt.Sprintf("Requests handled: %d", i*2))

		formData2 := url.Values{"baz2": {"bat2"}}
		req2 := httptest.NewRequest("POST", "http://example.com/worker.php?foo2=bar2", strings.NewReader(formData2.Encode()))
		req2.Header.Set("Content-Type", strings.Clone("application/x-www-form-urlencoded"))

		w2 := httptest.NewRecorder()
		handler(w2, req2)

		resp2 := w2.Result()
		body2, _ := io.ReadAll(resp2.Body)

		assert.Contains(t, string(body2), fmt.Sprintf("Requests handled: %d", i*2+1))
	}, &testOptions{workerScript: "worker.php", nbWorkers: 1, nbParallelRequests: 1})
}

func TestWorkerSurvivesRequestParseErrors(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		// max_input_vars exceeded while the worker script has a throwing error
		// handler installed (https://github.com/php/frankenphp/issues/2631)
		params := make([]string, 20)
		for i := range params {
			params[i] = fmt.Sprintf("p%d=%d", i, i)
		}
		body, resp := testGet("http://example.com/request-parse-errors.php?"+strings.Join(params, "&"), handler, t)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Contains(t, body, "requests:1")
		assert.Contains(t, body, "Input variables exceeded")

		// post_max_size exceeded: the same worker must have survived the
		// previous parse failure (the counter proves no restart happened)
		req := httptest.NewRequest("POST", "http://example.com/request-parse-errors.php", strings.NewReader(strings.Repeat("a=1&", 600)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		body2, resp2 := testRequest(req, handler, t)
		assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)
		assert.Contains(t, body2, "requests:2")
		assert.Contains(t, body2, "exceeds the limit")

		// clean request: recorded errors must be reset between requests
		body3, resp3 := testGet("http://example.com/request-parse-errors.php", handler, t)
		assert.Equal(t, http.StatusOK, resp3.StatusCode)
		assert.Contains(t, body3, "requests:3")
	}, &testOptions{workerScript: "request-parse-errors.php", nbWorkers: 1, nbParallelRequests: 1, phpIni: map[string]string{"max_input_vars": "10", "post_max_size": "1K", "display_errors": "0"}})
}

func TestErrorHandlerPersistsAcrossRequests(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for i := 0; i < 5; i++ {
			body, resp := testGet("http://example.com/error-handler.php", handler, t)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "handler is defined")
			assert.Contains(t, body, fmt.Sprintf("requests:%d", i+1))
		}
	}, &testOptions{workerScript: "error-handler.php", nbWorkers: 1, nbParallelRequests: 1})
}

func TestWorkerDie(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/die.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}, &testOptions{workerScript: "die.php", nbWorkers: 1, nbParallelRequests: 10})
}

func TestNonWorkerModeAlwaysWorks(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "I am by birth a Genevese")
	}, &testOptions{workerScript: "phpinfo.php"})
}

func TestCannotCallHandleRequestInNonWorkerMode(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/non-worker.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "<b>Fatal error</b>:  Uncaught RuntimeException: frankenphp_handle_request() called while not in worker mode")
	}, &testOptions{phpIni: map[string]string{"display_errors": "1", "html_errors": "1"}})
}

func TestWorkerEnv(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/worker-env.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, fmt.Sprintf("bar%d", i), string(body))
	}, &testOptions{workerScript: "worker-env.php", nbWorkers: 1, env: map[string]string{"FOO": "bar"}, nbParallelRequests: 10})
}

// A client that disconnects before its request reaches a worker thread must not crash the process.
func TestWorkerCanceledRequestWithUncommonHeader(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		req := httptest.NewRequestWithContext(ctx, "GET", "http://example.com/worker.php", nil)
		req.Header.Set(fmt.Sprintf("X-Uncached-%d", i), "1")
		handler(httptest.NewRecorder(), req)
	}, &testOptions{workerScript: "worker.php", nbWorkers: 1, nbParallelRequests: 5})
}

// https://github.com/php/frankenphp/issues/2697
func TestWorkerBootHasRequestURI(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/worker-boot-request-uri.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		body, _ := io.ReadAll(w.Result().Body)

		assert.Equal(t, "worker-boot-request-uri.php", string(body))
	}, &testOptions{workerScript: "worker-boot-request-uri.php", nbWorkers: 1, nbParallelRequests: 1})
}

func TestWorkerGetOpt(t *testing.T) {
	logger, buf := newTestLogger(t)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/worker-getopt.php?i=%d", i), nil)
		req.Header.Add("Request", strconv.Itoa(i))
		w := httptest.NewRecorder()

		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), fmt.Sprintf("[HTTP_REQUEST] => %d", i))
		assert.Contains(t, string(body), fmt.Sprintf("[REQUEST_URI] => /worker-getopt.php?i=%d", i))
	}, &testOptions{logger: logger, workerScript: "worker-getopt.php", env: map[string]string{"FOO": "bar"}})

	assert.NotRegexp(t, buf.String(), "exit_status=[1-9]")
}

// Ending the worker loop right after a non-form POST must not free its Content-Type twice.
func TestWorkerScriptEndsAfterNonFormPost(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("POST", fmt.Sprintf("http://example.com/worker-restart.php?i=%d", i), strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler(w, req)

		body, _ := io.ReadAll(w.Result().Body)
		assert.Equal(t, fmt.Sprintf("Counter (%d)", i), string(body))
	}, &testOptions{workerScript: "worker-restart.php", nbWorkers: 1, nbParallelRequests: 1, env: map[string]string{"EVERY": "1"}})
}

func TestWorkerCrashIsLoggedAboveDebugLevel(t *testing.T) {
	logger, buf := newTestLogger(t)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/crashing-worker.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}, &testOptions{logger: logger, workerScript: "crashing-worker.php", nbWorkers: 1, nbParallelRequests: 4})

	assert.Regexp(t, `level=WARN msg="unexpected termination, restarting" worker=\S+ thread=\d+ exit_status=1`, buf.String())
}

func ExampleServeHTTP_workers() {
	if err := frankenphp.Init(
		frankenphp.WithWorkers("worker1", "worker1.php", 4,
			frankenphp.WithWorkerEnv(map[string]string{"ENV1": "foo"}),
			frankenphp.WithWorkerWatchMode([]string{}),
			frankenphp.WithWorkerMaxFailures(0),
		),
		frankenphp.WithWorkers("worker2", "worker2.php", 2,
			frankenphp.WithWorkerEnv(map[string]string{"ENV2": "bar"}),
			frankenphp.WithWorkerWatchMode([]string{}),
			frankenphp.WithWorkerMaxFailures(0),
		),
	); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot("/path/to/document/root", false))
		if err != nil {
			panic(err)
		}

		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Request bodies must be released between requests without freeing temp streams the application still holds.
func TestWorkerTemporaryStreams(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		var first string
		for i := range 5 {
			req := httptest.NewRequest("POST", "http://example.com/worker-temp-streams.php", strings.NewReader("body"))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			handler(w, req)

			body, _ := io.ReadAll(w.Result().Body)
			assert.True(t, strings.HasPrefix(string(body), "hello "), string(body))
			if i == 0 {
				first = string(body)
			}
			assert.Equal(t, first, string(body), "streams leaked across requests")
		}
	}, &testOptions{workerScript: "worker-temp-streams.php", nbWorkers: 1, nbParallelRequests: 1})
}

// A php://input handle kept across requests must keep reading the body it was opened on.
func TestWorkerKeptInputStream(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for i, expected := range []string{"body0", "body0", "body0"} {
			req := httptest.NewRequest("POST", "http://example.com/worker-input-stream.php", strings.NewReader(fmt.Sprintf("body%d", i)))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			handler(w, req)

			body, _ := io.ReadAll(w.Result().Body)
			assert.Equal(t, expected, string(body))
		}
	}, &testOptions{workerScript: "worker-input-stream.php", nbWorkers: 1, nbParallelRequests: 1})
}

// A request body closed by the application must not be released a second time.
func TestWorkerClosedRequestBody(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("POST", "http://example.com/worker-input-stream.php?close=1", strings.NewReader(fmt.Sprintf("body%d", i)))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		handler(w, req)

		body, _ := io.ReadAll(w.Result().Body)
		assert.Equal(t, fmt.Sprintf("body%d", i), string(body))
	}, &testOptions{workerScript: "worker-input-stream.php", nbWorkers: 1, nbParallelRequests: 1})
}

func TestWorkerHasOSEnvironmentVariableInSERVER(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/worker.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "CUSTOM_OS_ENV_VARIABLE")
		assert.Contains(t, string(body), "custom_env_variable_value")
	}, &testOptions{workerScript: "worker.php", nbWorkers: 1, nbParallelRequests: 1})
}

func TestKeepRunningOnConnectionAbort(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/worker-with-counter.php", nil)

		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		req = req.WithContext(ctx)
		w := &cancelOnWriteResponseWriter{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
		handler(w, req)

		assert.ErrorIs(t, ctx.Err(), context.Canceled, "should have aborted during the response")
		assert.Equal(t, "requests:1", w.Body.String(), "should have handled exactly one request")
		body2, _ := testGet("http://example.com/worker-with-counter.php", handler, t)

		assert.Equal(t, "requests:2", body2, "should not have stopped execution after the first request was aborted")
	}, &testOptions{workerScript: "worker-with-counter.php", nbWorkers: 1, nbParallelRequests: 1})
}

// Cancel only once the worker writes, so dispatch cannot discard the request first.
type cancelOnWriteResponseWriter struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (w *cancelOnWriteResponseWriter) Write(p []byte) (int, error) {
	w.cancel()
	return w.ResponseRecorder.Write(p)
}

// Per-request env must not leak into later requests on the same worker thread, even after putenv().
func TestWorkerRequestEnvDoesNotLeak(t *testing.T) {
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		serve := func(query string, env map[string]string) string {
			r := httptest.NewRequest("GET", "http://example.com/worker-request-env.php?"+query, nil)
			req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(testDataDir, false), frankenphp.WithRequestEnv(env))
			require.NoError(t, err)

			w := httptest.NewRecorder()
			require.NoError(t, frankenphp.ServeHTTP(w, req))
			body, _ := io.ReadAll(w.Result().Body)

			return string(body)
		}

		assert.JSONEq(t, `{"server":"a","secret":"a","other":"a","worker":"bar","put":false,"all":true,"env":false}`, serve("", map[string]string{"SECRET": "a", "OTHER": "a"}))
		assert.JSONEq(t, `{"server":false,"secret":false,"other":"b","worker":"bar","put":false,"all":false,"env":false}`, serve("", map[string]string{"OTHER": "b"}))
		assert.JSONEq(t, `{"server":false,"secret":false,"other":false,"worker":"bar","put":false,"all":false,"env":false}`, serve("", nil))
		assert.JSONEq(t, `{"server":"c","secret":"c","other":"c","worker":"bar","put":"1","all":true,"env":false}`, serve("put=1", map[string]string{"SECRET": "c", "OTHER": "c"}))
		assert.JSONEq(t, `{"server":false,"secret":false,"other":"d","worker":"bar","put":"1","all":false,"env":false}`, serve("", map[string]string{"OTHER": "d"}))
	}, &testOptions{workerScript: "worker-request-env.php", nbWorkers: 1, nbParallelRequests: 1, env: map[string]string{"FOO": "bar"}, phpIni: map[string]string{"variables_order": "EGPCS"}})
}

// TestWorkerMaxRequests verifies that a worker restarts after reaching max_requests.
func TestWorkerMaxRequests(t *testing.T) {
	const maxRequests = 5
	const totalRequests = 20

	var buf syncBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		instanceIDs := make(map[string]int)

		for range totalRequests {
			body, resp := testGet("http://example.com/worker-counter-persistent.php", handler, t)
			assert.Equal(t, 200, resp.StatusCode)

			parts := strings.Split(body, ",")
			if len(parts) == 2 {
				instanceID := strings.TrimPrefix(parts[0], "instance:")
				instanceIDs[instanceID]++
			}
		}

		t.Logf("Unique worker instances seen: %d (expected >= %d)", len(instanceIDs), totalRequests/maxRequests)
		for id, count := range instanceIDs {
			t.Logf("  instance %s: handled %d requests", id, count)
		}

		assert.GreaterOrEqual(t, len(instanceIDs), totalRequests/maxRequests)

		for id, count := range instanceIDs {
			assert.LessOrEqual(t, count, maxRequests,
				fmt.Sprintf("instance %s handled %d requests, exceeding max_requests=%d", id, count, maxRequests))
		}

		restartCount := strings.Count(buf.String(), "max requests reached, restarting")
		t.Logf("Worker restarts observed: %d", restartCount)
		assert.GreaterOrEqual(t, restartCount, 2)
	}, &testOptions{
		workerScript:       "worker-counter-persistent.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		logger:             logger,
		initOpts:           []frankenphp.Option{frankenphp.WithNumThreads(2), frankenphp.WithMaxRequests(maxRequests)},
	})
}

// TestWorkerMaxRequestsHighConcurrency verifies max_requests works under concurrent load.
func TestWorkerMaxRequestsHighConcurrency(t *testing.T) {
	const maxRequests = 10
	const totalRequests = 200

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		var (
			mu          sync.Mutex
			instanceIDs = make(map[string]int)
		)
		var wg sync.WaitGroup

		for range totalRequests {
			wg.Go(func() {
				body, resp := testGet("http://example.com/worker-counter-persistent.php", handler, t)
				assert.Equal(t, 200, resp.StatusCode)

				mu.Lock()
				parts := strings.Split(body, ",")
				if len(parts) == 2 {
					instanceID := strings.TrimPrefix(parts[0], "instance:")
					instanceIDs[instanceID]++
				}
				mu.Unlock()
			})
		}
		wg.Wait()

		t.Logf("instances: %d", len(instanceIDs))
		assert.Greater(t, len(instanceIDs), 4, "workers should have restarted multiple times")

		for id, count := range instanceIDs {
			assert.LessOrEqual(t, count, maxRequests,
				fmt.Sprintf("instance %s handled %d requests, exceeding max_requests=%d", id, count, maxRequests))
		}
	}, &testOptions{
		workerScript:       "worker-counter-persistent.php",
		nbWorkers:          4,
		nbParallelRequests: 1,
		initOpts:           []frankenphp.Option{frankenphp.WithNumThreads(5), frankenphp.WithMaxRequests(maxRequests)},
	})
}
