//go:build nomercure

package caddy

import (
	"github.com/caddyserver/caddy/v2"
)

func (f *FrankenPHPModule) assignMercureHubRequestOption(_ caddy.Context) {
}

type mercureContext struct {
}

func (wc *workerConfig) appendMercureHubOption(opts []frankenphp.WorkerOption) []frankenphp.WorkerOption {
	return opts
}
