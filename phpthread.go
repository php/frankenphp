package frankenphp

// #cgo nocallback frankenphp_new_php_thread
// #include "frankenphp.h"
import "C"
import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"unsafe"
)

// representation of the actual underlying PHP thread
// identified by the index in the phpThreads slice
type phpThread struct {
	runtime.Pinner
	threadIndex  int
	requestChan  chan *frankenPHPContext
	drainChan    chan struct{}
	handlerMu    sync.Mutex
	handler      threadHandler
	state        *threadState
	sandboxedEnv map[string]*C.zend_string
}

// interface that defines how the callbacks from the C thread should be handled
type threadHandler interface {
	name() string
	beforeScriptExecution() string
	afterScriptExecution(exitStatus int)
	getRequestContext() *frankenPHPContext
}

func newPHPThread(threadIndex int) *phpThread {
	return &phpThread{
		threadIndex: threadIndex,
		requestChan: make(chan *frankenPHPContext),
		state:       newThreadState(),
	}
}

// boot starts the underlying PHP thread
func (thread *phpThread) boot() {
	// thread must be in reserved state to boot
	if !thread.state.compareAndSwap(stateReserved, stateBooting) && !thread.state.compareAndSwap(stateBootRequested, stateBooting) {
		logger.Error("thread is not in reserved state: " + thread.state.name())
		panic("thread is not in reserved state: " + thread.state.name())
	}

	// boot threads as inactive
	thread.handlerMu.Lock()
	thread.handler = &inactiveThread{thread: thread}
	thread.drainChan = make(chan struct{})
	thread.handlerMu.Unlock()

	// start the actual posix thread - TODO: try this with go threads instead
	if !C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) {
		logger.LogAttrs(context.Background(), slog.LevelError, "unable to create thread", slog.Int("thread", thread.threadIndex))
		panic("unable to create thread")
	}

	thread.state.waitFor(stateInactive)
}

// shutdown the underlying PHP thread
func (thread *phpThread) shutdown() {
	if !thread.state.requestSafeStateChange(stateShuttingDown) {
		// already shutting down or done
		return
	}
	close(thread.drainChan)
	thread.state.waitFor(stateDone)
	thread.drainChan = make(chan struct{})

	// threads go back to the reserved state from which they can be booted again
	if mainThread.state.is(stateReady) {
		thread.state.set(stateReserved)
	}
}

// change the thread handler safely
// must be called from outside the PHP thread
func (thread *phpThread) setHandler(handler threadHandler) {
	thread.handlerMu.Lock()
	defer thread.handlerMu.Unlock()
	if !thread.state.requestSafeStateChange(stateTransitionRequested) {
		// no state change allowed == shutdown or done
		return
	}
	close(thread.drainChan)
	thread.state.waitFor(stateTransitionInProgress)
	thread.handler = handler
	thread.drainChan = make(chan struct{})
	thread.state.set(stateTransitionComplete)
}

// transition to a new handler safely
// is triggered by setHandler and executed on the PHP thread
func (thread *phpThread) transitionToNewHandler() string {
	thread.state.set(stateTransitionInProgress)
	thread.state.waitFor(stateTransitionComplete)
	// execute beforeScriptExecution of the new handler
	return thread.handler.beforeScriptExecution()
}

func (thread *phpThread) getRequestContext() *frankenPHPContext {
	return thread.handler.getRequestContext()
}

func (thread *phpThread) name() string {
	thread.handlerMu.Lock()
	name := thread.handler.name()
	thread.handlerMu.Unlock()
	return name
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

func (thread *phpThread) updateContext(isWorker bool, isTaskWorker bool) {
	C.frankenphp_update_thread_context(C.bool(isWorker), C.bool(isTaskWorker))
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

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()
	thread.state.set(stateDone)
}
