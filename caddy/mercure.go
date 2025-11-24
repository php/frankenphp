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
	hub := mercureCaddy.FindHub(ctx.Modules())
	if hub == nil {
		return
	}

	opt := frankenphp.WithMercureHub(hub)
	f.mercureHubRequestOption = &opt

	for i, wc := range f.Workers {
		wc.mercureHub = hub

		f.Workers[i] = wc
	}
}

func (wc *workerConfig) appendMercureHubOption(opts []frankenphp.WorkerOption) []frankenphp.WorkerOption {
	if wc.mercureHub != nil {
		opts = append(opts, frankenphp.WithWorkerMercureHub(wc.mercureHub))
	}

	return opts
}
