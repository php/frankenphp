package caddy

import (
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// resolveScopeLabel picks a stable, human-friendly identifier for this
// module's background-worker scope so metric/log emitters can render it
// (e.g. server="api.example.com") instead of the opaque numeric id.
// Cascade:
//  1. First host of the route's host matcher.
//  2. First listener address of the server.
func (f *FrankenPHPModule) resolveScopeLabel(srv *caddyhttp.Server) string {
	if h := findHostInRoutes(srv.Routes, f); h != "" {
		return h
	}
	if len(srv.Listen) > 0 {
		return srv.Listen[0]
	}
	return ""
}

// findHostInRoutes walks routes (recursing into Subroute handlers) to
// locate the route that contains target, then returns the first host of
// that route's host matcher. Returns "" if no enclosing route or no host
// matcher is found.
func findHostInRoutes(routes caddyhttp.RouteList, target caddyhttp.MiddlewareHandler) string {
	for _, route := range routes {
		if !routeContainsHandler(route, target) {
			continue
		}
		for _, mset := range route.MatcherSets {
			for _, m := range mset {
				hp, ok := m.(*caddyhttp.MatchHost)
				if !ok || hp == nil || len(*hp) == 0 {
					continue
				}
				return (*hp)[0]
			}
		}
	}
	return ""
}

func routeContainsHandler(route caddyhttp.Route, target caddyhttp.MiddlewareHandler) bool {
	for _, h := range route.Handlers {
		if h == target {
			return true
		}
		if sub, ok := h.(*caddyhttp.Subroute); ok {
			for _, r := range sub.Routes {
				if routeContainsHandler(r, target) {
					return true
				}
			}
		}
	}
	return false
}
