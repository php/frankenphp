//go:build !nowatcher

package watcher

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// duration to wait before triggering a reload after a file change
	debounceDuration = 150 * time.Millisecond
	// times to retry watching if the watcher was closed prematurely
	maxFailureCount      = 5
	failureResetDuration = 5 * time.Second
)

var (
	ErrAlreadyStarted        = errors.New("watcher is already running")
	ErrUnableToStartWatching = errors.New("unable to start watcher")

	failureMu       sync.Mutex
	watcherIsActive atomic.Bool

	// the currently active file watcher
	activeWatcher *globalWatcher
	// after stopping the watcher we will wait for eventual reloads to finish
	reloadWaitGroup sync.WaitGroup
	// we are passing the context from the main package to the watcher
	globalCtx context.Context
	// we are passing the globalLogger from the main package to the watcher
	globalLogger *slog.Logger
)

type eventHolder struct {
	patternGroup *patternGroup
	event        *Event
}

type globalWatcher struct {
	watchers       []*pattern
	events         chan eventHolder
	stop           chan struct{}
	globalCallback func()
}

func InitWatcher(ct context.Context, slogger *slog.Logger, groups []*PatternGroup, globalCallback func()) error {
	if len(groups) == 0 {
		return nil
	}

	if watcherIsActive.Load() {
		return ErrAlreadyStarted
	}

	watcherIsActive.Store(true)
	globalCtx = ct
	globalLogger = slogger

	activeWatcher = &globalWatcher{globalCallback: globalCallback}

	for _, g := range groups {
		pg := &patternGroup{callback: g.Callback}
		for _, p := range g.Patterns {
			activeWatcher.watchers = append(activeWatcher.watchers, &pattern{patternGroup: pg, value: p})
		}
	}

	if err := activeWatcher.startWatching(); err != nil {
		return err
	}

	return nil
}

func DrainWatcher() {
	if !watcherIsActive.Load() {
		return
	}

	watcherIsActive.Store(false)

	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "stopping watcher")
	}

	activeWatcher.stopWatching()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

// TODO: how to test this?
func (p *pattern) retryWatching() {
	failureMu.Lock()
	defer failureMu.Unlock()

	if p.failureCount >= maxFailureCount {
		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "giving up watching", slog.String("pattern", p.value))
		}

		return
	}

	if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
		globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "watcher was closed prematurely, retrying...", slog.String("pattern", p.value))
	}

	p.failureCount++

	if err := p.startSession(); err != nil && globalLogger.Enabled(globalCtx, slog.LevelError) {
		globalLogger.LogAttrs(globalCtx, slog.LevelError, "unable to start watcher", slog.String("pattern", p.value), slog.Any("error", err))
	}

	// reset the failure-count if the watcher hasn't reached max failures after 5 seconds
	go func() {
		time.Sleep(failureResetDuration)

		failureMu.Lock()
		if p.failureCount < maxFailureCount {
			p.failureCount = 0
		}
		failureMu.Unlock()
	}()
}

func (g *globalWatcher) startWatching() error {
	g.events = make(chan eventHolder)
	g.stop = make(chan struct{})

	if err := g.parseFilePatterns(); err != nil {
		return err
	}

	for i, w := range g.watchers {
		w.events = g.events
		if err := w.startSession(); err != nil {
			for j := 0; j < i; j++ {
				g.watchers[j].stop()
			}

			return err
		}
	}

	go g.listenForFileEvents()

	return nil
}

func (g *globalWatcher) parseFilePatterns() error {
	for _, w := range g.watchers {
		if err := w.parse(); err != nil {
			return err
		}
	}

	return nil
}

func (g *globalWatcher) stopWatching() {
	close(g.stop)
	for _, w := range g.watchers {
		w.stop()
	}
}

func (g *globalWatcher) listenForFileEvents() {
	timer := time.NewTimer(debounceDuration)
	timer.Stop()

	eventsPerGroup := make(map[*patternGroup][]*Event)

	defer timer.Stop()
	for {
		select {
		case <-g.stop:
			return
		case eh := <-g.events:
			timer.Reset(debounceDuration)

			eventsPerGroup[eh.patternGroup] = append(eventsPerGroup[eh.patternGroup], eh.event)
		case <-timer.C:
			timer.Stop()

			if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
				var events []*Event
				for _, eventList := range eventsPerGroup {
					events = append(events, eventList...)
				}

				globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "filesystem changes detected", slog.Any("events", events))
			}

			scheduleReload(eventsPerGroup)
			eventsPerGroup = make(map[*patternGroup][]*Event)
		}
	}
}

func scheduleReload(eventsPerGroup map[*patternGroup][]*Event) {
	reloadWaitGroup.Add(1)

	// The global callback must be called first to prevent a race condition:
	// we need to be sure that the worker restarted before the Mercure events are sent
	activeWatcher.globalCallback()

	for group, events := range eventsPerGroup {
		group.callback(events)
	}

	reloadWaitGroup.Done()
}
