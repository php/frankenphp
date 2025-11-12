package frankenphp_test

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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
	}, nil)
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
	obs, logs := observer.New(zapcore.InfoLevel)
	logger := slog.New(zapslog.NewHandler(obs))

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

	for _, l := range logs.FilterFieldKey("exit_status").All() {
		assert.Failf(t, "unexpected exit status", "exit status: %d", l.ContextMap()["exit_status"])
	}
}

func ExampleServeHTTP_workers() {
	if err := frankenphp.Init(
		frankenphp.WithWorkers("worker1", "worker1.php", 4,
			frankenphp.WithWorkerEnv(map[string]string{"ENV1": "foo"}),
			frankenphp.WithWorkerWatchMode([]string{}),
		),
		frankenphp.WithWorkers("worker2", "worker2.php", 2,
			frankenphp.WithWorkerEnv(map[string]string{"ENV2": "bar"}),
			frankenphp.WithWorkerWatchMode([]string{}),
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
	require.NoError(t, os.Setenv("CUSTOM_OS_ENV_VARIABLE", "custom_env_variable_value"))

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
