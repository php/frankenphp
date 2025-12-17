package frankenphp

// #cgo nocallback frankenphp_new_php_thread
// #include "frankenphp.h"
import "C"
import (
	"context"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/state"
)

// representation of the actual underlying PHP thread
// identified by the index in the phpThreads slice
type phpThread struct {
	runtime.Pinner
	threadIndex            int
	requestChan            chan contextHolder
	drainChan              chan struct{}
	handlerMu              sync.Mutex
	handler                threadHandler
	state                  *state.ThreadState
	sandboxedEnv           map[string]*C.zend_string
	cpuRequestStartTime    C.struct_timespec
	lastRequestCpuUsage    time.Duration
	lastRequestMemoryUsage uint64
}

// interface that defines how the callbacks from the C thread should be handled
type threadHandler interface {
	name() string
	beforeScriptExecution() string
	afterScriptExecution(exitStatus int)
	context() context.Context
	frankenPHPContext() *frankenPHPContext
}

func newPHPThread(threadIndex int) *phpThread {
	return &phpThread{
		threadIndex: threadIndex,
		requestChan: make(chan contextHolder),
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

// shutdown the underlying PHP thread
func (thread *phpThread) shutdown() {
	if !thread.state.RequestSafeStateChange(state.ShuttingDown) {
		// already shutting down or done
		return
	}
	close(thread.drainChan)
	thread.state.WaitFor(state.Done)
	thread.drainChan = make(chan struct{})

	// threads go back to the reserved state from which they can be booted again
	if mainThread.state.Is(state.Ready) {
		thread.state.Set(state.Reserved)
	}
}

// change the thread handler safely
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

func (thread *phpThread) frankenPHPContext() *frankenPHPContext {
	return thread.handler.frankenPHPContext()
}

func (thread *phpThread) context() context.Context {
	if thread.handler == nil {
		// handler can be nil when using opcache.preload
		return globalCtx
	}

	return thread.handler.context()
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

func (*phpThread) updateContext(isWorker bool) {
	C.frankenphp_update_local_thread_context(C.bool(isWorker))
}

func (thread *phpThread) requestCpuUsage() time.Duration {
	var cpuEndTime C.struct_timespec
	C.clock_gettime(C.CLOCK_THREAD_CPUTIME_ID, &cpuEndTime)
	return time.Duration(float64(cpuEndTime.tv_sec-thread.cpuRequestStartTime.tv_sec)*1e9 + float64(cpuEndTime.tv_nsec-thread.cpuRequestStartTime.tv_nsec))
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

	// Calculate CPU usage
	thread.lastRequestCpuUsage = thread.requestCpuUsage()

	thread.handler.afterScriptExecution(int(exitStatus))

	// unpin all memory used during script execution
	thread.Unpin()
}

//export go_frankenphp_set_memory_peak_usage
func go_frankenphp_set_memory_peak_usage(threadIndex C.uintptr_t, memoryPeakUsage C.size_t) {
	thread := phpThreads[threadIndex]
	thread.lastRequestMemoryUsage = uint64(memoryPeakUsage)
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()
	thread.state.Set(state.Done)
}
