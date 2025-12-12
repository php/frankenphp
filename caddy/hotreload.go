//go:build !nowatcher && !nomercure

package caddy

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp"
)

const defaultHotReloadPattern = "./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}"

type hotReloadContext struct {
	// HotReload specifies files to watch for file changes to trigger hot reloads updates. Supports the glob syntax.
	HotReload *hotReloadConfig `json:"hot_reload,omitempty"`
}

type hotReloadConfig struct {
	Topic string   `json:"topic"`
	Watch []string `json:"watch"`
}

func (f *FrankenPHPModule) configureHotReload(app *FrankenPHPApp) error {
	if f.HotReload == nil {
		return nil
	}

	if f.mercureHub == nil {
		return errors.New("unable to enable hot reloading: no Mercure hub configured")
	}

	if len(f.HotReload.Watch) == 0 {
		f.HotReload.Watch = []string{defaultHotReloadPattern}
	}

	if f.HotReload.Topic == "" {
		uid, err := uniqueID(f)
		if err != nil {
			return err
		}

		f.HotReload.Topic = "https://frankenphp.dev/hot-reload/" + uid
	}

	app.opts = append(app.opts, frankenphp.WithHotReload(f.HotReload.Topic, f.mercureHub, f.HotReload.Watch))
	f.preparedEnv["FRANKENPHP_HOT_RELOAD\x00"] = "/.well-known/mercure?topic=" + url.QueryEscape(f.HotReload.Topic)

	return nil
}

func (f *FrankenPHPModule) unmarshalHotReload(d *caddyfile.Dispenser) error {
	patterns := d.RemainingArgs()
	if len(patterns) > 0 {
		f.HotReload = &hotReloadConfig{
			Watch: patterns,
		}
	}

	for d.NextBlock(1) {
		switch v := d.Val(); v {
		case "topic":
			if !d.NextArg() {
				return d.ArgErr()
			}

			if f.HotReload == nil {
				f.HotReload = &hotReloadConfig{}
			}

			f.HotReload.Topic = d.Val()

		case "watch":
			patterns := d.RemainingArgs()
			if len(patterns) == 0 {
				return d.ArgErr()
			}

			if f.HotReload == nil {
				f.HotReload = &hotReloadConfig{}
			}

			f.HotReload.Watch = append(f.HotReload.Watch, patterns...)

		default:
			return wrongSubDirectiveError("hot_reload", "topic, watch", v)
		}
	}

	return nil
}

func uniqueID(s any) (string, error) {
	var b bytes.Buffer

	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return "", fmt.Errorf("unable to generate unique name: %w", err)
	}

	h := fnv.New64a()
	if _, err := h.Write(b.Bytes()); err != nil {
		return "", fmt.Errorf("unable to generate unique name: %w", err)
	}

	return fmt.Sprintf("%016x", h.Sum64()), nil
}
