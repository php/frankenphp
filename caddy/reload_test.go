package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

var reloadTestEntered, reloadTestRelease chan struct{}

type reloadTestPause struct {
	Pause bool `json:"pause"`
}

func (*reloadTestPause) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.frankenphp_test_reload_pause",
		New: func() caddy.Module { return new(reloadTestPause) },
	}
}

func (p *reloadTestPause) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if p.Pause {
		close(reloadTestEntered)
		<-reloadTestRelease
	}
	return next.ServeHTTP(w, r)
}

func init() {
	caddy.RegisterModule(new(reloadTestPause))
}

func TestReloadPreservesRequestAndResponse(t *testing.T) {
	for _, duringRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("during_request=%t", duringRequest), func(t *testing.T) {
			t.Cleanup(func() {
				require.NoError(t, caddy.Stop())
				frankenphp.Shutdown()
				activeApp.Store(nil)
			})
			reloadTestEntered, reloadTestRelease = make(chan struct{}), make(chan struct{})
			defer func() {
				select {
				case <-reloadTestRelease:
				default:
					close(reloadTestRelease)
				}
			}()
			load := func(policy string, pause bool) *caddyhttp.Server {
				t.Helper()
				config := fmt.Sprintf(`{
					"admin":{"disabled":true,"config":{"persist":false}},
					"logging":{"logs":{"default":{"level":"ERROR"}}},
					"apps":{
						"frankenphp":{"num_threads":2},
						"http":{"servers":{"test":{
							"listen":["127.0.0.1:0"],
							"automatic_https":{"disable":true},
							"routes":[
							{"match":[{"path":["/static"]}],"handle":[{"handler":"static_response","body":%q}]},
							{"match":[{"host":["app.example"]}],"handle":[
								{"handler":"headers","request":{"set":{"Host":["internal.example"]},"add":{"X-Tag":["tag"]}},"response":{"add":{"X-Trace":[%q]}}},
								{"handler":"headers","response":{"set":{"X-Policy":[%q]},"deferred":true}},
								{"handler":"rewrite","uri":"/reload.php"},
								{"handler":"frankenphp_test_reload_pause","pause":%t},
								{"handler":"php","root":"../testdata"}
							]}]
						}}}
					}
				}`, policy, policy, policy, pause)
				require.NoError(t, caddy.Load([]byte(config), true))
				app := activeApp.Load()
				require.Equal(t, "app.example", app.modules[0].server.Name())
				return app.httpApp.Servers["test"]
			}
			request := func(server *caddyhttp.Server, response *httptest.ResponseRecorder) {
				r := httptest.NewRequest(http.MethodPost, "http://app.example/api?query=value", strings.NewReader("payload"))
				server.ServeHTTP(response, r)
			}

			old := load("old", duringRequest)
			forwarded := httptest.NewRecorder()
			done := make(chan struct{})
			if duringRequest {
				go func() {
					defer close(done)
					request(old, forwarded)
				}()
				select {
				case <-reloadTestEntered:
				case <-time.After(5 * time.Second):
					t.Fatal("request did not reach the old PHP handler")
				}
			}

			current := load("new", false)
			static := httptest.NewRecorder()
			old.ServeHTTP(static, httptest.NewRequest(http.MethodGet, "http://app.example/static", nil))
			require.Equal(t, "old", static.Body.String())
			if duringRequest {
				close(reloadTestRelease)
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("request handoff did not finish")
				}
			} else {
				request(old, forwarded)
			}
			direct := httptest.NewRecorder()
			request(current, direct)
			require.Equal(t, http.StatusOK, forwarded.Code)
			require.Equal(t, direct.Body.String(), forwarded.Body.String())
			// keeps the old tree's header state
			require.Equal(t, []string{"old"}, forwarded.Header().Values("X-Trace"))
			require.Equal(t, "old", forwarded.Header().Get("X-Policy"))
			var result map[string]string
			require.NoError(t, json.Unmarshal(forwarded.Body.Bytes(), &result))
			require.Equal(t, map[string]string{
				"host": "internal.example",
				"tag":  "tag",
				"uri":  "/api?query=value",
				"body": "payload",
			}, result)
		})
	}
}

func newReloadTestApp(root string, worker bool) *FrankenPHPApp {
	app := &FrankenPHPApp{
		NumThreads: 1,
		MaxThreads: 2,
		ctx:        context.Background(),
		logger:     slog.New(slog.DiscardHandler),
		started:    make(chan any),
	}
	module := &FrankenPHPModule{Name: "site", resolvedDocumentRoot: root, app: app}
	if worker {
		module.Workers = []workerConfig{{Name: "site-worker", FileName: filepath.Join(root, "index.php"), Num: 1}}
	}
	app.modules = []*FrankenPHPModule{module}
	return app
}

func newReloadTestRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "http://app.example/index.php", nil)
	ctx := context.WithValue(r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer())
	ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, *r)
	return r.WithContext(ctx)
}

func waitForReloadTest(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reload test did not finish")
	}
}

type reloadTestWaitingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *reloadTestWaitingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestReloadUsesNewRootWhileShutdownDrains(t *testing.T) {
	for _, worker := range []bool{false, true} {
		t.Run(fmt.Sprintf("worker=%t", worker), func(t *testing.T) {
			root := t.TempDir()
			oldRoot, newRoot := filepath.Join(root, "old"), filepath.Join(root, "new")
			for path, body := range map[string]string{oldRoot: "OLD", newRoot: "NEW"} {
				require.NoError(t, os.Mkdir(path, 0700))
				script := fmt.Sprintf("<?php echo %q;", body)
				if worker {
					script = fmt.Sprintf("<?php while (frankenphp_handle_request(function () { echo %q; })) {}", body)
				}
				require.NoError(t, os.WriteFile(filepath.Join(path, "index.php"), []byte(script), 0600))
			}

			draining, resume := make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(resume) })
			var reloadDone, requestDone chan struct{}
			t.Cleanup(func() {
				release()
				if reloadDone != nil {
					waitForReloadTest(t, reloadDone)
				}
				if requestDone != nil {
					waitForReloadTest(t, requestDone)
				}
				frankenphp.Shutdown()
				activeApp.Store(nil)
			})
			old := newReloadTestApp(oldRoot, worker)
			old.NumThreads, old.MaxThreads = 1, 3
			old.Workers = []workerConfig{{
				Name: "shutdown-hook", FileName: "../testdata/worker-with-counter.php", Num: 1,
				options: []frankenphp.WorkerOption{frankenphp.WithWorkerOnServerShutdown(func() {
					close(draining)
					<-resume
				})},
			}}
			require.NoError(t, old.Start())

			current := newReloadTestApp(newRoot, worker)
			waiting := &reloadTestWaitingContext{Context: context.Background(), waiting: make(chan struct{})}
			current.ctx = waiting
			reloadDone = make(chan struct{})
			var reloadErr error
			go func() {
				defer close(reloadDone)
				reloadErr = current.Start()
			}()
			waitForReloadTest(t, draining)

			response := httptest.NewRecorder()
			requestDone = make(chan struct{})
			var requestErr error
			go func() {
				defer close(requestDone)
				requestErr = old.modules[0].ServeHTTP(response, newReloadTestRequest(), nil)
			}()
			select {
			case <-waiting.waiting:
			case <-requestDone:
			case <-time.After(5 * time.Second):
				t.Fatal("request neither waited for startup nor finished")
			}
			release()
			waitForReloadTest(t, reloadDone)
			waitForReloadTest(t, requestDone)
			require.NoError(t, reloadErr)
			require.NoError(t, requestErr)
			require.Equal(t, "NEW", response.Body.String())
		})
	}
}

func TestFailedReloadReturnsServiceUnavailable(t *testing.T) {
	t.Cleanup(func() {
		frankenphp.Shutdown()
		activeApp.Store(nil)
	})
	old := newReloadTestApp("../testdata", false)
	require.NoError(t, old.Start())
	invalid := newReloadTestApp("../testdata", false)
	invalid.Workers = []workerConfig{{
		Name: "boot-failure", FileName: "../testdata/failing-worker.php", Num: 1, MaxConsecutiveFailures: 0,
	}}
	require.Error(t, invalid.Start())
	require.Nil(t, activeApp.Load())

	err := old.modules[0].ServeHTTP(httptest.NewRecorder(), newReloadTestRequest(), nil)
	var handlerErr caddyhttp.HandlerError
	require.ErrorAs(t, err, &handlerErr)
	require.Equal(t, http.StatusServiceUnavailable, handlerErr.StatusCode)
}

func TestStartupWaitUsesConfiguredLimits(t *testing.T) {
	for _, tc := range []struct {
		name           string
		grace, maxWait time.Duration
	}{
		{"grace_period", 10 * time.Millisecond, 0},
		{"max_wait_time", time.Hour, 10 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &FrankenPHPApp{
				ctx: context.Background(), started: make(chan any), MaxWaitTime: tc.maxWait,
				httpApp: &caddyhttp.App{GracePeriod: caddy.Duration(tc.grace)},
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := app.waitForStartup(ctx)
			require.ErrorIs(t, err, frankenphp.ErrMaxWaitTimeExceeded)
		})
	}
	t.Run("no_limit", func(t *testing.T) {
		app := &FrankenPHPApp{ctx: context.Background(), started: make(chan any), httpApp: &caddyhttp.App{}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, app.waitForStartup(ctx), context.Canceled)
	})
}
