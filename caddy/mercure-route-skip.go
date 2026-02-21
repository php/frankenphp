//go:build nomercure

package caddy

import "github.com/caddyserver/caddy/v2/modules/caddyhttp"

func addMercureRoute() (caddyhttp.Route, error) {
	// no-op
	return caddyhttp.Route{}, nil
}
