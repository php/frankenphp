package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"errors"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"path/filepath"
	"sync"
)

type taskWorker struct {
	threads     []*phpThread
	threadMutex sync.RWMutex
	filename    string
	taskChan    chan *PendingTask
	name        string
	num         int
	env         PreparedEnv
}

// representation of a thread that handles tasks directly assigned by go
// implements the threadHandler interface
type taskWorkerThread struct {
	thread       *phpThread
	taskWorker   *taskWorker
	dummyContext *frankenPHPContext
	currentTask  *PendingTask
}

var taskWorkers []*taskWorker

// EXPERIMENTAL: a task dispatched to a task worker
type PendingTask struct {
	str  string
	done sync.RWMutex
}

func (t *PendingTask) WaitForCompletion() {
	t.done.RLock()
}

// EXPERIMENTAL: DispatchTask dispatches a task to a named task worker
func DispatchTask(task string, workerName string) (*PendingTask, error) {
	tw := getTaskWorkerByName(workerName)
	if tw == nil {
		return nil, errors.New("no task worker found with name " + workerName)
	}

	pt := &PendingTask{str: task}
	pt.done.Lock()

	tw.taskChan <- pt

	return pt, nil
}

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
				taskChan: make(chan *PendingTask),
				name:     opt.name,
				num:      opt.num,
				env:      opt.env,
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

	tw.threadMutex.Lock()
	tw.threads = append(tw.threads, thread)
	tw.threadMutex.Unlock()

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
	case stateRestarting:
		thread.state.set(stateYielding)
		thread.state.waitFor(stateReady, stateShuttingDown)

		return handler.beforeScriptExecution()
	case stateShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.name())
}

func (handler *taskWorkerThread) setupWorkerScript() string {
	fc, err := newDummyContext(
		filepath.Base(handler.taskWorker.filename),
		WithRequestPreparedEnv(handler.taskWorker.env),
	)

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

func getTaskWorkerByName(name string) *taskWorker {
	for _, w := range taskWorkers {
		if w.name == name {
			return w
		}
	}

	return nil
}

//export go_frankenphp_worker_handle_task
func go_frankenphp_worker_handle_task(threadIndex C.uintptr_t) C.go_string {
	thread := phpThreads[threadIndex]
	handler, _ := thread.handler.(*taskWorkerThread)
	thread.Unpin()
	thread.state.markAsWaiting(true)

	select {
	case task := <-handler.taskWorker.taskChan:
		handler.currentTask = task
		thread.state.markAsWaiting(false)
		return C.go_string{len: C.size_t(len(task.str)), data: thread.pinString(task.str)}
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
		worker = getTaskWorkerByName(C.GoStringN(name, C.int(nameLen)))
	} else {
		worker = taskWorkers[taskWorkerIndex]
	}

	if worker == nil {
		logger.Error("no task worker found to handle this task", "name", C.GoStringN(name, C.int(nameLen)))
		return C.bool(false)
	}

	// create a new task and lock it until the task is done
	task := &PendingTask{str: C.GoStringN(taskChar, C.int(taskLen))}
	task.done.Lock()

	// dispatch immediately if available (best performance)
	select {
	case taskWorkers[taskWorkerIndex].taskChan <- task:
		return C.bool(true)
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
