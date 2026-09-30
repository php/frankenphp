//go:build nowatcher

package frankenphp

import "errors"

type hotReloadOpt struct {
}

func (*hotReloadOpt) hasHotReload() bool {
	return false
}

var errWatcherNotEnabled = errors.New("watcher support is not enabled")

func initWatchers(o *opt) error {
	for _, o := range o.workers {
		if len(o.watch) != 0 {
			return errWatcherNotEnabled
		}
	}

	return nil
}

func drainWatchers() {
}
