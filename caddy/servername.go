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
	if module.Name != "" {
		return module.Name
	}

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

func (f *FrankenPHPApp) httpServerFor(module *FrankenPHPModule) *caddyhttp.Server {
	if f.httpApp == nil {
		return nil
	}
	for _, srv := range f.httpApp.Servers {
		if serverContainsHandler(srv, module) {
			return srv
		}
	}
	return nil
}

// findHostInRoutes walks routes (recursing into route-wrapping handlers) to
// locate the route that contains target, then returns the first host of that
// route's host matcher. Returns "" if no enclosing route or no host matcher
// is found.
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
		for _, handler := range route.Handlers {
			if nested, ok := wrappedRoutes(handler); ok {
				if host := findHostInRoutes(nested, target); host != "" {
					return host
				}
			}
		}
	}

	return ""
}

// wrappedRoutes returns the route list h wraps, if h is a route-wrapping
// handler such as Subroute or the reload capture's route holder.
func wrappedRoutes(h caddyhttp.MiddlewareHandler) (caddyhttp.RouteList, bool) {
	switch h := h.(type) {
	case *caddyhttp.Subroute:
		return h.Routes, true
	case *reloadRoutes:
		return h.routes, true
	}

	return nil, false
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
		if nested, ok := wrappedRoutes(h); ok {
			for _, r := range nested {
				if routeContainsHandler(r, target) {
					return true
				}
			}
		}
	}

	return false
}
