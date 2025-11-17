//go:build !nomercure

package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
	mercureCaddy "github.com/dunglas/mercure/caddy"
)

func init() {
	mercureCaddy.AllowNoPublish = true
}

func (f *FrankenPHPModule) assignMercureHubRequestOption(ctx caddy.Context) {
	if hub := mercureCaddy.FindHub(ctx.Modules()); hub != nil {
		opt := frankenphp.WithMercureHub(hub)
		f.mercureHubRequestOption = &opt
	}
}
