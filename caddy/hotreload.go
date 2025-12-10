//go:build !nowatcher && !nomercure
package caddy

import (
	"errors"
	"net/url"

	"github.com/dunglas/frankenphp"
)

func (f *FrankenPHPModule) configureHotReload(app *FrankenPHPApp) error {
	if len(f.HotReload) == 0 {
		return nil
	}

	if f.mercureHub == nil {
		return errors.New("unable to enable hot reloading: no Mercure hub configured")
	}

	app.opts = append(app.opts, frankenphp.WithHotReload(f.Name, f.mercureHub, f.HotReload))
	f.preparedEnv["FRANKENPHP_HOT_RELOAD\x00"] = "/.well-known/mercure?topic=https%3A%2F%2Ffrankenphp.dev%2Fhot-reload%2F" + url.QueryEscape(f.Name)

	return nil
}
