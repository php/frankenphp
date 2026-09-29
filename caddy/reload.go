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
	for _, m := range app.modules {
		if m.server == nil || f.server == nil || m.server.Name() != f.server.Name() {
			continue
		}
		select {
		case <-m.app.started:
		case <-r.Context().Done():
			return r.Context().Err()
		case <-time.After(10 * time.Second):
			return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
		}
		if !m.app.hasStarted.Load() {
			return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
		}
		return m.ServeHTTP(w, r, nil)
	}
	return caddyhttp.Error(http.StatusServiceUnavailable, frankenphp.ErrNotRunning)
}
