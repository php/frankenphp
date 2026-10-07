package caddy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

type reloadWaitingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *reloadWaitingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestReloadUsesNewPHPOptions(t *testing.T) {
	for _, worker := range []bool{false, true} {
		t.Run(fmt.Sprintf("worker=%t", worker), func(t *testing.T) {
			newApp := func(body string) *FrankenPHPApp {
				root, err := filepath.EvalSymlinks(t.TempDir())
				require.NoError(t, err)
				script := fmt.Sprintf("<?php $handle = function () { echo %q . file_get_contents('php://input'); };", body)
				if worker {
					script += "while (frankenphp_handle_request($handle)) {}"
				} else {
					script += "$handle();"
				}
				require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(script), 0600))
				app := &FrankenPHPApp{NumThreads: 1, MaxThreads: 3, ctx: context.Background(), logger: slog.New(slog.DiscardHandler), started: make(chan any)}
				module := &FrankenPHPModule{Name: "site", resolvedDocumentRoot: root, app: app}
				if worker {
					module.Workers = []workerConfig{{Name: "site-worker", FileName: filepath.Join(root, "index.php"), Num: 1}}
				}
				app.modules = []*FrankenPHPModule{module}
				return app
			}
			old, current := newApp("old:"), newApp("new:")
			draining, resume := make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(resume) })
			t.Cleanup(func() { release(); frankenphp.Shutdown(); activeApp.Store(nil) })
			var external frankenphp.Workers
			var hookErr error
			external, option := frankenphp.WithExtensionWorkers("hook", "../testdata/worker-with-counter.php", 1,
				frankenphp.WithWorkerOnServerShutdown(func() {
					hookErr = external.SendRequest(httptest.NewRecorder(), httptest.NewRequest("GET", "http://localhost/", nil))
					close(draining)
					<-resume
				}))
			old.provisionOpts = []frankenphp.Option{option}
			require.NoError(t, old.Start())
			reloaded := make(chan error, 1)
			go func() { reloaded <- current.Start() }()
			<-draining
			r := httptest.NewRequest("POST", "http://site/index.php", strings.NewReader("payload"))
			ctx := context.WithValue(r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer())
			ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, *r)
			waiting := &reloadWaitingContext{Context: ctx, waiting: make(chan struct{})}
			response := httptest.NewRecorder()
			response.Header().Set("X-Existing", "preserved")
			served := make(chan error, 1)
			go func() { served <- old.modules[0].ServeHTTP(response, r.WithContext(waiting), nil) }()
			<-waiting.waiting
			release()
			require.NoError(t, <-reloaded)
			require.NoError(t, <-served)
			require.NoError(t, hookErr)
			require.Equal(t, "new:payload", response.Body.String())
			require.Equal(t, "preserved", response.Header().Get("X-Existing"))
		})
	}
}

func TestNewModuleRequestStallsUntilAppStarts(t *testing.T) {
	newApp := func(body string) *FrankenPHPApp {
		root, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		script := fmt.Sprintf("<?php $handle = function () { echo %q . file_get_contents('php://input'); }; $handle();", body)
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(script), 0600))
		app := &FrankenPHPApp{NumThreads: 1, MaxThreads: 3, ctx: context.Background(), logger: slog.New(slog.DiscardHandler), started: make(chan any)}
		module := &FrankenPHPModule{Name: "site", resolvedDocumentRoot: root, app: app}
		app.modules = []*FrankenPHPModule{module}
		return app
	}
	old, current := newApp("old:"), newApp("new:")
	t.Cleanup(func() { frankenphp.Shutdown(); activeApp.Store(nil) })
	require.NoError(t, old.Start())

	r := httptest.NewRequest("POST", "http://site/index.php", strings.NewReader("payload"))
	ctx := context.WithValue(r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer())
	ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, *r)
	response := httptest.NewRecorder()
	served := make(chan error, 1)
	go func() { served <- current.modules[0].ServeHTTP(response, r.WithContext(ctx), nil) }()

	select {
	case err := <-served:
		t.Fatalf("a request to a module whose app has not started must stall, got: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, current.Start())
	require.NoError(t, <-served)
	require.Equal(t, "new:payload", response.Body.String())
}
