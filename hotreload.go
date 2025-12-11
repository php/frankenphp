//go:build !nomercure && !nowatcher

package frankenphp

import (
	"encoding/json"
	"log/slog"

	"github.com/dunglas/frankenphp/internal/watcher"
	"github.com/dunglas/mercure"
	watcherGo "github.com/e-dant/watcher/watcher-go"
)

// WithHotReload sets files to watch for file changes to trigger a hot reload update.
func WithHotReload(topic string, hub *mercure.Hub, patterns []string) Option {
	return func(o *opt) error {
		o.hotReload = append(o.hotReload, &watcher.PatternGroup{
			Patterns: patterns,
			Callback: func(events []*watcherGo.Event) {
				// Wait for workers to restart before sending the update
				go func() {
					data, err := json.Marshal(events)
					if err != nil {
						if globalLogger.Enabled(globalCtx, slog.LevelError) {
							globalLogger.LogAttrs(globalCtx, slog.LevelError, "error marshaling watcher events", slog.Any("error", err))
						}

						return
					}

					if err := hub.Publish(globalCtx, &mercure.Update{
						Topics: []string{topic},
						Event:  mercure.Event{Data: string(data)},
						Debug:  globalLogger.Enabled(globalCtx, slog.LevelDebug),
					}); err != nil && globalLogger.Enabled(globalCtx, slog.LevelError) {
						globalLogger.LogAttrs(globalCtx, slog.LevelError, "error publishing hot reloading Mercure update", slog.Any("error", err))
					}
				}()
			},
		})

		return nil
	}
}
