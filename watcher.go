//go:build !nowatcher

package frankenphp

import (
	"sync/atomic"

	"github.com/dunglas/frankenphp/internal/watcher"
	watcherGo "github.com/e-dant/watcher/watcher-go"
)

type hotReloadOpt struct {
	hotReload   []*watcher.PatternGroup
}

var restartWorkers atomic.Bool

func initWatchers(o *opt) error {
	watchPatterns := make([]*watcher.PatternGroup, 0, len(o.hotReload))

	for _, o := range o.workers {
		if len(o.watch) == 0 {
			continue
		}

		watcherIsEnabled = true
		watchPatterns = append(watchPatterns, &watcher.PatternGroup{Patterns: o.watch, Callback: func(_ []*watcherGo.Event) {
			restartWorkers.Store(true)
		}})
	}

	if watcherIsEnabled {
		watchPatterns = append(watchPatterns, &watcher.PatternGroup{
			Callback: func(_ []*watcherGo.Event) {
				if restartWorkers.Swap(false) {
					RestartWorkers()
				}
			},
		})
	}

	return watcher.InitWatcher(globalCtx, globalLogger, append(watchPatterns, o.hotReload...))
}
