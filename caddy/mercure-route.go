//go:build !nomercure

package caddy

import (
	"encoding/json"
	"errors"
	"os"

	mercureModule "github.com/dunglas/mercure/caddy"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func addMercureRoute() (caddyhttp.Route, error) {
	mercurePublisherJwtKey := os.Getenv("MERCURE_PUBLISHER_JWT_KEY")
	if mercurePublisherJwtKey == "" {
		return caddyhttp.Route{}, errors.New(`The "MERCURE_PUBLISHER_JWT_KEY" environment variable must be set to use the Mercure.rocks hub`)
	}

	mercureSubscriberJwtKey := os.Getenv("MERCURE_SUBSCRIBER_JWT_KEY")
	if mercureSubscriberJwtKey == "" {
		return caddyhttp.Route{}, errors.New(`The "MERCURE_SUBSCRIBER_JWT_KEY" environment variable must be set to use the Mercure.rocks hub`)
	}

	mercureRoute := caddyhttp.Route{
		HandlersRaw: []json.RawMessage{caddyconfig.JSONModuleObject(
			mercureModule.Mercure{
				PublisherJWT: mercureModule.JWTConfig{
					Alg: os.Getenv("MERCURE_PUBLISHER_JWT_ALG"),
					Key: mercurePublisherJwtKey,
				},
				SubscriberJWT: mercureModule.JWTConfig{
					Alg: os.Getenv("MERCURE_SUBSCRIBER_JWT_ALG"),
					Key: mercureSubscriberJwtKey,
				},
			},
			"handler",
			"mercure",
			nil,
		),
		},
	}

	return mercureRoute, nil;
}
