package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"path/filepath"
	"sync"
)

type taskWorker struct {
	threads  []*phpThread
	mu       sync.Mutex
	filename string
	taskChan chan string
	name     string
}

// representation of a thread that handles tasks directly assigned by go
// implements the threadHandler interface
type taskWorkerThread struct {
	thread       *phpThread
	taskWorker   *taskWorker
	dummyContext *frankenPHPContext
}

var taskWorkers []*taskWorker

func initTaskWorkers() {
	taskWorkers = []*taskWorker{}
	tw := &taskWorker{
		threads:  []*phpThread{},
		filename: "/go/src/app/testdata/task_worker.php",
		taskChan: make(chan string),
		name:     "Default Task Worker",
	}

	taskWorkers = append(taskWorkers, tw)

	// need at least max_threads >= 2 + num_threads
	convertToTaskWorkerThread(getInactivePHPThread(), tw)
	convertToTaskWorkerThread(getInactivePHPThread(), tw)
}

func convertToTaskWorkerThread(thread *phpThread, tw *taskWorker) *taskWorkerThread {
	handler := &taskWorkerThread{
		thread:     thread,
		taskWorker: tw,
	}
	thread.setHandler(handler)

	tw.mu.Lock()
	tw.threads = append(tw.threads, thread)
	tw.mu.Unlock()

	return handler
}

func (handler *taskWorkerThread) beforeScriptExecution() string {
	thread := handler.thread

	switch thread.state.get() {
	case stateTransitionRequested:
		return thread.transitionToNewHandler()
	case stateBooting, stateTransitionComplete:
		thread.state.set(stateReady)

		return handler.setupWorkerScript()
	case stateReady:

		return handler.setupWorkerScript()
	case stateShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.name())
}

func (handler *taskWorkerThread) setupWorkerScript() string {
	fc, err := newDummyContext(filepath.Base(handler.taskWorker.filename))

	if err != nil {
		panic(err)
	}

	handler.dummyContext = fc

	return handler.taskWorker.filename
}

func (handler *taskWorkerThread) afterScriptExecution(int) {
	// shutdown?
}

func (handler *taskWorkerThread) getRequestContext() *frankenPHPContext {
	return handler.dummyContext
}

func (handler *taskWorkerThread) name() string {
	return "Task PHP Thread"
}

//export go_frankenphp_worker_handle_task
func go_frankenphp_worker_handle_task(threadIndex C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	handler, ok := thread.handler.(*taskWorkerThread)
	if !ok {
		panic("thread is not a task thread")
	}

	if !thread.state.is(stateReady) {
		thread.state.set(stateReady)
	}

	select {
	case taskString := <-handler.taskWorker.taskChan:
		return thread.pinCString(taskString)
	case <-handler.thread.drainChan:
		// thread is shutting down, do not execute the function
		return nil
	}
}

//export go_frankenphp_worker_dispatch_task
func go_frankenphp_worker_dispatch_task(taskWorkerIndex C.uintptr_t, taskString *C.char, taskLen C.size_t) C.bool {
	go func() {
		taskWorkers[taskWorkerIndex].taskChan <- C.GoStringN(taskString, C.int(taskLen))
	}()

	return C.bool(true)
}
