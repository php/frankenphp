//go:build nomercure

package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
)

func (f *FrankenPHPModule) assignHotReload(_ *FrankenPHPApp) {
}

func (f *FrankenPHPModule) assignMercureHub(_ caddy.Context) {
}

type mercureContext struct {
}

type hotReload struct {
}

func (wc *workerConfig) appendMercureHubOption(opts []frankenphp.WorkerOption) []frankenphp.WorkerOption {
	return opts
}

func (f *FrankenPHPModule) configureHotReload(_ *FrankenPHPApp) error {
	return nil
}
