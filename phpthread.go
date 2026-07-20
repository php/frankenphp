package frankenphp

// #cgo nocallback frankenphp_new_php_thread
// #include "frankenphp.h"
import "C"
import (
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
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
	// drain is a hook called right before drainChan is closed on shutdown
	// and reboot. Handlers that need to wake up a thread parked in a
	// blocking C call (background workers' stream_select on the stop pipe)
	// plug their signal in here; the other handlers are no-ops.
	drain()
}

func newPHPThread(threadIndex int) *phpThread {
	return &phpThread{
		threadIndex: threadIndex,
		requestChan: make(chan *frankenPHPContext),
		state:       state.NewThreadState(),
	}
}

// boot starts the underlying PHP thread
func (thread *phpThread) boot() {
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
	if !C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) {
		panic("unable to create thread")
	}

	thread.state.WaitFor(state.Inactive)
}

// reboot the underlying C thread. Ignore the request if state is currently not Ready.
func (thread *phpThread) reboot() bool {
	if !thread.state.CompareAndSwap(state.Ready, state.Rebooting) {
		return false // thread is not ready to reboot
	}

	go func() {
		thread.state.WaitFor(state.RebootReady)

		if !C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) {
			panic("unable to create thread")
		}
	}()

	return true
}

// force the underlying C thread to reboot. Will always reboot unless already shutting down or done.
func (thread *phpThread) forceReboot() bool {
	if !thread.state.RequestSafeStateChange(state.ForceRebooting) {
		// thread already shutting down or done
		return false
	}

	go func() {
		thread.state.WaitFor(state.RebootReady)

		if !C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) {
			panic("unable to create thread")
		}
	}()

	return true
}

// shutdown the underlying PHP thread
func (thread *phpThread) shutdown() {
	if !thread.state.RequestSafeStateChange(state.ShuttingDown) {
		// thread is already shutting down, prefer the stable reserved state over done
		_ = thread.state.CompareAndSwap(state.Done, state.Reserved)
		return
	}

	// wake up handlers parked in a blocking C call (background workers'
	// stream_select on the stop pipe); no-op for the other handlers
	thread.handler.drain()
	close(thread.drainChan)

	// Arm force-kill after the grace period to wake any thread stuck in
	// a blocking syscall (sleep, blocking I/O). The wait remains
	// unbounded - on platforms where force-kill cannot interrupt the
	// syscall (macOS, Windows non-alertable Sleep) the thread will exit
	// when the syscall completes naturally; the operator's orchestrator
	// is responsible for any harder timeout.
	if !thread.state.WaitForStateWithTimeout(shutDownGracePeriod, state.Done) {
		globalLogger.LogAttrs(
			globalCtx,
			slog.LevelWarn,
			"force-killing thread on shutdown timeout",
			slog.String("name", thread.name()),
			slog.String("state", thread.state.Name()),
			slog.String("timeout", shutDownGracePeriod.String()),
		)
		thread.sendKillSignal()
		thread.state.WaitFor(state.Done)
	}

	thread.drainChan = make(chan struct{})

	// threads go back to the reserved state from which they can be booted again
	thread.state.Set(state.Reserved)
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
	if exitStatus < 0 {
		panic(ErrScriptExecution)
	}
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
