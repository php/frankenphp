package frankenphp

// #include "frankenphp.h"
// #include <php_variables.h>
import "C"
import (
	"errors"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"path/filepath"
	"sync"
	"unsafe"
)

type taskWorker struct {
	threads     []*phpThread
	threadMutex sync.RWMutex
	fileName    string
	taskChan    chan *pendingTask
	name        string
	num         int
	env         PreparedEnv
}

// representation of a thread that handles tasks directly assigned by go or via frankenphp_dispatch_request()
// can also just execute a script in a loop
// implements the threadHandler interface
type taskWorkerThread struct {
	thread       *phpThread
	taskWorker   *taskWorker
	dummyContext *frankenPHPContext
	currentTask  *pendingTask
}

var taskWorkers []*taskWorker

// EXPERIMENTAL: a task dispatched to a task worker
type pendingTask struct {
	arg      any // the argument passed to frankenphp_send_request()
	result   any // the return value of frankenphp_handle_request()
	done     sync.RWMutex
	callback func() // optional callback for direct execution (tests)
}

func (t *pendingTask) dispatch(tw *taskWorker) error {
	t.done.Lock()
	select {
	case tw.taskChan <- t:
		return nil
	default:
		return errors.New("Task worker queue is full, cannot dispatch task: " + tw.name)
	}
}

func initTaskWorkers(opts []workerOpt) error {
	taskWorkers = make([]*taskWorker, 0, len(opts))
	ready := sync.WaitGroup{}
	for _, opt := range opts {
		fileName, err := fastabs.FastAbs(opt.fileName)
		if err != nil {
			return err
		}

		if opt.maxQueueLen <= 0 {
			opt.maxQueueLen = 10000 // default queue len, TODO: unlimited?
		}

		tw := &taskWorker{
			threads:  make([]*phpThread, 0, opt.num),
			fileName: fileName,
			taskChan: make(chan *pendingTask, opt.maxQueueLen),
			name:     opt.name,
			num:      opt.num,
			env:      opt.env,
		}
		taskWorkers = append(taskWorkers, tw)

		// start the actual PHP threads
		ready.Add(tw.num)
		for i := 0; i < tw.num; i++ {
			thread := getInactivePHPThread()
			convertToTaskWorkerThread(thread, tw)
			go func(thread *phpThread) {
				thread.state.waitFor(stateReady)
				ready.Done()
			}(thread)
		}
	}
	ready.Wait()

	return nil
}

func drainTaskWorkers() {
	for _, tw := range taskWorkers {
		tw.drainQueue()
	}
}

func convertToTaskWorkerThread(thread *phpThread, tw *taskWorker) *taskWorkerThread {
	handler := &taskWorkerThread{
		thread:     thread,
		taskWorker: tw,
	}
	thread.setHandler(handler)

	return handler
}

func (handler *taskWorkerThread) beforeScriptExecution() string {
	thread := handler.thread

	switch thread.state.get() {
	case stateTransitionRequested:
		handler.taskWorker.detach(thread)

		return thread.transitionToNewHandler()
	case stateBooting, stateTransitionComplete:
		tw := handler.taskWorker
		tw.threadMutex.Lock()
		tw.threads = append(tw.threads, thread)
		tw.threadMutex.Unlock()
		thread.state.set(stateReady)
		thread.updateContext(false, true)

		return handler.setupWorkerScript()
	case stateReady:

		return handler.setupWorkerScript()
	case stateRestarting:
		thread.state.set(stateYielding)
		thread.state.waitFor(stateReady, stateShuttingDown)

		return handler.beforeScriptExecution()
	case stateShuttingDown:
		handler.taskWorker.detach(thread)
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.name())
}

func (handler *taskWorkerThread) setupWorkerScript() string {
	fc, err := newDummyContext(
		filepath.Base(handler.taskWorker.fileName),
		WithRequestPreparedEnv(handler.taskWorker.env),
	)

	if err != nil {
		panic(err)
	}

	handler.dummyContext = fc
	clearSandboxedEnv(handler.thread)

	return handler.taskWorker.fileName
}

func (handler *taskWorkerThread) afterScriptExecution(int) {
	// restart the script
}

func (handler *taskWorkerThread) getRequestContext() *frankenPHPContext {
	return handler.dummyContext
}

func (handler *taskWorkerThread) name() string {
	return "Task Worker PHP Thread - " + handler.taskWorker.fileName
}

func (tw *taskWorker) detach(thread *phpThread) {
	tw.threadMutex.Lock()
	for i, t := range tw.threads {
		if t == thread {
			tw.threads = append(tw.threads[:i], tw.threads[i+1:]...)
			return
		}
	}
	tw.threadMutex.Unlock()
}

// make sure all tasks are done by re-queuing them until the channel is empty
func (tw *taskWorker) drainQueue() {
	for {
		select {
		case pt := <-tw.taskChan:
			tw.taskChan <- pt
			pt.done.RLock() // wait for completion
		default:
			return
		}
	}
}

func getTaskWorkerByName(name string) *taskWorker {
	for _, w := range taskWorkers {
		if w.name == name {
			return w
		}
	}

	return nil
}

//export go_frankenphp_worker_handle_task
func go_frankenphp_worker_handle_task(threadIndex C.uintptr_t) *C.zval {
	thread := phpThreads[threadIndex]
	handler, _ := thread.handler.(*taskWorkerThread)
	thread.Unpin()
	thread.state.markAsWaiting(true)

	select {
	case task := <-handler.taskWorker.taskChan:
		handler.currentTask = task
		thread.state.markAsWaiting(false)

		// if the task has a callback, execute it (see types_test.go)
		if task.callback != nil {
			task.callback()
			go_frankenphp_finish_task(threadIndex, nil)

			return go_frankenphp_worker_handle_task(threadIndex)
		}

		zval := phpValue(task.arg)
		thread.Pin(unsafe.Pointer(zval)) // TODO: refactor types.go so no pinning is required

		return zval
	case <-handler.thread.drainChan:
		thread.state.markAsWaiting(false)
		// send an empty task to drain the thread
		return nil
	}
}

//export go_frankenphp_finish_task
func go_frankenphp_finish_task(threadIndex C.uintptr_t, zv *C.zval) {
	thread := phpThreads[threadIndex]
	handler, ok := thread.handler.(*taskWorkerThread)
	if !ok {
		panic("thread is not a task thread")
	}

	if zv != nil {
		result, err := goValue[any](zv)
		if err != nil {
			panic("failed to convert go_frankenphp_finish_task() return value: " + err.Error())
		}
		handler.currentTask.result = result
	}
	handler.currentTask.done.Unlock()
	handler.currentTask = nil
}

//export go_frankenphp_dispatch_request
func go_frankenphp_dispatch_request(threadIndex C.uintptr_t, zv *C.zval, name *C.char, nameLen C.size_t) *C.char {
	if zv == nil {
		return phpThreads[threadIndex].pinCString("Task argument cannot be null")
	}

	var tw *taskWorker
	if nameLen != 0 {
		tw = getTaskWorkerByName(C.GoStringN(name, C.int(nameLen)))
	} else if len(taskWorkers) != 0 {
		tw = taskWorkers[0]
	}

	if tw == nil {
		return phpThreads[threadIndex].pinCString("No worker found to handle this task: " + C.GoStringN(name, C.int(nameLen)))
	}

	// create a new task and lock it until the task is done
	goArg, err := goValue[any](zv)
	if err != nil {
		return phpThreads[threadIndex].pinCString("Failed to convert go_frankenphp_dispatch_request() argument: " + err.Error())
	}

	task := &pendingTask{arg: goArg}
	err = task.dispatch(tw)

	if err != nil {
		return phpThreads[threadIndex].pinCString(err.Error())
	}

	return nil
}
