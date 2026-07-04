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

func initServer(t *testing.T, opts ...frankenphp.Option) {
	t.Helper()
	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(opts...))
}

func serverRequest(t *testing.T, serverIdx int, req *http.Request) (string, *http.Response) {
	t.Helper()

	w := httptest.NewRecorder()
	err := frankenphp.ServeHTTPSrv(serverIdx, w, req)
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

func serverGet(t *testing.T, serverIdx int, url string) (string, *http.Response) {
	t.Helper()

	return serverRequest(t, serverIdx, httptest.NewRequest(http.MethodGet, url, nil))
}

func TestServer(t *testing.T) {
	t.Run("idx", func(t *testing.T) {
		initServer(t,
			frankenphp.WithServer(1,
				testDataDir,
				[]string{},
				map[string]string{
					"PHP_SERVER_IDX": "1",
				},
			),
			frankenphp.WithServer(2,
				testDataDir,
				[]string{},
				map[string]string{
					"PHP_SERVER_IDX": "2",
				},
			),
		)

		body1, _ := serverGet(t, 1, "http://example.com/server-variable.php")
		body2, _ := serverGet(t, 2, "http://example.com/server-variable.php")

		assert.Contains(t, body1, "[PHP_SERVER_IDX] => 1")
		assert.Contains(t, body2, "[PHP_SERVER_IDX] => 2")
		assert.NotContains(t, body1, "[PHP_SERVER_IDX] => 2")
		assert.NotContains(t, body2, "[PHP_SERVER_IDX] => 1")
	})

	t.Run("root", func(t *testing.T) {
		initServer(t, frankenphp.WithServer(1,
			testDataDir,
			[]string{},
			map[string]string{},
		))

		body, _ := serverGet(t, 1, "http://example.com/server-globals.php")

		expectedRoot := filepath.Clean(strings.TrimSuffix(testDataDir, string(filepath.Separator)))
		assert.Contains(t, body, "DOCUMENT_ROOT: "+expectedRoot+"\n")
	})

	t.Run("env", func(t *testing.T) {
		initServer(t, frankenphp.WithServer(1,
			testDataDir,
			[]string{},
			map[string]string{
				"PHP_SERVER_TEST_KEY": "from_php_server",
			},
		))

		body, _ := serverGet(t, 1, "http://example.com/server-variable.php")

		assert.Contains(t, body, "[PHP_SERVER_TEST_KEY] => from_php_server")
	})

	t.Run("split_path", func(t *testing.T) {
		initServer(t, frankenphp.WithServer(
			1,
			testDataDir,
			[]string{".custom"},
			map[string]string{},
		))

		body, _ := serverGet(t, 1, "http://example.com/split-path.custom/pathinfo")

		assert.Contains(t, body, "PATH_INFO: /pathinfo\n")
		assert.Contains(t, body, "SCRIPT_NAME: /split-path.custom\n")
		assert.Contains(t, body, "PHP_SELF: /split-path.custom/pathinfo\n")
	})

	t.Run("workers_by_path_and_request_matcher", func(t *testing.T) {
		initServer(
			t,
			frankenphp.WithServer(1,
				testDataDir,
				[]string{},
				map[string]string{},
				frankenphp.WithServerWorker("counter", testDataDir+"worker-with-counter.php", 1),
			),
			frankenphp.WithServer(2,
				testDataDir,
				[]string{},
				map[string]string{},
				frankenphp.WithServerWorker("match", testDataDir+"worker-with-counter.php", 1,
					frankenphp.WithWorkerMatchOn(func(r *http.Request) bool {
						return strings.HasPrefix(r.URL.Path, "/match/")
					}),
				),
			),
		)

		body1, _ := serverGet(t, 1, "http://example.com/worker-with-counter.php")
		body2, _ := serverGet(t, 1, "http://example.com/worker-with-counter.php")
		body3, _ := serverGet(t, 2, "http://example.com/match/anything")
		body4, _ := serverGet(t, 2, "http://example.com/match/anything")
		body5, _ := serverGet(t, 2, "http://example.com/index.php")

		assert.Equal(t, "requests:1", body1, "should contain the counter for the first worker")
		assert.Equal(t, "requests:2", body2, "should contain the counter for the first worker")
		assert.Equal(t, "requests:1", body3, "should contain the counter for the second worker")
		assert.Equal(t, "requests:2", body4, "should contain the counter for the second worker")
		assert.Contains(t, body5, "I am by birth a Genevese (i not set)", "should fall back to (non-worker) index.php")
	})

	t.Run("worker_env_inheritance", func(t *testing.T) {
		initServer(t, frankenphp.WithServer(
			1,
			testDataDir,
			[]string{},
			map[string]string{
				"APP_ENV": "staging",
			},
			frankenphp.WithServerWorker("env", testDataDir+"worker-with-env.php", 1),
		))

		body, _ := serverGet(t, 1, "http://example.com/worker-with-env.php")

		assert.Equal(t, "Worker has APP_ENV=staging", body)
	})

	t.Run("duplicate_worker_filenames_in_php_server", func(t *testing.T) {
		t.Cleanup(frankenphp.Shutdown)

		err := frankenphp.Init(
			frankenphp.WithServer(1,
				testDataDir,
				[]string{},
				map[string]string{},
				frankenphp.WithServerWorker("worker1", testDataDir+"worker-with-counter.php", 1),
				frankenphp.WithServerWorker("worker2", testDataDir+"worker-with-counter.php", 1),
			),
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "two workers in a server cannot have the same filename")
	})

	t.Run("duplicate_registration", func(t *testing.T) {
		initServer(t,
			frankenphp.WithServer(1,
				testDataDir,
				[]string{},
				map[string]string{
					"PHP_SERVER_IDX": "first",
				},
			),
			frankenphp.WithServer(1,
				testDataDir+"/other/",
				[]string{},
				map[string]string{
					"PHP_SERVER_IDX": "second",
				},
			),
		)

		body, _ := serverGet(t, 1, "http://example.com/server-variable.php")

		assert.Contains(t, body, "[PHP_SERVER_IDX] => first", "should contain the first server env variable")
		assert.NotContains(t, body, "[PHP_SERVER_IDX] => second", "should not contain the duplicate server env variable")
	})

	t.Run("serve_http_validation", func(t *testing.T) {
		initServer(t, frankenphp.WithServer(1,
			testDataDir,
			[]string{},
			map[string]string{},
		))

		req := httptest.NewRequest(http.MethodGet, "http://example.com/server-variable.php", nil)
		req.Header.Add("Content-Length", "-1")
		body, resp := serverRequest(t, 1, req)

		assert.Equal(t, 400, resp.StatusCode)
		assert.Contains(t, body, "invalid")
	})

	t.Run("idx_not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/server-variable.php", nil)
		w := httptest.NewRecorder()

		err := frankenphp.ServeHTTPSrv(1, w, req)

		assert.ErrorIs(t, err, frankenphp.ServerNotFoundError)
	})
}
