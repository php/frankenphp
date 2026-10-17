//go:build nowatcher

package frankenphp

import "errors"

type hotReloadOpt struct {
}

func (*hotReloadOpt) hasHotReload() bool {
	return false
}

var errWatcherNotEnabled = errors.New("watcher support is not enabled")

// validateWatchers reports what initWatchers() would refuse, so Validate()
// refuses it too, before Start() stops the configuration in place
func validateWatchers(o *opt) error {
	for _, o := range o.workers {
		if len(o.watch) != 0 {
			return errWatcherNotEnabled
		}
	}

	return nil
}

func initWatchers(o *opt, mThread *phpMainThread) error {
	return validateWatchers(o)
}

func drainWatchers() {
}
