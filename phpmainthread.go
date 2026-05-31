package frankenphp

// #cgo nocallback frankenphp_new_main_thread
// #cgo noescape frankenphp_new_main_thread
// #include "frankenphp.h"
// #include <php_variables.h>
import "C"
import (
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/memory"
	"github.com/dunglas/frankenphp/internal/phpheaders"
	"github.com/dunglas/frankenphp/internal/state"
)

// represents the main PHP thread
// the thread needs to keep running as long as all other threads are running
type phpMainThread struct {
	state       *state.ThreadState
	done        chan struct{}
	numThreads  int
	maxThreads  int
	phpIni      map[string]string
	isRebooting atomic.Bool
}

var (
	phpThreads    []*phpThread
	mainThread    *phpMainThread
	commonHeaders map[string]*C.zend_string

	// timeouts to wait for threads to yield before arming force-kill
	shutDownGracePeriod = 30 * time.Second
	rebootGracePeriod   = 6 * time.Second
)

// initPHPThreads starts the main PHP thread,
// a fixed number of inactive PHP threads
// and reserves a fixed number of possible PHP threads
func initPHPThreads(numThreads int, numMaxThreads int, phpIni map[string]string) (*phpMainThread, error) {
	mainThread = &phpMainThread{
		state:      state.NewThreadState(),
		done:       make(chan struct{}),
		numThreads: numThreads,
		maxThreads: numMaxThreads,
		phpIni:     phpIni,
	}

	// initialize the first thread
	// this needs to happen before starting the main thread
	// since some extensions access environment variables on startup
	// the threadIndex on the main thread defaults to 0 -> phpThreads[0].Pin(...)
	initialThread := newPHPThread(0)
	phpThreads = []*phpThread{initialThread}

	if err := mainThread.start(); err != nil {
		return nil, err
	}

	// Must follow start(): maxThreads is only final once
	// setAutomaticMaxThreads runs on the main PHP thread (before Ready).
	C.frankenphp_init_thread_metrics(C.int(mainThread.maxThreads))

	// initialize all other threads
	phpThreads = make([]*phpThread, mainThread.maxThreads)
	phpThreads[0] = initialThread
	for i := 1; i < mainThread.maxThreads; i++ {
		phpThreads[i] = newPHPThread(i)
	}

	// start the underlying C threads
	var ready sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		ready.Go(phpThreads[i].boot)
	}

	ready.Wait()

	return mainThread, nil
}

func drainPHPThreads() {
	// disallow any scaling or restarting threads while draining
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if mainThread == nil || !mainThread.state.Is(state.Ready) {
		return // mainThread was never initialized
	}

	doneWG := sync.WaitGroup{}
	doneWG.Add(len(phpThreads))
	mainThread.state.Set(state.ShuttingDown)
	close(mainThread.done)
	for _, thread := range phpThreads {
		go func(thread *phpThread) {
			thread.shutdown()
			doneWG.Done()
		}(thread)
	}

	doneWG.Wait()
	mainThread.state.Set(state.Done)
	mainThread.state.WaitFor(state.Reserved)
	C.frankenphp_destroy_thread_metrics()
	phpThreads = nil
}

func (mainThread *phpMainThread) start() error {
	if C.frankenphp_new_main_thread(C.int(mainThread.numThreads)) != 0 {
		return ErrMainThreadCreation
	}

	mainThread.state.WaitFor(state.Ready)

	// cache common request headers as zend_strings (HTTP_ACCEPT, HTTP_USER_AGENT, etc.)
	if commonHeaders == nil {
		commonHeaders = make(map[string]*C.zend_string, len(phpheaders.CommonRequestHeaders))
		for key, phpKey := range phpheaders.CommonRequestHeaders {
			commonHeaders[key] = newPersistentZendString(phpKey)
		}
	}

	return nil
}

// rebootAllThreads reboots all underlying C threads, but keeps the go side alive
func (mainThread *phpMainThread) rebootAllThreads() bool {
	if !mainThread.isRebooting.CompareAndSwap(false, true) {
		return false
	}

	// allow no scaling or shutdown while rebooting
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.Is(state.Ready) {
		// mainThread has shut down in the meantime, no need to reboot
		return false
	}

	rebootStart := time.Now()
	rebootWg := sync.WaitGroup{}
	rebootingThreads := []*phpThread{}

	globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "rebooting all PHP threads")

	for _, thread := range phpThreads {
		if thread.reboot() {
			rebootingThreads = append(rebootingThreads, thread)
		}
	}

	for _, thread := range rebootingThreads {
		rebootWg.Go(func() {
			close(thread.drainChan)
			if thread.state.WaitForStateWithTimeout(rebootGracePeriod, state.YieldingForReboot) {
				return
			}

			// thread has not shut down in time, force kill the thread on the PHP side
			globalLogger.LogAttrs(
				globalCtx,
				slog.LevelWarn,
				"force-killing thread on reboot timeout",
				slog.String("name", thread.name()),
				slog.String("state", thread.state.Name()),
				slog.String("timeout", rebootGracePeriod.String()),
			)
			thread.sendKillSignal()
			thread.state.WaitFor(state.YieldingForReboot)
		})
	}

	rebootWg.Wait()

	mainThread.state.Set(state.Rebooting)
	mainThread.state.WaitFor(state.YieldingForReboot)
	if C.frankenphp_new_main_thread(C.int(mainThread.numThreads)) != 0 {
		panic("unable to recreate main thread after reboot")
	}
	mainThread.state.WaitFor(state.Ready)

	for _, thread := range rebootingThreads {
		thread.drainChan = make(chan struct{})
		if thread.state.CompareAndSwap(state.YieldingForReboot, state.RebootReady) {
			// wait for any of the stable states
			thread.state.WaitFor(state.Ready, state.Inactive, state.Reserved)
		} else {
			panic("unexpected state on reboot: " + thread.state.Name() + " for thread " + thread.name())
		}
	}

	mainThread.isRebooting.Store(false)

	globalLogger.LogAttrs(
		globalCtx,
		slog.LevelInfo,
		"all PHP threads rebooted",
		slog.String("duration", time.Since(rebootStart).String()),
		slog.Int("num_threads", len(rebootingThreads)),
	)

	return true
}

func getInactivePHPThread() *phpThread {
	for _, thread := range phpThreads {
		if thread.state.Is(state.Inactive) {
			return thread
		}
	}

	for _, thread := range phpThreads {
		if thread.state.CompareAndSwap(state.Reserved, state.BootRequested) {
			thread.boot()
			return thread
		}
	}

	return nil
}

//export go_frankenphp_main_thread_is_ready
func go_frankenphp_main_thread_is_ready() {
	mainThread.setAutomaticMaxThreads()
	if mainThread.maxThreads < mainThread.numThreads {
		mainThread.maxThreads = mainThread.numThreads
	}

	mainThread.state.Set(state.Ready)
	mainThread.state.WaitFor(state.Done, state.Rebooting)
}

// max_threads = auto
// setAutomaticMaxThreads estimates the amount of threads based on php.ini and system memory_limit
// If unable to get the system's memory limit, simply double num_threads
func (mainThread *phpMainThread) setAutomaticMaxThreads() {
	if mainThread.maxThreads >= 0 {
		return
	}
	perThreadMemoryLimit := int64(C.frankenphp_get_current_memory_limit())
	totalSysMemory := memory.TotalSysMemory()
	if perThreadMemoryLimit <= 0 || totalSysMemory == 0 {
		mainThread.maxThreads = mainThread.numThreads * 2
		return
	}
	maxAllowedThreads := totalSysMemory / uint64(perThreadMemoryLimit)
	mainThread.maxThreads = int(maxAllowedThreads)

	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "Automatic thread limit", slog.Int("perThreadMemoryLimitMB", int(perThreadMemoryLimit/1024/1024)), slog.Int("maxThreads", mainThread.maxThreads))
	}
}

//export go_frankenphp_shutdown_main_thread
func go_frankenphp_shutdown_main_thread() {
	if mainThread.state.CompareAndSwap(state.Rebooting, state.YieldingForReboot) {
		return
	}
	mainThread.state.Set(state.Reserved)
}

//export go_get_custom_php_ini
func go_get_custom_php_ini(disableTimeouts C.bool) *C.char {
	if mainThread.phpIni == nil {
		mainThread.phpIni = make(map[string]string)
	}

	// Timeouts are currently fundamentally broken
	// with ZTS except on Linux and FreeBSD: https://bugs.php.net/bug.php?id=79464
	// Disable timeouts if ZEND_MAX_EXECUTION_TIMERS is not supported
	if disableTimeouts {
		mainThread.phpIni["max_execution_time"] = "0"
		mainThread.phpIni["max_input_time"] = "-1"
	}

	// Pass the php.ini overrides to PHP before startup
	// TODO: if needed this would also be possible on a per-thread basis
	var overrides strings.Builder

	// 32 is an over-estimate for php.ini settings
	overrides.Grow(len(mainThread.phpIni) * 32)
	for k, v := range mainThread.phpIni {
		overrides.WriteString(k)
		overrides.WriteByte('=')
		overrides.WriteString(v)
		overrides.WriteByte('\n')
	}

	return C.CString(overrides.String())
}
