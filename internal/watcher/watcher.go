//go:build !nowatcher

package watcher

// #include <stdint.h>
// #include <stdlib.h>
// #include "watcher.h"
import "C"
import (
	"context"
	"errors"
	"log/slog"
	"runtime/cgo"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type watcher struct {
	sessions []C.uintptr_t
	callback func()
	trigger  chan string
	stop     chan struct{}
}

// duration to wait before triggering a reload after a file change
const debounceDuration = 150 * time.Millisecond

// times to retry watching if the watcher was closed prematurely
const maxFailureCount = 5
const failureResetDuration = 5 * time.Second

var failureMu = sync.Mutex{}
var watcherIsActive = atomic.Bool{}

var (
	ErrAlreadyStarted        = errors.New("the watcher is already running")
	ErrUnableToStartWatching = errors.New("unable to start the watcher")

	// the currently active file watcher
	activeWatcher *watcher
	// after stopping the watcher we will wait for eventual reloads to finish
	reloadWaitGroup sync.WaitGroup
	// we are passing the context from the main package to the watcher
	ctx context.Context
	// we are passing the logger from the main package to the watcher
	logger *slog.Logger
)

func InitWatcher(ct context.Context, filePatterns []string, callback func(), slogger *slog.Logger) error {
	if len(filePatterns) == 0 {
		return nil
	}

	if watcherIsActive.Load() {
		return ErrAlreadyStarted
	}

	watcherIsActive.Store(true)
	ctx = ct
	logger = slogger
	activeWatcher = &watcher{callback: callback}

	if 	err := activeWatcher.startWatching(ctx, filePatterns); err != nil {
		return err
	}

	return nil
}

func DrainWatcher() {
	if !watcherIsActive.Load() {
		return
	}

	watcherIsActive.Store(false)

	if logger.Enabled(ctx, slog.LevelDebug) {
		logger.LogAttrs(ctx, slog.LevelDebug, "stopping watcher")
	}

	activeWatcher.stopWatching()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

// TODO: how to test this?
func retryWatching(watchPattern *watchPattern) {
	failureMu.Lock()
	defer failureMu.Unlock()
	if watchPattern.failureCount >= maxFailureCount {
		if logger.Enabled(ctx, slog.LevelWarn) {
			logger.LogAttrs(ctx, slog.LevelWarn, "giving up watching", slog.String("dir", watchPattern.dir))
		}

		return
	}

	if logger.Enabled(ctx, slog.LevelInfo) {
		logger.LogAttrs(ctx, slog.LevelInfo, "watcher was closed prematurely, retrying...", slog.String("dir", watchPattern.dir))
	}

	watchPattern.failureCount++
	session, err := startSession(watchPattern)
	if err != nil {
		activeWatcher.sessions = append(activeWatcher.sessions, session)
	}

	// reset the failure-count if the watcher hasn't reached max failures after 5 seconds
	go func() {
		time.Sleep(failureResetDuration * time.Second)
		failureMu.Lock()
		if watchPattern.failureCount < maxFailureCount {
			watchPattern.failureCount = 0
		}
		failureMu.Unlock()
	}()
}

func (w *watcher) startWatching(ctx context.Context, filePatterns []string) error {
	w.trigger = make(chan string)
	w.stop = make(chan struct{})
	w.sessions = make([]C.uintptr_t, len(filePatterns))
	watchPatterns, err := parseFilePatterns(filePatterns)
	if err != nil {
		return err
	}
	for i, watchPattern := range watchPatterns {
		watchPattern.trigger = w.trigger
		session, err := startSession(watchPattern)
		if err != nil {
			return err
		}
		w.sessions[i] = session
	}
	go listenForFileEvents(w.trigger, w.stop)
	return nil
}

func (w *watcher) stopWatching() {
	close(w.stop)
	for _, session := range w.sessions {
		stopSession(session)
	}
}

func startSession(w *watchPattern) (C.uintptr_t, error) {
	handle := cgo.NewHandle(w)
	cDir := C.CString(w.dir)
	defer C.free(unsafe.Pointer(cDir))
	watchSession := C.start_new_watcher(cDir, C.uintptr_t(handle))
	if watchSession != 0 {
		if logger.Enabled(ctx, slog.LevelDebug) {
			logger.LogAttrs(ctx, slog.LevelDebug, "watching", slog.String("dir", w.dir), slog.Any("patterns", w.patterns))
		}

		return watchSession, nil
	}

	if logger.Enabled(ctx, slog.LevelError) {
		logger.LogAttrs(ctx, slog.LevelError, "couldn't start watching", slog.String("dir", w.dir))
	}

	return watchSession, ErrUnableToStartWatching
}

func stopSession(session C.uintptr_t) {
	success := C.stop_watcher(session)
	if success == 0 && logger.Enabled(ctx, slog.LevelWarn) {
		logger.LogAttrs(ctx, slog.LevelWarn, "couldn't close the watcher")
	}
}

//export go_handle_file_watcher_event
func go_handle_file_watcher_event(path *C.char, associatedPath *C.char, eventType C.int, pathType C.int, handle C.uintptr_t) {
	watchPattern := cgo.Handle(handle).Value().(*watchPattern)
	handleWatcherEvent(watchPattern, C.GoString(path), C.GoString(associatedPath), int(eventType), int(pathType))
}

func handleWatcherEvent(watchPattern *watchPattern, path string, associatedPath string, eventType int, pathType int) {
	// If the watcher prematurely sends the die@ event, retry watching
	if pathType == 4 && strings.HasPrefix(path, "e/self/die@") && watcherIsActive.Load() {
		retryWatching(watchPattern)
		return
	}

	if watchPattern.allowReload(path, eventType, pathType) {
		watchPattern.trigger <- path
		return
	}

	// some editors create temporary files and never actually modify the original file
	// so we need to also check the associated path of an event
	// see https://github.com/php/frankenphp/issues/1375
	if associatedPath != "" && watchPattern.allowReload(associatedPath, eventType, pathType) {
		watchPattern.trigger <- associatedPath
	}
}

func listenForFileEvents(triggerWatcher chan string, stopWatcher chan struct{}) {
	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	lastChangedFile := ""
	defer timer.Stop()
	for {
		select {
		case <-stopWatcher:
		case lastChangedFile = <-triggerWatcher:
			timer.Reset(debounceDuration)
		case <-timer.C:
			timer.Stop()

			if logger.Enabled(ctx, slog.LevelInfo) {
				logger.LogAttrs(ctx, slog.LevelInfo, "filesystem change detected", slog.String("file", lastChangedFile))
			}

			scheduleReload()
		}
	}
}

func scheduleReload() {
	reloadWaitGroup.Add(1)
	activeWatcher.callback()
	reloadWaitGroup.Done()
}
