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

func serverRequest(t *testing.T, server *frankenphp.Server, req *http.Request) (string, *http.Response) {
	t.Helper()

	w := httptest.NewRecorder()
	require.NoError(t, server.ServeHTTP(w, req))

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body), resp
}

func serverGet(t *testing.T, server *frankenphp.Server, url string) string {
	t.Helper()

	body, _ := serverRequest(t, server, httptest.NewRequest(http.MethodGet, url, nil))

	return body
}

func TestServer(t *testing.T) {
	t.Run("idx", func(t *testing.T) {
		server1, _ := frankenphp.NewServer("", testDataDir, nil, map[string]string{"PHP_SERVER_IDX_1": "1"}, nil)
		server2, _ := frankenphp.NewServer("", testDataDir, nil, map[string]string{"PHP_SERVER_IDX_2": "2"}, nil)

		initServers(t, frankenphp.WithServer(server1), frankenphp.WithServer(server2))

		body1 := serverGet(t, server1, "http://example.com/server-variable.php")
		body2 := serverGet(t, server2, "http://example.com/server-variable.php")

		assert.Contains(t, body1, "[PHP_SERVER_IDX_1] => 1")
		assert.Contains(t, body2, "[PHP_SERVER_IDX_2] => 2")
		assert.NotContains(t, body1, "[PHP_SERVER_IDX_2]")
		assert.NotContains(t, body2, "[PHP_SERVER_IDX_1]")
	})

	t.Run("name", func(t *testing.T) {
		named, _ := frankenphp.NewServer("api", testDataDir, nil, nil, nil)
		unnamed, _ := frankenphp.NewServer("", testDataDir, nil, nil, nil)

		initServers(t, frankenphp.WithServer(named), frankenphp.WithServer(unnamed))

		assert.Equal(t, "api", named.Name())
		// an empty name defaults to the server index at registration
		assert.Equal(t, "server_1", unnamed.Name())
	})

	t.Run("root", func(t *testing.T) {
		server, _ := frankenphp.NewServer("", testDataDir, nil, nil, nil)
		initServers(t, frankenphp.WithServer(server))

		body := serverGet(t, server, "http://example.com/server-globals.php")

		expectedRoot := filepath.Clean(strings.TrimSuffix(testDataDir, string(filepath.Separator)))
		assert.Contains(t, body, "DOCUMENT_ROOT: "+expectedRoot+"\n")
	})

	t.Run("env", func(t *testing.T) {
		server, _ := frankenphp.NewServer("", testDataDir, nil, map[string]string{"TEST_123": "123"}, nil)
		initServers(t, frankenphp.WithServer(server))

		body := serverGet(t, server, "http://example.com/server-variable.php")

		assert.Contains(t, body, "[TEST_123] => 123")
	})

	t.Run("split_path", func(t *testing.T) {
		server, _ := frankenphp.NewServer("", testDataDir, []string{".custom"}, nil, nil)
		initServers(t, frankenphp.WithServer(server))

		body := serverGet(t, server, "http://example.com/split-path.custom/pathinfo")

		assert.Contains(t, body, "PATH_INFO: /pathinfo\n")
		assert.Contains(t, body, "SCRIPT_NAME: /split-path.custom\n")
		assert.Contains(t, body, "PHP_SELF: /split-path.custom/pathinfo\n")
	})

	t.Run("workers_by_path_and_request_matcher", func(t *testing.T) {
		server1, _ := frankenphp.NewServer("", testDataDir, nil, nil, nil)
		server2, _ := frankenphp.NewServer("", testDataDir, nil, nil, nil)
		initServers(
			t,
			frankenphp.WithServer(server1),
			frankenphp.WithServer(server2),
			frankenphp.WithWorkers("counter", testDataDir+"worker-with-counter.php", 1, frankenphp.WithWorkerServerScope(server1)),
			frankenphp.WithWorkers("match", testDataDir+"worker-with-counter.php", 1,
				frankenphp.WithWorkerServerScope(server2),
				frankenphp.WithWorkerMatcher(func(r *http.Request) bool {
					return strings.HasPrefix(r.URL.Path, "/match/")
				}),
			),
		)

		body1 := serverGet(t, server1, "http://example.com/worker-with-counter.php")
		body2 := serverGet(t, server1, "http://example.com/worker-with-counter.php")
		body3 := serverGet(t, server2, "http://example.com/match/anything")
		body4 := serverGet(t, server2, "http://example.com/match/anything")
		body5 := serverGet(t, server2, "http://example.com/index.php")
		body6 := serverGet(t, server1, "http://example.com/match/anything")

		assert.Equal(t, "requests:1", body1, "could not access the worker by path on server 1")
		assert.Equal(t, "requests:2", body2, "could not access the worker by path on server 1")
		assert.Equal(t, "requests:1", body3, "could not access the worker by matcher on server 2")
		assert.Equal(t, "requests:2", body4, "could not access the worker by matcher on server 2")
		assert.Contains(t, body5, "I am by birth a Genevese (i not set)", "could not access the worker by path on server 2")
		assert.Contains(t, body6, "Failed opening required", "worker is scoped to server 1, so should not be found on server 2")
	})

	t.Run("worker_env_inheritance", func(t *testing.T) {
		server, _ := frankenphp.NewServer("", testDataDir, nil, map[string]string{
			"FROM_SERVER_ENV": "original",
			"FROM_WORKER_ENV": "overridden",
		}, nil)
		initServers(
			t,
			frankenphp.WithPhpIni(map[string]string{"variables_order": "EGPCS"}),
			frankenphp.WithServer(server),
			frankenphp.WithWorkers(
				"env",
				testDataDir+"env/env.php",
				1,
				frankenphp.WithWorkerServerScope(server),
				frankenphp.WithWorkerEnv(map[string]string{
					"FROM_WORKER_ENV": "original",
				}),
			),
		)

		body := serverGet(t, server, "http://example.com/env/env.php?keys[]=FROM_SERVER_ENV&keys[]=FROM_WORKER_ENV")

		assert.Equal(
			t,
			"FROM_SERVER_ENV=original,FROM_WORKER_ENV=original",
			body,
			"should contain the server env and not override the worker env",
		)
	})

	t.Run("error_on_duplicate_worker_filenames", func(t *testing.T) {
		t.Cleanup(frankenphp.Shutdown)

		server, _ := frankenphp.NewServer("", testDataDir, nil, nil, nil)
		err := frankenphp.Init(
			frankenphp.WithServer(server),
			frankenphp.WithWorkers("worker1", testDataDir+"worker-with-counter.php", 1, frankenphp.WithWorkerServerScope(server)),
			frankenphp.WithWorkers("worker2", testDataDir+"worker-with-counter.php", 1, frankenphp.WithWorkerServerScope(server)),
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "two workers in a server cannot have the same filename")
	})

	t.Run("error_on_missing_registration", func(t *testing.T) {
		server, _ := frankenphp.NewServer("", testDataDir, nil, nil, nil)

		assert.ErrorIs(t, server.ServeHTTP(nil, nil), frankenphp.ErrNotRunning)
	})

	t.Run("server_logger", func(t *testing.T) {
		logger, buf := newTestLogger(t)
		server, _ := frankenphp.NewServer("", testDataDir, nil, nil, logger)
		initServers(t, frankenphp.WithServer(server))

		_ = serverGet(t, server, "http://example.com/log-frankenphp_log.php")
		_ = serverGet(t, server, "http://example.com/log-frankenphp_log.php")

		assert.Contains(t, buf.String(), "some error message")
	})
}
