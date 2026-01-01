//go:build !nowatcher

package watcher

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/e-dant/watcher/watcher-go"
)

const (
	// duration to wait before triggering a reload after a file change
	debounceDuration = 150 * time.Millisecond
	// times to retry watching if the watcher was closed prematurely
	maxFailureCount      = 5
	failureResetDuration = 5 * time.Second
)

var (
	ErrAlreadyStarted = errors.New("watcher is already running")

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

type PatternGroup struct {
	Patterns []string
	Callback func([]*watcher.Event)
}

type eventHolder struct {
	patternGroup *PatternGroup
	event        *watcher.Event
}

type globalWatcher struct {
	groups   []*PatternGroup
	watchers []*pattern
	excludes map[*PatternGroup][]*pattern
	events   chan eventHolder
	stop     chan struct{}
}

func InitWatcher(ct context.Context, slogger *slog.Logger, groups []*PatternGroup) error {
	if len(groups) == 0 {
		return nil
	}

	if watcherIsActive.Load() {
		return ErrAlreadyStarted
	}

	watcherIsActive.Store(true)
	globalCtx = ct
	globalLogger = slogger

	activeWatcher = &globalWatcher{groups: groups}

	for _, g := range groups {
		if len(g.Patterns) == 0 {
			continue
		}

		for _, p := range g.Patterns {
			activeWatcher.watchers = append(activeWatcher.watchers, &pattern{patternGroup: g, value: p})
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

	p.startSession()

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
	g.excludes = make(map[*PatternGroup][]*pattern)

	if err := g.parseFilePatterns(); err != nil {
		return err
	}

	for _, w := range g.watchers {
		if w.isExclude {
			g.excludes[w.patternGroup] = append(g.excludes[w.patternGroup], w)
		}
	}

	for _, w := range g.watchers {
		w.events = g.events
		w.startSession()
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

func (g *globalWatcher) isExcludedEvent(pg *PatternGroup, e *watcher.Event) bool {
	excludes := g.excludes[pg]
	for _, ex := range excludes {
		if ex.matchesEvent(e) {
			return true
		}
	}

	return false
}

func (g *globalWatcher) listenForFileEvents() {
	timer := time.NewTimer(debounceDuration)
	timer.Stop()

	eventsPerGroup := make(map[*PatternGroup][]*watcher.Event, len(g.groups))

	defer timer.Stop()
	for {
		select {
		case <-g.stop:
			return
		case eh := <-g.events:
			if g.isExcludedEvent(eh.patternGroup, eh.event) {
				continue
			}

			timer.Reset(debounceDuration)
			eventsPerGroup[eh.patternGroup] = append(eventsPerGroup[eh.patternGroup], eh.event)
		case <-timer.C:
			timer.Stop()

			if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
				var events []*watcher.Event
				for _, eventList := range eventsPerGroup {
					events = append(events, eventList...)
				}

				globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "filesystem changes detected", slog.Any("events", events))
			}

			g.scheduleReload(eventsPerGroup)
			eventsPerGroup = make(map[*PatternGroup][]*watcher.Event, len(g.groups))
		}
	}
}

func (g *globalWatcher) scheduleReload(eventsPerGroup map[*PatternGroup][]*watcher.Event) {
	reloadWaitGroup.Add(1)

	// Call callbacks in order
	for _, g := range g.groups {
		if len(g.Patterns) == 0 {
			g.Callback(nil)
		}

		if e, ok := eventsPerGroup[g]; ok {
			g.Callback(e)
		}
	}

	reloadWaitGroup.Done()
}
