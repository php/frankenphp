package frankenphp

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type discardResponseWriter struct{}

func (discardResponseWriter) Header() http.Header { return make(http.Header) }

func (discardResponseWriter) Write(b []byte) (int, error) { return len(b), nil }

func (discardResponseWriter) WriteHeader(int) {}

func startWorkerPings() {
	for _, w := range workers {
		w.startPings()
	}
}

func stopWorkerPings() {
	for _, w := range workers {
		w.stopPings()
	}
}

func (w *worker) startPings() {
	if len(w.pings) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(globalCtx)
	w.pingCancel = cancel

	for _, ping := range w.pings {
		go w.runPing(ctx, ping)
	}
}

func (w *worker) stopPings() {
	if w.pingCancel != nil {
		w.pingCancel()
		w.pingCancel = nil
	}
}

func (w *worker) runPing(ctx context.Context, ping workerPing) {
	if ping.aligned {
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

func (w *worker) sendPings(ping workerPing) {
	if ping.each {
		w.sendPingToEachThread(ping.message)
		return
	}

	w.sendPing(ping.message)
}

func (w *worker) newPingContext(message string) *frankenPHPContext {
	return newContextFromMessage(message, discardResponseWriter{}, globalCtx, w)
}

func (w *worker) sendPing(message string) {
	fc := w.newPingContext(message)

	if err := w.handleRequest(fc); err != nil && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker ping failed", slog.String("worker", w.name), slog.String("message", message), slog.Any("error", err))
	}
}

func (w *worker) sendPingToEachThread(message string) {
	w.threadMutex.RLock()
	threads := make([]*phpThread, len(w.threads))
	copy(threads, w.threads)
	w.threadMutex.RUnlock()

	for _, thread := range threads {
		fc := w.newPingContext(message)

		if err := w.handleRequestOnThread(thread, fc); err != nil && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "worker ping failed", slog.String("worker", w.name), slog.String("message", message), slog.Any("error", err))
		}
	}
}
