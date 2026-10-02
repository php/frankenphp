package caddy

import (
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
	select {
	case <-app.started:
	case <-r.Context().Done():
		return r.Context().Err()
	case <-time.After(10 * time.Second):
		return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
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
