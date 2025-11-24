//go:build nowatcher

package watcher

import (
	"context"
	"log/slog"
)

func InitWatcher(ct context.Context, slogger *slog.Logger, _ []*PatternGroup, _ func()) error {
	err := errors.New("globalWatcher support is not enabled")

	if logger.Enabled(ct, slog.LevelError) {
		logger.LogAttrs(ct, slog.LevelError, err)
	}

	return err
}

func DrainWatcher() {
}
