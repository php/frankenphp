//go:build !nowatcher

package frankenphp

import (
	"sync/atomic"

	"github.com/dunglas/frankenphp/internal/watcher"
	watcherGo "github.com/e-dant/watcher/watcher-go"
)

type hotReloadOpt struct {
	hotReload []*watcher.PatternGroup
}

var restartWorkers atomic.Bool

// validateWatchers has nothing to refuse where the watcher is compiled in:
// the patterns are only known to be good once the watcher took them
func validateWatchers(*opt) error {
	return nil
}

func (o *hotReloadOpt) hasHotReload() bool {
	return len(o.hotReload) != 0
}

func initWatchers(o *opt, mThread *phpMainThread) error {
	watchPatterns := make([]*watcher.PatternGroup, 0, len(o.hotReload))

	for _, o := range o.workers {
		if len(o.watch) == 0 {
			continue
		}

		mThread.watcherIsEnabled = true
		watchPatterns = append(watchPatterns, &watcher.PatternGroup{Patterns: o.watch, Callback: func(_ []*watcherGo.Event) {
			restartWorkers.Store(true)
		}})
	}

	if mThread.watcherIsEnabled {
		watchPatterns = append(watchPatterns, &watcher.PatternGroup{
			Callback: func(_ []*watcherGo.Event) {
				if restartWorkers.Swap(false) {
					mainThread.rebootAllThreads()
				}
			},
		})
	}

	return watcher.InitWatcher(globalCtx, globalLogger, append(watchPatterns, o.hotReload...))
}

func drainWatchers() {
	watcher.DrainWatcher()
}
