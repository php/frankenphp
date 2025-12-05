//go:build !nomercure

package caddy

import (
	"errors"
	"net/url"

	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/mercure"
	mercureCaddy "github.com/dunglas/mercure/caddy"
)

func init() {
	mercureCaddy.AllowNoPublish = true
}

type mercureContext struct {
	mercureHub *mercure.Hub
}

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

func (f *FrankenPHPModule) assignMercureHub(ctx caddy.Context) {
	if f.mercureHub = mercureCaddy.FindHub(ctx.Modules()); f.mercureHub == nil {
		return
	}

	opt := frankenphp.WithMercureHub(f.mercureHub)
	f.mercureHubRequestOption = &opt

	for i, wc := range f.Workers {
		wc.mercureHub = f.mercureHub
		wc.options = append(wc.options, frankenphp.WithWorkerMercureHub(wc.mercureHub))

		f.Workers[i] = wc
	}
}
