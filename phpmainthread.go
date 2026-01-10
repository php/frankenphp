package frankenphp

// #cgo nocallback frankenphp_new_main_thread
// #cgo nocallback frankenphp_init_persistent_string
// #cgo noescape frankenphp_new_main_thread
// #cgo noescape frankenphp_init_persistent_string
// #include <php_variables.h>
// #include "frankenphp.h"
import "C"
import (
	"log/slog"
	"strings"
	"sync"

	"github.com/dunglas/frankenphp/internal/memory"
	"github.com/dunglas/frankenphp/internal/phpheaders"
	"github.com/dunglas/frankenphp/internal/state"
)

// represents the main PHP thread
// the thread needs to keep running as long as all other threads are running
type phpMainThread struct {
	state           *state.ThreadState
	done            chan struct{}
	numThreads      int
	maxThreads      int
	phpIni          map[string]string
	commonHeaders   map[string]*C.zend_string
	knownServerKeys map[string]*C.zend_string
	sandboxedEnv    map[string]*C.zend_string
}

var (
	phpThreads []*phpThread
	mainThread *phpMainThread
)

// initPHPThreads starts the main PHP thread,
// a fixed number of inactive PHP threads
// and reserves a fixed number of possible PHP threads
func initPHPThreads(numThreads int, numMaxThreads int, phpIni map[string]string) (*phpMainThread, error) {
	mainThread = &phpMainThread{
		state:        state.NewThreadState(),
		done:         make(chan struct{}),
		numThreads:   numThreads,
		maxThreads:   numMaxThreads,
		phpIni:       phpIni,
		sandboxedEnv: initializeEnv(),
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
	if mainThread == nil {
		return // mainThread was never initialized
	}
	doneWG := sync.WaitGroup{}
	doneWG.Add(len(phpThreads))
	mainThread.state.Set(state.ShuttingDown)
	close(mainThread.done)
	for _, thread := range phpThreads {
		// shut down all reserved threads
		if thread.state.CompareAndSwap(state.Reserved, state.Done) {
			doneWG.Done()
			continue
		}
		// shut down all active threads
		go func(thread *phpThread) {
			thread.shutdown()
			doneWG.Done()
		}(thread)
	}

	doneWG.Wait()
	mainThread.state.Set(state.Done)
	mainThread.state.WaitFor(state.Reserved)
	phpThreads = nil
}

func (mainThread *phpMainThread) start() error {
	if C.frankenphp_new_main_thread(C.int(mainThread.numThreads)) != 0 {
		return ErrMainThreadCreation
	}

	mainThread.state.WaitFor(state.Ready)

	// cache common request headers as zend_strings (HTTP_ACCEPT, HTTP_USER_AGENT, etc.)
	mainThread.commonHeaders = make(map[string]*C.zend_string, len(phpheaders.CommonRequestHeaders))
	for key, phpKey := range phpheaders.CommonRequestHeaders {
		mainThread.commonHeaders[key] = C.frankenphp_init_persistent_string(C.CString(phpKey), C.size_t(len(phpKey)))
	}

	// cache $_SERVER keys as zend_strings (SERVER_PROTOCOL, SERVER_SOFTWARE, etc.)
	mainThread.knownServerKeys = make(map[string]*C.zend_string, len(knownServerKeys))
	for _, phpKey := range knownServerKeys {
		mainThread.knownServerKeys[phpKey] = C.frankenphp_init_persistent_string(toUnsafeChar(phpKey), C.size_t(len(phpKey)))
	}

	return nil
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
	mainThread.state.WaitFor(state.Done)
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
	for k, v := range mainThread.phpIni {
		overrides.WriteString(k)
		overrides.WriteByte('=')
		overrides.WriteString(v)
		overrides.WriteByte('\n')
	}

	return C.CString(overrides.String())
}
