package frankenphp_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initPhpServer(t *testing.T, opts ...frankenphp.Option) {
	t.Helper()
	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(opts...))
}

func initPhpServerWithOptions(t *testing.T, idx int, serverOpts ...frankenphp.PhpServerOption) *frankenphp.PhpServer {
	t.Helper()

	opts := append([]frankenphp.PhpServerOption{
		frankenphp.WithPhpServerRoot(testDataDir, false),
	}, serverOpts...)

	initPhpServer(t, frankenphp.WithPhpServer(idx, opts...))

	return frankenphp.PhpServers[idx]
}

func phpServerRequest(t *testing.T, server *frankenphp.PhpServer, req *http.Request) (string, *http.Response) {
	t.Helper()

	w := httptest.NewRecorder()
	err := server.ServeHTTP(w, req)
	if err != nil {
		var rejected frankenphp.ErrRejected
		if !errors.As(err, &rejected) {
			require.NoError(t, err)
		}
	}

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body), resp
}

func phpServerGet(t *testing.T, server *frankenphp.PhpServer, url string) (string, *http.Response) {
	t.Helper()

	return phpServerRequest(t, server, httptest.NewRequest(http.MethodGet, url, nil))
}

func TestPhpServer(t *testing.T) {
	t.Run("idx", func(t *testing.T) {
		initPhpServer(t,
			frankenphp.WithPhpServer(1,
				frankenphp.WithPhpServerRoot(testDataDir, false),
				frankenphp.WithPhpServerEnv(map[string]string{"PHP_SERVER_IDX": "1"}),
			),
			frankenphp.WithPhpServer(2,
				frankenphp.WithPhpServerRoot(testDataDir, false),
				frankenphp.WithPhpServerEnv(map[string]string{"PHP_SERVER_IDX": "2"}),
			),
		)

		body1, _ := phpServerGet(t, frankenphp.PhpServers[1], "http://example.com/server-variable.php")
		body2, _ := phpServerGet(t, frankenphp.PhpServers[2], "http://example.com/server-variable.php")

		assert.Contains(t, body1, "[PHP_SERVER_IDX] => 1")
		assert.Contains(t, body2, "[PHP_SERVER_IDX] => 2")
		assert.NotContains(t, body1, "[PHP_SERVER_IDX] => 2")
		assert.NotContains(t, body2, "[PHP_SERVER_IDX] => 1")
	})

	t.Run("root", func(t *testing.T) {
		server := initPhpServerWithOptions(t, 1)

		body, _ := phpServerGet(t, server, "http://example.com/server-globals.php")

		expectedRoot := filepath.Clean(strings.TrimSuffix(testDataDir, string(filepath.Separator)))
		assert.Contains(t, body, "DOCUMENT_ROOT: "+expectedRoot+"\n")
	})

	t.Run("env", func(t *testing.T) {
		server := initPhpServerWithOptions(t, 1, frankenphp.WithPhpServerEnv(map[string]string{
			"PHP_SERVER_TEST_KEY": "from_php_server",
		}))

		body, _ := phpServerGet(t, server, "http://example.com/server-variable.php")

		assert.Contains(t, body, "[PHP_SERVER_TEST_KEY] => from_php_server")
	})

	t.Run("split_path", func(t *testing.T) {
		server := initPhpServerWithOptions(t, 1, frankenphp.WithPhpServerSplitPath([]string{".custom"}))

		body, _ := phpServerGet(t, server, "http://example.com/split-path.custom/pathinfo")

		assert.Contains(t, body, "PATH_INFO: /pathinfo\n")
		assert.Contains(t, body, "SCRIPT_NAME: /split-path.custom\n")
		assert.Contains(t, body, "PHP_SELF: /split-path.custom/pathinfo\n")
	})

	t.Run("logger", func(t *testing.T) {
		logger, buf := newTestLogger(t)

		initPhpServer(t, frankenphp.WithPhpServer(1,
			frankenphp.WithPhpServerRoot(testDataDir, false),
			frankenphp.WithPHPServerLogger(logger),
			frankenphp.WithPhpServerWorker("test", testDataDir+"/index.php", 1),
		))

		_, _ = phpServerGet(t, frankenphp.PhpServers[1], "http://example.com/index.php")

		assert.Contains(t, buf.String(), "request handling started", "should contain the debug message from worker start")
	})

	t.Run("workers_by_path_and_request_matcher", func(t *testing.T) {
		initPhpServer(
			t,
			frankenphp.WithPhpServer(1,
				frankenphp.WithPhpServerRoot(testDataDir, false),
				frankenphp.WithPhpServerWorker("counter", testDataDir+"worker-with-counter.php", 1),
			),
			frankenphp.WithPhpServer(2,
				frankenphp.WithPhpServerRoot(testDataDir, false),
				frankenphp.WithPhpServerWorker("match", testDataDir+"worker-with-counter.php", 1,
					frankenphp.WithWorkerMatchOn(func(r *http.Request) bool {
						return strings.HasPrefix(r.URL.Path, "/match/")
					}),
				),
			),
		)

		body1, _ := phpServerGet(t, frankenphp.PhpServers[1], "http://example.com/worker-with-counter.php")
		body2, _ := phpServerGet(t, frankenphp.PhpServers[1], "http://example.com/worker-with-counter.php")
		body3, _ := phpServerGet(t, frankenphp.PhpServers[2], "http://example.com/match/anything")
		body4, _ := phpServerGet(t, frankenphp.PhpServers[2], "http://example.com/match/anything")
		body5, _ := phpServerGet(t, frankenphp.PhpServers[2], "http://example.com/index.php")

		assert.Equal(t, "requests:1", body1, "should contain the counter for the first worker")
		assert.Equal(t, "requests:2", body2, "should contain the counter for the first worker")
		assert.Equal(t, "requests:1", body3, "should contain the counter for the second worker")
		assert.Equal(t, "requests:2", body4, "should contain the counter for the second worker")
		assert.Contains(t, body5, "I am by birth a Genevese (i not set)", "should fall back to (non-worker) index.php")
	})

	t.Run("disallow_duplicate_worker_filenames_in_php_server", func(t *testing.T) {
		initPhpServer(t, frankenphp.WithPhpServer(1,
			frankenphp.WithPhpServerRoot(testDataDir, false),
			frankenphp.WithPhpServerEnv(map[string]string{"APP_ENV": "staging"}),
			frankenphp.WithPhpServerWorker("env", testDataDir+"worker-with-env.php", 1),
		))

		body, _ := phpServerGet(t, frankenphp.PhpServers[1], "http://example.com/worker-with-env.php")

		assert.Equal(t, "Worker has APP_ENV=staging", body)
	})

	t.Run("duplicate_worker_filenames_in_php_server", func(t *testing.T) {
		t.Cleanup(frankenphp.Shutdown)

		err := frankenphp.Init(
			frankenphp.WithPhpServer(1,
				frankenphp.WithPhpServerRoot(testDataDir, false),
				frankenphp.WithPhpServerWorker("worker1", testDataDir+"worker-with-counter.php", 1),
				frankenphp.WithPhpServerWorker("worker2", testDataDir+"worker-with-counter.php", 1),
			),
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "two workers in a php_server cannot have the same filename")
	})

	t.Run("duplicate_registration", func(t *testing.T) {
		initPhpServer(t,
			frankenphp.WithPhpServer(1,
				frankenphp.WithPhpServerRoot(testDataDir, false),
				frankenphp.WithPhpServerEnv(map[string]string{"PHP_SERVER_IDX": "first"}),
			),
			frankenphp.WithPhpServer(1,
				frankenphp.WithPhpServerRoot(testDataDir+"/other/", false),
				frankenphp.WithPhpServerEnv(map[string]string{"PHP_SERVER_IDX": "second"}),
			),
		)

		body, _ := phpServerGet(t, frankenphp.PhpServers[1], "http://example.com/server-variable.php")

		assert.Contains(t, body, "[PHP_SERVER_IDX] => first")
		assert.NotContains(t, body, "[PHP_SERVER_IDX] => second")
	})

	t.Run("serve_http_validation", func(t *testing.T) {
		server := initPhpServerWithOptions(t, 1)

		req := httptest.NewRequest(http.MethodGet, "http://example.com/server-variable.php", nil)
		req.Header.Add("Content-Length", "-1")
		body, resp := phpServerRequest(t, server, req)

		assert.Equal(t, 400, resp.StatusCode)
		assert.Contains(t, body, "invalid")
	})

	t.Run("not_running", func(t *testing.T) {
		server := &frankenphp.PhpServer{}
		req := httptest.NewRequest(http.MethodGet, "http://example.com/server-variable.php", nil)
		w := httptest.NewRecorder()

		err := server.ServeHTTP(w, req)

		assert.ErrorIs(t, err, frankenphp.ErrNotRunning)
	})
}
