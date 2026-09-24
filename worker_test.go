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
		assert.Contains(t, body, `"type":2`)

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
		assert.Contains(t, body3, "[]")
	}, &testOptions{workerScript: "request-parse-errors.php", nbWorkers: 1, nbParallelRequests: 1, phpIni: map[string]string{"max_input_vars": "10", "post_max_size": "1K", "display_errors": "0"}})
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
		req = req.WithContext(ctx)
		cancel()
		body1, _ := testRequest(req, handler, t)

		assert.Equal(t, "requests:1", body1, "should have handled exactly one request")
		body2, _ := testGet("http://example.com/worker-with-counter.php", handler, t)

		assert.Equal(t, "requests:2", body2, "should not have stopped execution after the first request was aborted")
	}, &testOptions{workerScript: "worker-with-counter.php", nbWorkers: 1, nbParallelRequests: 1})
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
