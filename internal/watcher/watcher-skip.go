//go:build nowatcher

package watcher

import (
	"context"
	"errors"
	"log/slog"
)

var errWatcherNotEnabled = errors.New("watcher support is not enabled")

func InitWatcher(_ context.Context, _ *slog.Logger, _ []*PatternGroup) error {
	return errWatcherNotEnabled
}

func DrainWatcher() {
}
