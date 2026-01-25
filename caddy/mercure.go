//go:build !nomercure

package caddy

import (
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
