package frankenphp

import (
	"context"
	"log/slog"
	"sync"
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

var (
	pingCancel chan any
	pingWg     sync.WaitGroup
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
		for _, p := range w.pings {
			if pingCancel == nil {
				pingCancel = make(chan any)
				pingWg = sync.WaitGroup{}
			}
			pingWg.Add(1)
			p.worker = w
			if p.aligned {
				go p.startAlignedLoop(globalCtx)
			} else {
				go p.startLoop(globalCtx)
			}
		}
	}
}

func shutdownPings() {
	if pingCancel == nil {
		return
	}
	close(pingCancel)
	pingWg.Wait()
	pingCancel = nil
}

func (p *ping) startLoop(ctx context.Context) {
	interval := p.interval
	if p.mode == PingModeIdle {
		// reduce the interval when pinging for idle threads
		// this way threads will be idle for at most 4/3 of the original interval
		interval = p.interval / 3
	}
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-pingCancel:
			ticker.Stop()
			pingWg.Done()
			return
		case <-ticker.C:
			p.send(ctx)
		}
	}
}

func (p *ping) startAlignedLoop(ctx context.Context) {
	timer := time.NewTimer(time.Until(nextAlignedPing(p.interval, time.Now())))

	for {
		select {
		case <-pingCancel:
			timer.Stop()
			pingWg.Done()
			return
		case <-timer.C:
			p.send(ctx)
			timer.Reset(time.Until(nextAlignedPing(p.interval, time.Now())))
		}
	}
}

// nextAlignedPing returns the next time that is a multiple of the given interval
// e.g. interval=15m aligns to :00, :15, :30, :45
func nextAlignedPing(interval time.Duration, now time.Time) time.Time {
	var periodStart time.Time
	switch {
	case interval <= time.Minute:
		periodStart = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	case interval <= time.Hour:
		periodStart = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	case interval <= 24*time.Hour:
		periodStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	default:
		// multi-day intervals: fall back to epoch-based truncation
		return now.Truncate(interval).Add(interval)
	}

	return periodStart.Add((now.Sub(periodStart)/interval + 1) * interval)
}

func (p *ping) send(ctx context.Context) {
	switch p.mode {
	case PingModeEach, PingModeIdle:
		p.sendToEachThread(ctx)
	case PingModeOverlapping:
		go p.sendOnce(ctx)
	case PingModeSynchronous:
		p.sendOnce(ctx)
	}
}

func (p *ping) sendOnce(ctx context.Context) {
	pingWg.Add(1)
	fc := newContextFromMessage(p.message, nil, ctx, p.worker)

	if err := p.worker.handleRequest(fc); err != nil && globalLogger.Enabled(ctx, slog.LevelWarn) {
		globalLogger.LogAttrs(ctx, slog.LevelWarn, "worker ping failed", slog.String("worker", p.worker.name), slog.String("message", p.message), slog.Any("error", err))
	}
	pingWg.Done()
}

func (p *ping) sendToEachThread(ctx context.Context) {
	w := p.worker
	w.threadMutex.RLock()
	for _, thread := range w.threads {
		if p.mode == PingModeIdle && thread.state.WaitTime() < p.interval.Milliseconds() {
			continue
		}
		pingWg.Add(1)
		go func(thread *phpThread) {
			fc := newContextFromMessage(p.message, nil, ctx, w)
			if err := w.handleRequestOnThread(thread, fc); err != nil && globalLogger.Enabled(ctx, slog.LevelWarn) {
				globalLogger.LogAttrs(ctx, slog.LevelWarn, "worker ping failed", slog.String("worker", w.name), slog.String("message", p.message), slog.Any("error", err))
			}
			pingWg.Done()
		}(thread)
	}
	w.threadMutex.RUnlock()
}
