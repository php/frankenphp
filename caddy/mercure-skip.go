//go:build nomercure

package caddy

type mercureContext struct {
}

func (f *FrankenPHPModule) configureHotReload(_ *FrankenPHPApp) error {
	return nil
}

func (f *FrankenPHPModule) assignMercureHub(_ caddy.Context) {
}
