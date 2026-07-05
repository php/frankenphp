package frankenphp_test

import (
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

func initServers(t *testing.T, opts ...frankenphp.Option) {
	t.Helper()
	t.Cleanup(frankenphp.Shutdown)
	require.NoError(t, frankenphp.Init(opts...))
}

func serverRequest(t *testing.T, serverIdx int, req *http.Request) (string, *http.Response) {
	t.Helper()

	w := httptest.NewRecorder()
	require.NoError(t, frankenphp.ServeHTTPSrv(serverIdx, w, req))

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body), resp
}

func serverGet(t *testing.T, serverIdx int, url string) string {
	t.Helper()

	body, _ := serverRequest(t, serverIdx, httptest.NewRequest(http.MethodGet, url, nil))

	return body
}

func TestServer(t *testing.T) {
	t.Run("idx", func(t *testing.T) {
		initServers(
			t,
			frankenphp.WithServer(1, testDataDir, nil, map[string]string{"PHP_SERVER_IDX_1": "1"}),
			frankenphp.WithServer(2, testDataDir, nil, map[string]string{"PHP_SERVER_IDX_2": "2"}),
		)

		body1 := serverGet(t, 1, "http://example.com/server-variable.php")
		body2 := serverGet(t, 2, "http://example.com/server-variable.php")

		assert.Contains(t, body1, "[PHP_SERVER_IDX_1] => 1")
		assert.Contains(t, body2, "[PHP_SERVER_IDX_2] => 2")
		assert.NotContains(t, body1, "[PHP_SERVER_IDX_2]")
		assert.NotContains(t, body2, "[PHP_SERVER_IDX_1]")
	})

	t.Run("root", func(t *testing.T) {
		initServers(t, frankenphp.WithServer(1, testDataDir, nil, nil))

		body := serverGet(t, 1, "http://example.com/server-globals.php")

		expectedRoot := filepath.Clean(strings.TrimSuffix(testDataDir, string(filepath.Separator)))
		assert.Contains(t, body, "DOCUMENT_ROOT: "+expectedRoot+"\n")
	})

	t.Run("env", func(t *testing.T) {
		initServers(t, frankenphp.WithServer(1, testDataDir, nil, map[string]string{"TEST_123": "123"}))

		body := serverGet(t, 1, "http://example.com/server-variable.php")

		assert.Contains(t, body, "[TEST_123] => 123")
	})

	t.Run("split_path", func(t *testing.T) {
		initServers(t, frankenphp.WithServer(1, testDataDir, []string{".custom"}, nil))

		body := serverGet(t, 1, "http://example.com/split-path.custom/pathinfo")

		assert.Contains(t, body, "PATH_INFO: /pathinfo\n")
		assert.Contains(t, body, "SCRIPT_NAME: /split-path.custom\n")
		assert.Contains(t, body, "PHP_SELF: /split-path.custom/pathinfo\n")
	})

	t.Run("workers_by_path_and_request_matcher", func(t *testing.T) {
		server1Idx := 1
		server2Idx := 2
		initServers(
			t,
			frankenphp.WithServer(server1Idx, testDataDir, nil, nil),
			frankenphp.WithServer(server2Idx, testDataDir, nil, nil),
			frankenphp.WithWorkers("counter", testDataDir+"worker-with-counter.php", 1, frankenphp.WithWorkerServerScope(server1Idx)),
			frankenphp.WithWorkers("match", testDataDir+"worker-with-counter.php", 1,
				frankenphp.WithWorkerServerScope(server2Idx),
				frankenphp.WithWorkerMatcher(func(r *http.Request) bool {
					return strings.HasPrefix(r.URL.Path, "/match/")
				}),
			),
		)

		body1 := serverGet(t, 1, "http://example.com/worker-with-counter.php")
		body2 := serverGet(t, 1, "http://example.com/worker-with-counter.php")
		body3 := serverGet(t, 2, "http://example.com/match/anything")
		body4 := serverGet(t, 2, "http://example.com/match/anything")
		body5 := serverGet(t, 2, "http://example.com/index.php")
		body6 := serverGet(t, 1, "http://example.com/match/anything")

		assert.Equal(t, "requests:1", body1, "could not access the worker by path on server 1")
		assert.Equal(t, "requests:2", body2, "could not access the worker by path on server 1")
		assert.Equal(t, "requests:1", body3, "could not access the worker by matcher on server 2")
		assert.Equal(t, "requests:2", body4, "could not access the worker by matcher on server 2")
		assert.Contains(t, body5, "I am by birth a Genevese (i not set)", "could not access the worker by path on server 2")
		assert.Contains(t, body6, "Failed opening required", "worker is scoped to server 1, so should not be found on server 2")
	})

	t.Run("worker_env_inheritance", func(t *testing.T) {
		initServers(
			t,
			frankenphp.WithServer(1, testDataDir, nil, map[string]string{
				"FROM_SERVER_ENV": "original",
				"FROM_WORKER_ENV": "overridden",
			}),
			frankenphp.WithWorkers(
				"env",
				testDataDir+"env/env.php",
				1,
				frankenphp.WithWorkerServerScope(1),
				frankenphp.WithWorkerEnv(map[string]string{
					"FROM_WORKER_ENV": "original",
				}),
			),
		)

		body := serverGet(t, 1, "http://example.com/env/env.php?keys[]=FROM_SERVER_ENV&keys[]=FROM_WORKER_ENV")

		assert.Equal(
			t,
			"FROM_SERVER_ENV=original,FROM_WORKER_ENV=original",
			body,
			"should contain the server env and not override the worker env",
		)
	})

	t.Run("error_on_duplicate_worker_filenames", func(t *testing.T) {
		t.Cleanup(frankenphp.Shutdown)

		err := frankenphp.Init(
			frankenphp.WithServer(1, testDataDir, nil, nil),
			frankenphp.WithWorkers("worker1", testDataDir+"worker-with-counter.php", 1, frankenphp.WithWorkerServerScope(1)),
			frankenphp.WithWorkers("worker2", testDataDir+"worker-with-counter.php", 1, frankenphp.WithWorkerServerScope(1)),
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "two workers in a server cannot have the same filename")
	})

	t.Run("error_on_duplicate_registration", func(t *testing.T) {
		err := frankenphp.Init(
			frankenphp.WithServer(1, testDataDir, nil, nil),
			frankenphp.WithServer(1, testDataDir, nil, nil),
		)

		assert.ErrorIs(t, err, frankenphp.ErrAlreadyRegistered)
	})

	t.Run("error_on_missing_server", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/server-variable.php", nil)
		w := httptest.NewRecorder()

		err := frankenphp.ServeHTTPSrv(1, w, req)

		assert.ErrorIs(t, err, frankenphp.ErrServerNotFound)
	})
}
