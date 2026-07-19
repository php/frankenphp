package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/stretchr/testify/require"
)

func TestResolveServerName(t *testing.T) {
	moduleWithHost := &FrankenPHPModule{}
	moduleWithoutHost := &FrankenPHPModule{}
	moduleInSubroute := &FrankenPHPModule{}
	unroutedModule := &FrankenPHPModule{}

	host := caddyhttp.MatchHost{"api.example.com", "www.example.com"}

	app := &FrankenPHPApp{
		httpApp: &caddyhttp.App{
			Servers: map[string]*caddyhttp.Server{
				"srv0": {
					Listen: []string{":8080"},
					Routes: caddyhttp.RouteList{
						{
							MatcherSets: caddyhttp.MatcherSets{{&host}},
							Handlers:    []caddyhttp.MiddlewareHandler{moduleWithHost},
						},
						{
							Handlers: []caddyhttp.MiddlewareHandler{moduleWithoutHost},
						},
						{
							Handlers: []caddyhttp.MiddlewareHandler{
								&caddyhttp.Subroute{
									Routes: caddyhttp.RouteList{
										{Handlers: []caddyhttp.MiddlewareHandler{moduleInSubroute}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// first host of the enclosing route's host matcher wins
	require.Equal(t, "api.example.com", app.resolveServerName(moduleWithHost))

	// no host matcher: fall back to the first listener address
	require.Equal(t, ":8080", app.resolveServerName(moduleWithoutHost))

	// modules nested in subroutes are found too
	require.Equal(t, ":8080", app.resolveServerName(moduleInSubroute))

	// module not present in any route tree
	require.Equal(t, "", app.resolveServerName(unroutedModule))

	// no http app configured
	require.Equal(t, "", (&FrankenPHPApp{}).resolveServerName(moduleWithHost))
}
