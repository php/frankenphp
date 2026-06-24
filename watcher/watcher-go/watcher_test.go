package watcher_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/e-dant/watcher/watcher-go"
)

func Example() {
	w := watcher.NewWatcher("/path/to/dir", func(e *watcher.Event) {
		slog.Info("filesystem event", "event", e)
	})
	defer w.Close()

	// Wait for a new line to exit
	_, _ = fmt.Scanln()
}

func TestWatcher(t *testing.T) {
	dir := t.TempDir()
	modifyEvent := make(chan *watcher.Event)
	ready := make(chan struct{})

	w := watcher.NewWatcher(dir, func(e *watcher.Event) {
		if strings.HasPrefix(e.PathName, "s/self/live@") {
			ready <- struct{}{}

			return
		}

		if e.EffectType == watcher.EffectTypeModify && strings.HasSuffix(e.PathName, "test.txt") {
			modifyEvent <- e
		}
	})
	t.Cleanup(w.Close)

	// Wait for the watcher to be fully started
	<-ready

	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	<-modifyEvent
}
