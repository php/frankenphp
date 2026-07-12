package frankenphp

import (
	"context"
	"log/slog"
	"time"
)

type PingMode int

const (
	// PingModeSynchronous sends the ping to the worker and waits for completion before sending the next ping
	PingModeSynchronous PingMode = iota
	// PingModeOverlapping sends the ping to the worker without waiting for completion
	PingModeOverlapping
	// PingModeEach sends the ping to each active worker thread without waiting for completion
	PingModeEach
	// PingModeIdle sends the ping to each thread that has been idle for at least the interval
	PingModeIdle
)

// pings are periodic internal messages sent to the worker.
// they are received via frankenphp_handle_request(fn(string $message) => ...).
type ping struct {
	interval time.Duration
	message  string
	aligned  bool
	mode     PingMode
	worker   *worker
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

	for _, p := range w.pings {
		p.worker = w
		if p.aligned {
			go p.startAlignedLoop(ctx)
		} else {
			go p.startLoop(ctx)
		}
	}
}

func (w *worker) stopPings() {
	if w.pingCancel != nil {
		w.pingCancel()
		w.pingCancel = nil
	}
}

func (p *ping) startLoop(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.send()
		}
	}
}

func (p *ping) startAlignedLoop(ctx context.Context) {
	timer := time.NewTimer(time.Until(nextAlignedPing(p.interval, time.Now())))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			p.send()
			timer.Reset(time.Until(nextAlignedPing(p.interval, time.Now())))
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

func (p *ping) send() {
	switch p.mode {
	case PingModeEach, PingModeIdle:
		p.sendToEachThread()
	case PingModeOverlapping:
		go p.sendOnce()
	case PingModeSynchronous:
		p.sendOnce()
	}
}

func (p *ping) sendOnce() {
	fc := newContextFromMessage(p.message, nil, globalCtx, p.worker)

	if err := p.worker.handleRequest(fc); err != nil && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker ping failed", slog.String("worker", p.worker.name), slog.String("message", p.message), slog.Any("error", err))
	}
}

func (p *ping) sendToEachThread() {
	w := p.worker
	w.threadMutex.RLock()
	for _, thread := range w.threads {
		if p.mode == PingModeIdle && thread.state.WaitTime() < p.interval.Milliseconds() {
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
