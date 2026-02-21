//go:build nomercure

package caddy

import "github.com/caddyserver/caddy/v2"

type mercureContext struct {
}

func (f *FrankenPHPModule) assignMercureHub(_ caddy.Context) {
}
