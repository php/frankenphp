package frankenphp

import (
	"context"
	"log/slog"
	"time"
)

// pings are periodic internal messages sent to the worker.
// they are received via frankenphp_handle_request(fn($message) => ...).
type ping struct {
	interval time.Duration
	message  string
	aligned  bool
	each     bool
	idle     bool
}

func initPings() {
	for _, w := range workers {
		w.initPings()
	}
}

func shutdownPings() {
	for _, w := range workers {
		w.stopPings()
	}
}

func (w *worker) initPings() {
	if len(w.pings) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(globalCtx)
	w.pingCancel = cancel

	for _, ping := range w.pings {
		if ping.aligned {
			go w.startAlignedPingLoop(ctx, ping)
		} else {
			go w.startPingLoop(ctx, ping)
		}
	}
}

func (w *worker) stopPings() {
	if w.pingCancel != nil {
		w.pingCancel()
		w.pingCancel = nil
	}
}

func (w *worker) startPingLoop(ctx context.Context, ping *ping) {
	ticker := time.NewTicker(ping.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sendPings(ping)
		}
	}
}

func (w *worker) startAlignedPingLoop(ctx context.Context, ping *ping) {
	timer := time.NewTimer(time.Until(nextAlignedPing(ping.interval, time.Now())))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			w.sendPings(ping)
			timer.Reset(time.Until(nextAlignedPing(ping.interval, time.Now())))
		}
	}
}

func nextAlignedPing(interval time.Duration, now time.Time) time.Time {
	switch interval {
	case time.Minute:
		return now.Truncate(time.Minute).Add(time.Minute)
	case time.Hour:
		return now.Truncate(time.Hour).Add(time.Hour)
	default:
		return now.Truncate(interval).Add(interval)
	}
}

func (w *worker) sendPings(p *ping) {
	if p.each {
		w.sendPingToEachThread(p)
		return
	}

	w.sendPing(p)
}

func (w *worker) sendPing(p *ping) {
	fc := newContextFromMessage(p.message, nil, globalCtx, w)

	if err := w.handleRequest(fc); err != nil && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker ping failed", slog.String("worker", w.name), slog.String("message", p.message), slog.Any("error", err))
	}
}

func (w *worker) sendPingToEachThread(p *ping) {
	w.threadMutex.RLock()
	for _, thread := range w.threads {
		if p.idle && thread.state.WaitTime() < p.interval.Milliseconds() {
			// only ping the thread if it's been idle for at least the interval
			continue
		}
		go func(thread *phpThread) {
			fc := newContextFromMessage(p.message, nil, globalCtx, w)
			if err := w.handleRequestOnThread(thread, fc); err != nil && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
				globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker ping failed", slog.String("worker", w.name), slog.String("message", p.message), slog.Any("error", err))
			}
		}(thread)
	}
	w.threadMutex.RUnlock()
}
