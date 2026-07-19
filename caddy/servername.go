package caddy

import (
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// resolveServerName picks a stable, human-friendly name for the php_server
// block represented by module, so workers, metrics and logs can be attributed
// to a block (e.g. "api.example.com") instead of an opaque index. Cascade:
//  1. First host of the enclosing route's host matcher.
//  2. First listener address of the http server containing the module.
//
// Returns "" when the http app is not available or the module cannot be
// located in any route tree.
func (f *FrankenPHPApp) resolveServerName(module *FrankenPHPModule) string {
	if f.httpApp == nil {
		return ""
	}

	for _, srv := range f.httpApp.Servers {
		if !serverContainsHandler(srv, module) {
			continue
		}
		if h := findHostInRoutes(srv.Routes, module); h != "" {
			return h
		}
		if len(srv.Listen) > 0 {
			return srv.Listen[0]
		}
	}

	return ""
}

// findHostInRoutes walks routes (recursing into Subroute handlers) to locate
// the route that contains target, then returns the first host of that route's
// host matcher. Returns "" if no enclosing route or no host matcher is found.
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

func serverContainsHandler(srv *caddyhttp.Server, target caddyhttp.MiddlewareHandler) bool {
	for _, route := range srv.Routes {
		if routeContainsHandler(route, target) {
			return true
		}
	}

	return false
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
