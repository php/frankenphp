package frankenphp

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type TickMode int

const (
	// TickModeSynchronous sends the tick to the worker and waits for completion before sending the next tick
	TickModeSynchronous TickMode = iota
	// TickModeOverlapping sends the tick to the worker without waiting for completion
	TickModeOverlapping
	// TickModeEach sends the tick to each active worker thread without waiting for completion
	TickModeEach
	// TickModeIdle sends the tick to each thread that has been idle for at least the interval
	TickModeIdle
)

var (
	tickCancel chan any
	tickWg     sync.WaitGroup
)

// ticks are periodic internal messages sent to the worker.
// they are received via frankenphp_handle_request(fn(string $message) => ...).
type tick struct {
	interval time.Duration
	message  string
	aligned  bool
	mode     TickMode
	worker   *worker
}

func initTicks() {
	for _, w := range workers {
		for _, t := range w.ticks {
			if tickCancel == nil {
				tickCancel = make(chan any)
				tickWg = sync.WaitGroup{}
			}
			tickWg.Add(1)
			t.worker = w
			if t.aligned {
				go t.startAlignedLoop(globalCtx)
			} else {
				go t.startLoop(globalCtx)
			}
		}
	}
}

func shutdownTicks() {
	if tickCancel == nil {
		return
	}
	close(tickCancel)
	tickWg.Wait()
	tickCancel = nil
}

func (t *tick) startLoop(ctx context.Context) {
	interval := t.interval
	if t.mode == TickModeIdle {
		// reduce the interval when ticking for idle threads
		// this way threads will be idle for at most 4/3 of the original interval
		interval = t.interval / 3
	}
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-tickCancel:
			ticker.Stop()
			tickWg.Done()
			return
		case <-ticker.C:
			t.send(ctx)
		}
	}
}

func (t *tick) startAlignedLoop(ctx context.Context) {
	timer := time.NewTimer(time.Until(nextAlignedTick(t.interval, time.Now())))

	for {
		select {
		case <-tickCancel:
			timer.Stop()
			tickWg.Done()
			return
		case <-timer.C:
			t.send(ctx)
			timer.Reset(time.Until(nextAlignedTick(t.interval, time.Now())))
		}
	}
}

// nextAlignedTick returns the next time that is a multiple of the given interval
// e.g. interval=15m aligns to :00, :15, :30, :45
func nextAlignedTick(interval time.Duration, now time.Time) time.Time {
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

func (t *tick) send(ctx context.Context) {
	switch t.mode {
	case TickModeEach, TickModeIdle:
		t.sendToEachThread(ctx)
	case TickModeOverlapping:
		go t.sendOnce(ctx)
	case TickModeSynchronous:
		t.sendOnce(ctx)
	}
}

func (t *tick) sendOnce(ctx context.Context) {
	tickWg.Add(1)
	fc := newContextFromMessage(t.message, nil, ctx, t.worker)

	if err := t.worker.handleRequest(fc); err != nil && globalLogger.Enabled(ctx, slog.LevelWarn) {
		globalLogger.LogAttrs(ctx, slog.LevelWarn, "worker tick failed", slog.String("worker", t.worker.name), slog.String("message", t.message), slog.Any("error", err))
	}
	tickWg.Done()
}

func (t *tick) sendToEachThread(ctx context.Context) {
	w := t.worker
	w.threadMutex.RLock()
	for _, thread := range w.threads {
		if t.mode == TickModeIdle && thread.state.WaitTime() < t.interval.Milliseconds() {
			continue
		}
		tickWg.Add(1)
		go func(thread *phpThread) {
			fc := newContextFromMessage(t.message, nil, ctx, w)
			if err := w.handleRequestOnThread(thread, fc); err != nil && globalLogger.Enabled(ctx, slog.LevelWarn) {
				globalLogger.LogAttrs(ctx, slog.LevelWarn, "worker tick failed", slog.String("worker", w.name), slog.String("message", t.message), slog.Any("error", err))
			}
			tickWg.Done()
		}(thread)
	}
	w.threadMutex.RUnlock()
}
