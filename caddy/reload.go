package caddy

import (
	"context"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
)

// requestReload dispatches r on the new runtime. Safe because the request
// never started executing: its body is unread and no response was written.
func (f *FrankenPHPModule) requestReload(w http.ResponseWriter, r *http.Request) error {
	app := activeApp.Load()
	if app == nil {
		return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
	}
	if err := app.waitForStartup(r.Context()); err != nil {
		return err
	}
	if !app.hasStarted.Load() || f.server == nil {
		return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
	}

	// Runtime-generated server_N names depend on registration order across reloads.
	name := f.reloadName
	if old := f.app.reloadModule(name); old == nil || old.server != f.server {
		return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
	}
	if m := app.reloadModule(name); m != nil {
		return m.ServeHTTP(w, r, nil)
	}
	return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
}

func (f *FrankenPHPApp) waitForStartup(ctx context.Context) error {
	select {
	case <-f.started:
		return nil
	default:
	}

	wait := f.MaxWaitTime
	if f.httpApp != nil {
		if grace := time.Duration(f.httpApp.GracePeriod); grace > 0 && (wait == 0 || grace < wait) {
			wait = grace
		}
	}
	var expired <-chan time.Time
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		expired = timer.C
	}
	select {
	case <-f.started:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-f.ctx.Done():
		return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
	case <-expired:
		return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrMaxWaitTimeExceeded)
	}
}

func (f *FrankenPHPApp) reloadModule(name string) *FrankenPHPModule {
	var target *FrankenPHPModule
	for _, m := range f.modules {
		if m.server == nil || m.reloadName != name {
			continue
		}
		// A php_server may embed several handlers sharing one server.
		if target != nil && target.server != m.server {
			return nil
		}
		if target == nil {
			target = m
		}
	}
	return target
}
