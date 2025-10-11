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
	taskChan    chan *PendingTask
	name        string
	num         int
	env         PreparedEnv
}

// representation of a thread that handles tasks directly assigned by go or via frankenphp_dispatch_task()
// can also just execute a script in a loop
// implements the threadHandler interface
type taskWorkerThread struct {
	thread       *phpThread
	taskWorker   *taskWorker
	dummyContext *frankenPHPContext
	currentTask  *PendingTask
}

const maxQueueLen = 1500 // TODO: make configurable somehow
var taskWorkers []*taskWorker

// EXPERIMENTAL: a task dispatched to a task worker
type PendingTask struct {
	arg      any
	done     sync.RWMutex
	callback func()
	Result   any
}

func (t *PendingTask) WaitForCompletion() {
	t.done.RLock()
}

// EXPERIMENTAL: DispatchTask dispatches a task to a named task worker
func DispatchTask(arg any, taskWorkerName string) (*PendingTask, error) {
	tw := getTaskWorkerByName(taskWorkerName)
	if tw == nil {
		return nil, errors.New("no task worker found with name " + taskWorkerName)
	}

	pt := &PendingTask{arg: arg}
	pt.done.Lock()

	tw.taskChan <- pt

	return pt, nil
}

// EXPERIMENTAL: ExecuteCallbackOnTaskWorker executes the callback func() directly on a task worker thread
// this gives the callback access to PHP's memory management
func ExecuteCallbackOnTaskWorker(callback func(), taskWorkerName string) (*PendingTask, error) {
	tw := getTaskWorkerByName(taskWorkerName)
	if tw == nil {
		return nil, errors.New("no task worker found with name " + taskWorkerName)
	}

	pt := &PendingTask{callback: callback}
	pt.done.Lock()

	tw.taskChan <- pt

	return pt, nil
}

func initTaskWorkers(opts []workerOpt) error {
	taskWorkers = make([]*taskWorker, 0, len(opts))
	ready := sync.WaitGroup{}
	for _, opt := range opts {
		fileName, err := fastabs.FastAbs(opt.fileName)
		if err != nil {
			return err
		}

		tw := &taskWorker{
			threads:  make([]*phpThread, 0, opt.num),
			fileName: fileName,
			taskChan: make(chan *PendingTask, maxQueueLen),
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
		select {
		// make sure all tasks are done by re-queuing them until the channel is empty
		case pt := <-tw.taskChan:
			tw.taskChan <- pt
		default:
		}
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

		// if the task has a callback, handle it directly
		// callbacks may call into C (C -> GO -> C)
		if task.callback != nil {
			task.callback()
			go_frankenphp_finish_task(threadIndex, nil)

			return go_frankenphp_worker_handle_task(threadIndex)
		}

		// if the task has no callback, forward it to PHP
		zval := phpValue(task.arg)
		thread.Pin(unsafe.Pointer(zval))

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
		handler.currentTask.Result = goValue(zv)
	}
	handler.currentTask.done.Unlock()
	handler.currentTask = nil
}

//export go_frankenphp_dispatch_task
func go_frankenphp_dispatch_task(zv *C.zval, name *C.char, nameLen C.size_t) C.bool {
	if zv == nil {
        logger.Error("no task argument provided")
        return C.bool(false)
    }

	var worker *taskWorker
	if name != nil && nameLen != 0 {
		worker = getTaskWorkerByName(C.GoStringN(name, C.int(nameLen)))
	} else if len(taskWorkers) != 0 {
		worker = taskWorkers[0]
	}

	if worker == nil {
		logger.Error("no task worker found to handle this task", "name", C.GoStringN(name, C.int(nameLen)))
		return C.bool(false)
	}

	// create a new task and lock it until the task is done
	task := &PendingTask{arg: goValue(zv)}
	task.done.Lock()

	// dispatch task immediately if a thread available (best performance)
	select {
	case worker.taskChan <- task:
		return C.bool(true)
	default:
	}

	go func() {
		worker.taskChan <- task
	}()

	return C.bool(true)
}
