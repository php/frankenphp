//go:build nowatcher || nomercure

package caddy

import "errors"

type hotReloadContext struct {
}

func (_ *FrankenPHPModule) configureHotReload(_ *FrankenPHPApp) error {
	return nil
}

func (_ *FrankenPHPModule) unmarshalHotReload(d *caddyfile.Dispenser) error {
	return errors.New("hot reload support disabled")
}
