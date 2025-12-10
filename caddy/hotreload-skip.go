//go:build nowatcher || nomercure
package caddy

func (_ *FrankenPHPModule) configureHotReload(_ *FrankenPHPApp) error {
	return nil
}
