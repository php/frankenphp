package frankenphp

// #cgo nocallback frankenphp_new_php_thread
// #include "frankenphp.h"
import "C"
import (
	"log/slog"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

// representation of the actual underlying PHP thread
// identified by the index in the phpThreads slice
type phpThread struct {
	runtime.Pinner
	threadIndex  int
	requestChan  chan *frankenPHPContext
	drainChan    chan struct{}
	handlerMu    sync.RWMutex
	handler      threadHandler
	contextMu    sync.RWMutex
	state        *state.ThreadState
	requestCount atomic.Int64
	// forceKill holds &EG() pointers captured on the PHP thread itself.
	// forceKillMu pairs with go_frankenphp_clear_force_kill_slot's write
	// lock so a concurrent kill never dereferences pointers freed by
	// ts_free_thread.
	forceKillMu sync.RWMutex
	forceKill   C.force_kill_slot
}

// threadHandler defines how the callbacks from the C thread should be handled
type threadHandler interface {
	name() string
	beforeScriptExecution() string
	afterScriptExecution(exitStatus int)
	frankenPHPContext() *frankenPHPContext
	// drain is a hook called by drainWorkerThreads right before drainChan is
	// closed. Handlers that need to wake up a thread parked in a blocking C
	// call (e.g. by closing a stop pipe) plug their signal in here. All
	// current handlers are no-ops; this is the seam later handler types use
	// without having to modify drainWorkerThreads.
	drain()
}

func newPHPThread(threadIndex int) *phpThread {
	return &phpThread{
		threadIndex: threadIndex,
		requestChan: make(chan *frankenPHPContext),
		state:       state.NewThreadState(),
	}
}

const (
	maxThreadCreationRetryDelay = 256 * time.Millisecond
	maxThreadRestoreDelay       = 10 * time.Second
)

// overridden in tests to simulate the OS refusing to create a thread
var newCThread = func(threadIndex int) bool {
	return bool(C.frankenphp_new_php_thread(C.uintptr_t(threadIndex)))
}

// boot starts the underlying PHP thread
func (thread *phpThread) boot() error {
	// thread must be in reserved state to boot
	if !thread.state.CompareAndSwap(state.Reserved, state.Booting) && !thread.state.CompareAndSwap(state.BootRequested, state.Booting) {
		panic("thread is not in reserved state: " + thread.state.Name())
	}

	// boot threads as inactive
	thread.handlerMu.Lock()
	thread.handler = &inactiveThread{thread: thread}
	thread.drainChan = make(chan struct{})
	thread.handlerMu.Unlock()

	// start the actual posix thread - TODO: try this with go threads instead
	if !newCThread(thread.threadIndex) {
		// e.g. RLIMIT_NPROC reached: the thread can be booted again later
		thread.state.Set(state.Reserved)

		return ErrThreadCreation
	}

	thread.state.WaitFor(state.Inactive)

	return nil
}

// reboot the underlying C thread. Ignore the request if state is currently not Ready.
func (thread *phpThread) reboot() bool {
	if !thread.state.CompareAndSwap(state.Ready, state.Rebooting) {
		return false // thread is not ready to reboot
	}

	go thread.startRebootedThread(thread.handler)

	return true
}

// force the underlying C thread to reboot. Will always reboot unless already shutting down or done.
func (thread *phpThread) forceReboot() bool {
	if !thread.state.RequestSafeStateChange(state.ForceRebooting) {
		// thread already shutting down or done
		return false
	}

	// read without handlerMu: a setHandler() waiting for this reboot holds it, and nothing writes the handler until then
	go thread.startRebootedThread(thread.handler)

	return true
}

// retryThreadCreation retries for about half a second, as an exiting C thread may still count against the OS thread limit
func retryThreadCreation(create func() bool) bool {
	for delay := time.Millisecond; !create(); delay *= 2 {
		if delay > maxThreadCreationRetryDelay {
			return false
		}

		time.Sleep(delay)
	}

	return true
}

func (thread *phpThread) startRebootedThread(handler threadHandler) {
	thread.state.WaitFor(state.RebootReady)

	if !retryThreadCreation(func() bool { return newCThread(thread.threadIndex) }) {
		thread.releaseAfterFailedReboot(handler)
	}
}

func (thread *phpThread) releaseAfterFailedReboot(handler threadHandler) {
	globalLogger.LogAttrs(globalCtx, slog.LevelError, "unable to restart PHP thread", slog.Int("thread", thread.threadIndex), slog.String("name", handler.name()), slog.Any("error", ErrThreadCreation))

	mt := mainThread
	switch handler := handler.(type) {
	case *regularThread:
		detachRegularThread(thread)
		go restoreThread(mt, numRegularThreads, regularThreadsSnapshot, addRegularThread)
	case *workerThread:
		w := handler.worker
		if w.onThreadShutdown != nil {
			w.onThreadShutdown(thread.threadIndex)
		}
		w.detachThread(thread)
		go restoreThread(mt, w.num, w.threadsSnapshot, func() (*phpThread, error) { return addWorkerThread(w) })
	}

	thread.state.Set(state.Reserved)
}

// restoreThread keeps trying to bring a pool back to its configured size, so pools without autoscaling recover
func restoreThread(mt *phpMainThread, size int, pool func() []*phpThread, add func() (*phpThread, error)) {
	for delay := maxThreadCreationRetryDelay; ; delay = min(2*delay, maxThreadRestoreDelay) {
		select {
		case <-mt.done:
			return
		case <-time.After(delay):
		}

		scalingMu.Lock()
		if !mt.state.Is(state.Ready) || poolRestored(size, pool()) {
			scalingMu.Unlock()

			return
		}

		thread, err := add()
		if err != nil {
			scalingMu.Unlock()

			if err == errMaxWorkerThreadsReached {
				return
			}

			continue
		}

		globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "PHP thread restored", slog.Int("thread", thread.threadIndex), slog.String("name", thread.name()))
		scalingMu.Unlock()

		return
	}
}

// poolRestored reports whether the pool has its configured threads, adopting an autoscaled thread that took a free slot.
// It must be called with scalingMu held.
func poolRestored(size int, pool []*phpThread) bool {
	configured, scaled := 0, -1
	for _, thread := range pool {
		if i := slices.Index(autoScaledThreads, thread); i >= 0 {
			scaled = i
		} else {
			configured++
		}
	}

	if configured >= size {
		// the thread that failed was autoscaled, or another restore already replaced it
		return true
	}

	if scaled < 0 {
		return false
	}

	autoScaledThreads = slices.Delete(autoScaledThreads, scaled, scaled+1)

	return true
}

// shutdown the underlying PHP thread, block until shutdown is finished
func (thread *phpThread) shutdown() {
	if !thread.state.RequestSafeStateChange(state.ShuttingDown) {
		// a worker that failed to boot published ShuttingDown itself and is
		// still walking its C exit path: join it before TSRM is torn down
		thread.waitForExit(state.Done, state.Reserved)
		// prefer the stable reserved state over done
		_ = thread.state.CompareAndSwap(state.Done, state.Reserved)

		return
	}

	close(thread.drainChan)

	thread.waitForExit(state.Done)

	thread.drainChan = make(chan struct{})

	// threads go back to the reserved state from which they can be booted again
	thread.state.Set(state.Reserved)
}

// waitForExit joins the C thread, arming force-kill past the grace period
func (thread *phpThread) waitForExit(exitStates ...state.State) {
	if thread.state.WaitForStateWithTimeout(shutDownGracePeriod, exitStates...) {
		return
	}

	// Arm force-kill after the grace period to wake any thread stuck in
	// a blocking syscall (sleep, blocking I/O). The wait remains
	// unbounded - on platforms where force-kill cannot interrupt the
	// syscall (macOS, Windows non-alertable Sleep) the thread will exit
	// when the syscall completes naturally; the operator's orchestrator
	// is responsible for any harder timeout.
	globalLogger.LogAttrs(
		globalCtx,
		slog.LevelWarn,
		"force-killing thread on shutdown timeout",
		slog.String("name", thread.name()),
		slog.String("state", thread.state.Name()),
		slog.String("timeout", shutDownGracePeriod.String()),
	)
	thread.sendKillSignal()
	thread.state.WaitFor(exitStates...)
}

// setHandler changes the thread handler safely
// must be called from outside the PHP thread
func (thread *phpThread) setHandler(handler threadHandler) {
	thread.handlerMu.Lock()
	defer thread.handlerMu.Unlock()

	if !thread.state.RequestSafeStateChange(state.TransitionRequested) {
		// no state change allowed == shutdown or done
		return
	}

	close(thread.drainChan)

	thread.state.WaitFor(state.TransitionInProgress)
	thread.handler = handler
	thread.drainChan = make(chan struct{})
	thread.state.Set(state.TransitionComplete)
}

// transition to a new handler safely
// is triggered by setHandler and executed on the PHP thread
func (thread *phpThread) transitionToNewHandler() string {
	thread.state.Set(state.TransitionInProgress)
	thread.state.WaitFor(state.TransitionComplete)

	// execute beforeScriptExecution of the new handler
	return thread.handler.beforeScriptExecution()
}

func (thread *phpThread) name() string {
	thread.handlerMu.RLock()
	defer thread.handlerMu.RUnlock()

	if thread.handler == nil {
		return "unknown"
	}

	return thread.handler.name()
}

// currentContext returns the request context of the current handler, if any.
func (thread *phpThread) currentContext() *frankenPHPContext {
	thread.handlerMu.RLock()
	defer thread.handlerMu.RUnlock()

	if thread.handler == nil {
		return nil
	}

	return thread.handler.frankenPHPContext()
}

// send a kill signal to PHP (ZTS compatible)
// make sure to only call this if PHP is actively handling a request
func (thread *phpThread) sendKillSignal() {
	thread.forceKillMu.RLock()
	C.frankenphp_force_kill_thread(thread.forceKill)
	thread.forceKillMu.RUnlock()
}

// Pin a string that is not null-terminated
// PHP's zend_string may contain null-bytes
func (thread *phpThread) pinString(s string) *C.char {
	sData := unsafe.StringData(s)
	if sData == nil {
		return nil
	}

	thread.Pin(sData)

	return (*C.char)(unsafe.Pointer(sData))
}

// C strings must be null-terminated
func (thread *phpThread) pinCString(s string) *C.char {
	return thread.pinString(s + "\x00")
}

func (*phpThread) updateContext(isWorker bool) {
	C.frankenphp_update_local_thread_context(C.bool(isWorker))
}

//export go_frankenphp_before_script_execution
func go_frankenphp_before_script_execution(threadIndex C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	scriptName := thread.handler.beforeScriptExecution()

	// if no scriptName is passed, shut down
	if scriptName == "" {
		return nil
	}

	// return the name of the PHP script that should be executed
	return thread.pinCString(scriptName)
}

//export go_frankenphp_after_script_execution
func go_frankenphp_after_script_execution(threadIndex C.uintptr_t, exitStatus C.int) {
	thread := phpThreads[threadIndex]
	thread.handler.afterScriptExecution(int(exitStatus))

	// unpin all memory used during script execution
	thread.Unpin()
}

//export go_frankenphp_store_force_kill_slot
func go_frankenphp_store_force_kill_slot(threadIndex C.uintptr_t, slot C.force_kill_slot) {
	thread := phpThreads[threadIndex]
	thread.forceKillMu.Lock()
	// Release any prior slot's OS resource (Windows HANDLE) before
	// overwriting; a phpThread can reboot and re-register.
	C.frankenphp_release_thread_for_kill(thread.forceKill)
	thread.forceKill = slot
	thread.forceKillMu.Unlock()
}

//export go_frankenphp_clear_force_kill_slot
func go_frankenphp_clear_force_kill_slot(threadIndex C.uintptr_t) {
	// Called from C before ts_free_thread on both exit paths. Zeroing
	// the slot under the write lock guarantees any concurrent kill
	// either completed before we got the lock or sees a zero slot.
	thread := phpThreads[threadIndex]
	thread.forceKillMu.Lock()
	C.frankenphp_release_thread_for_kill(thread.forceKill)
	thread.forceKill = C.force_kill_slot{}
	thread.forceKillMu.Unlock()
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()

	switch thread.state.Get() {
	case state.Rebooting:
		thread.state.Set(state.RebootReady)
	case state.ForceRebooting:
		thread.state.Set(state.YieldingForReboot)
	default:
		thread.state.Set(state.Done)
	}
}
