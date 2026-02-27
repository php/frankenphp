//go:build nomercure

package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type mercureContext struct {
}

func (f *FrankenPHPModule) assignMercureHub(_ caddy.Context) {
}

func createMercureRoute() (caddyhttp.Route, error) {
	return caddyhttp.Route{}, nil
}
