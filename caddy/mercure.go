//go:build !nomercure

package caddy

import (
	"encoding/json"
	"errors"
	"os"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/mercure"
	mercureCaddy "github.com/dunglas/mercure/caddy"
)

func init() {
	mercureCaddy.AllowNoPublish = true
}

type mercureContext struct {
	mercureHub *mercure.Hub
}

func (f *FrankenPHPModule) assignMercureHub(ctx caddy.Context) {
	if f.mercureHub = mercureCaddy.FindHub(ctx.Modules()); f.mercureHub == nil {
		return
	}

	f.requestOptions = append(f.requestOptions, frankenphp.WithMercureHub(f.mercureHub))

	for i, wc := range f.Workers {
		wc.mercureHub = f.mercureHub
		wc.options = append(wc.options, frankenphp.WithWorkerMercureHub(wc.mercureHub))

		f.Workers[i] = wc
	}
}

func createMercureRoute() (caddyhttp.Route, error) {
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
			mercureCaddy.Mercure{
				PublisherJWT: mercureCaddy.JWTConfig{
					Alg: os.Getenv("MERCURE_PUBLISHER_JWT_ALG"),
					Key: mercurePublisherJwtKey,
				},
				SubscriberJWT: mercureCaddy.JWTConfig{
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
