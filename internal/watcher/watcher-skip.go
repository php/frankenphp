//go:build nowatcher

package watcher

import (
	"context"
	"errors"
	"log/slog"
)

func InitWatcher(ct context.Context, logger *slog.Logger, _ []*PatternGroup) error {
	err := errors.New("watcher support is not enabled")

	if logger.Enabled(ct, slog.LevelError) {
		logger.LogAttrs(ct, slog.LevelError, err.Error())
	}

	return err
}

func DrainWatcher() {
}
