package frankenphp

//#include "frankenphp.h"
//#include <sys/resource.h>
import "C"
import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/cpu"
)

const (
	// requests have to be stalled for at least this amount of time before scaling
	minStallTime = 5 * time.Millisecond
	// time to check for CPU usage before scaling a single thread
	cpuProbeTime = 120 * time.Millisecond
	// do not scale over this amount of CPU usage
	maxCpuUsageForScaling = 0.8
	// downscale idle threads every x seconds
	downScaleCheckTime = 5 * time.Second
	// max amount of threads stopped in one iteration of downScaleCheckTime
	maxTerminationCount = 10
	// autoscaled threads waiting for longer than this time are downscaled
	maxThreadIdleTime = 5 * time.Second
)

var (
	ErrMaxThreadsReached = errors.New("max amount of overall threads reached")

	scaleChan         chan *frankenPHPContext
	autoScaledThreads = []*phpThread{}
	scalingMu         = new(sync.RWMutex)
)

func initAutoScaling(mainThread *phpMainThread) {
	if mainThread.maxThreads <= mainThread.numThreads {
		scaleChan = nil
		return
	}

	scalingMu.Lock()
	scaleChan = make(chan *frankenPHPContext)
	maxScaledThreads := mainThread.maxThreads - mainThread.numThreads
	autoScaledThreads = make([]*phpThread, 0, maxScaledThreads)
	scalingMu.Unlock()

	go startUpscalingThreads(maxScaledThreads, scaleChan, mainThread.done)
	go startDownScalingThreads(mainThread.done)
}

func drainAutoScaling() {
	scalingMu.Lock()
	logger.LogAttrs(context.Background(), slog.LevelDebug, "shutting down autoscaling", slog.Int("autoScaledThreads", len(autoScaledThreads)))
	scalingMu.Unlock()
}

func addRegularThread() (*phpThread, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return nil, ErrMaxThreadsReached
	}
	convertToRegularThread(thread)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	return thread, nil
}

func addWorkerThread(worker *worker) (*phpThread, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return nil, ErrMaxThreadsReached
	}
	convertToWorkerThread(thread, worker)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	return thread, nil
}

// scaleWorkerThread adds a worker PHP thread automatically
func scaleWorkerThread(worker *worker) {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) {
		return
	}

	// probe CPU usage before scaling
	if !cpu.ProbeCPUs(cpuProbeTime, maxCpuUsageForScaling, mainThread.done) {
		return
	}

	thread, err := addWorkerThread(worker)
	if err != nil {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "could not increase max_threads, consider raising this limit", slog.String("worker", worker.name), slog.Any("error", err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)

	logger.LogAttrs(context.Background(), slog.LevelInfo, "upscaling worker thread", slog.String("worker", worker.name), slog.Int("thread", thread.threadIndex), slog.Int("num_threads", len(autoScaledThreads)))
}

// scaleRegularThread adds a regular PHP thread automatically
func scaleRegularThread() {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) {
		return
	}

	// probe CPU usage before scaling
	if !cpu.ProbeCPUs(cpuProbeTime, maxCpuUsageForScaling, mainThread.done) {
		return
	}

	thread, err := addRegularThread()
	if err != nil {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "could not increase max_threads, consider raising this limit", slog.Any("error", err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)

	logger.LogAttrs(context.Background(), slog.LevelInfo, "upscaling regular thread", slog.Int("thread", thread.threadIndex), slog.Int("num_threads", len(autoScaledThreads)))
}

func startUpscalingThreads(maxScaledThreads int, scale chan *frankenPHPContext, done chan struct{}) {
	for {
		scalingMu.Lock()
		scaledThreadCount := len(autoScaledThreads)
		scalingMu.Unlock()
		if scaledThreadCount >= maxScaledThreads {
			// we have reached max_threads, check again later
			select {
			case <-done:
				return
			case <-time.After(downScaleCheckTime):
				continue
			}
		}

		select {
		case fc := <-scale:
			timeSinceStalled := time.Since(fc.startedAt)

			// if the request has not been stalled long enough, wait and repeat
			if timeSinceStalled < minStallTime {
				select {
				case <-done:
					return
				case <-time.After(minStallTime - timeSinceStalled):
					continue
				}
			}

			// if the request has been stalled long enough, scale
			if fc.worker == nil {
				scaleRegularThread()
				return
			}

			if fc.worker.maxThreads > 0 && fc.worker.countThreads() >= fc.worker.maxThreads {
				logger.Debug("cannot scale worker thread, max threads reached for worker " + fc.worker.name)
				continue
			}

			scaleWorkerThread(fc.worker)
		case <-done:
			return
		}
	}
}

func startDownScalingThreads(done chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-time.After(downScaleCheckTime):
			deactivateThreads()
		}
	}
}

// deactivateThreads checks all threads and removes those that have been inactive for too long
func deactivateThreads() {
	stoppedThreadCount := 0
	scalingMu.Lock()
	defer scalingMu.Unlock()
	for i := len(autoScaledThreads) - 1; i >= 0; i-- {
		thread := autoScaledThreads[i]

		// the thread might have been stopped otherwise, remove it
		if thread.state.is(stateReserved) {
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
			continue
		}

		waitTime := thread.state.waitTime()
		if stoppedThreadCount > maxTerminationCount || waitTime == 0 {
			continue
		}

		// convert threads to inactive if they have been idle for too long
		if thread.state.is(stateReady) && waitTime > maxThreadIdleTime.Milliseconds() {
			convertToInactiveThread(thread)
			stoppedThreadCount++
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
			logger.LogAttrs(context.Background(), slog.LevelInfo, "downscaling thread", slog.Int("thread", thread.threadIndex), slog.Int64("wait_time", waitTime), slog.Int("num_threads", len(autoScaledThreads)))

			continue
		}

		// TODO: Completely stopping threads is more memory efficient
		// Some PECL extensions like #1296 will prevent threads from fully stopping (they leak memory)
		// Reactivate this if there is a better solution or workaround
		// if thread.state.is(stateInactive) && waitTime > maxThreadIdleTime.Milliseconds() {
		// 	logger.LogAttrs(nil, slog.LevelDebug, "auto-stopping thread", slog.Int("thread", thread.threadIndex))
		// 	thread.shutdown()
		// 	stoppedThreadCount++
		// 	autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
		// 	continue
		// }
	}
}
