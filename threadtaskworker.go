package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"github.com/dunglas/frankenphp/internal/fastabs"
	"path/filepath"
	"sync"
)

type taskWorker struct {
	threads  []*phpThread
	mu       sync.Mutex
	filename string
	taskChan chan *dispatchedTask
	name     string
	num      int
}

// representation of a thread that handles tasks directly assigned by go
// implements the threadHandler interface
type taskWorkerThread struct {
	thread       *phpThread
	taskWorker   *taskWorker
	dummyContext *frankenPHPContext
	currentTask  *dispatchedTask
}

type dispatchedTask struct {
	char *C.char
	len  C.size_t
	done sync.RWMutex
}

func (t *dispatchedTask) waitForCompletion() {
	t.done.RLock()
}

var taskWorkers []*taskWorker

func initTaskWorkers(opts []workerOpt) error {
	taskWorkers = make([]*taskWorker, 0, len(opts))
	for _, opt := range opts {
		filename, err := fastabs.FastAbs(opt.fileName)
		if err != nil {
			return err
		}

		taskWorkers = append(taskWorkers,
			&taskWorker{
				threads:  make([]*phpThread, 0, opt.num),
				filename: filename,
				taskChan: make(chan *dispatchedTask),
				name:     opt.name,
				num:      opt.num,
			},
		)
	}

	ready := sync.WaitGroup{}
	for _, tw := range taskWorkers {
		ready.Add(tw.num)
		for i := 0; i < tw.num; i++ {
			thread := getInactivePHPThread()
			convertToTaskWorkerThread(thread, tw)
			go func() {
				thread.state.waitFor(stateReady)
				ready.Done()
			}()
		}
	}

	ready.Wait()

	return nil
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
func go_frankenphp_worker_handle_task(threadIndex C.uintptr_t) C.go_string {
	thread := phpThreads[threadIndex]
	handler, ok := thread.handler.(*taskWorkerThread)
	if !ok {
		panic("thread is not a task thread")
	}

	if !thread.state.is(stateReady) {
		thread.state.set(stateReady)
	}

	thread.state.markAsWaiting(true)

	select {
	case task := <-handler.taskWorker.taskChan:
		handler.currentTask = task
		thread.state.markAsWaiting(false)
		return C.go_string{len: task.len, data: task.char}
	case <-handler.thread.drainChan:
		thread.state.markAsWaiting(false)
		// send an empty task to drain the thread
		return C.go_string{len: 0, data: nil}
	}
}

//export go_frankenphp_finish_task
func go_frankenphp_finish_task(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	handler, ok := thread.handler.(*taskWorkerThread)
	if !ok {
		panic("thread is not a task thread")
	}

	handler.currentTask.done.Unlock()
	handler.currentTask = nil
}

//export go_frankenphp_worker_dispatch_task
func go_frankenphp_worker_dispatch_task(taskWorkerIndex C.uintptr_t, taskChar *C.char, taskLen C.size_t, name *C.char, nameLen C.size_t) C.bool {
	var worker *taskWorker
	if name != nil {
		name := C.GoStringN(name, C.int(nameLen))
		for _, w := range taskWorkers {
			if w.name == name {
				worker = w
				break
			}
		}
	} else {
		worker = taskWorkers[taskWorkerIndex]
	}

	if worker == nil {
		logger.Error("task worker does not exist", "name", C.GoStringN(name, C.int(nameLen)))
		return C.bool(false)
	}

	// create a new task and lock it until the task is done
	task := &dispatchedTask{char: taskChar, len: taskLen}
	task.done.Lock()

	// dispatch immediately if available (best performance)
	select {
	case taskWorkers[taskWorkerIndex].taskChan <- task:
		return C.bool(false)
	default:
	}

	// otherwise queue up in a non-blocking way
	go func() {
		taskWorkers[taskWorkerIndex].taskChan <- task
	}()

	return C.bool(true)
}

//export go_is_task_worker_thread
func go_is_task_worker_thread(threadIndex C.uintptr_t) C.bool {
	thread := phpThreads[threadIndex]
	_, ok := thread.handler.(*taskWorkerThread)

	return C.bool(ok)
}
