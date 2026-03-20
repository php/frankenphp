package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// drainPendingTasks closes all queued tasks so waiting senders get EOF.
// Called when the background worker exits.
func (s *backgroundWorkerState) drainPendingTasks() {
	for {
		select {
		case task := <-s.tasks:
			task.crashDrain()
		default:
			return
		}
	}
}

// taskRequest represents a task sent from an HTTP worker to a background worker.
type taskRequest struct {
	id          int            // registered once in task_send, shared by both sides
	payload     unsafe.Pointer // persistent C HashTable, owned by sender
	fifo        *taskFIFO      // bounded FIFO for results/progress
	pipeFds     [2]int         // [0]=read (sender), [1]=write (background worker)
	cancelled   atomic.Bool    // set when sender cancels before background worker picks up
	closedSides atomic.Int32   // 0->1 by first closer, 1->2 triggers freeTask
}

// retire increments closedSides. When both sides have closed, frees the task slot.
func (t *taskRequest) retire() {
	if t.closedSides.Add(1) == 2 {
		freeTask(t.id)
	}
}

// closeBackgroundWorker is called when the background worker fcloses the task stream (clean completion).
func (t *taskRequest) closeBackgroundWorker() {
	t.fifo.close()
	C.frankenphp_worker_close_fd(C.int(t.pipeFds[1]))
	t.retire()
}

// abortBackgroundWorker is called when the background worker exits without closing the task stream.
func (t *taskRequest) abortBackgroundWorker() {
	t.fifo.abortClose()
	C.frankenphp_worker_close_fd(C.int(t.pipeFds[1]))
	t.retire()
}

// closeSender is called when the sender fcloses the task stream.
func (t *taskRequest) closeSender() {
	t.cancelled.Store(true)
	t.fifo.drainAndFree()
	t.retire()
}

// cancelBeforeDelivery is called in task_receive when a cancelled task is dequeued.
func (t *taskRequest) cancelBeforeDelivery() {
	C.frankenphp_worker_close_fd(C.int(t.pipeFds[1]))
	C.frankenphp_worker_free_persistent_ht(t.payload)
	freeTask(t.id)
}

// crashDrain is called by drainPendingTasks for queued tasks when the background worker exits.
func (t *taskRequest) crashDrain() {
	t.fifo.abortClose()
	C.frankenphp_worker_close_fd(C.int(t.pipeFds[1]))
	C.frankenphp_worker_free_persistent_ht(t.payload)
	t.retire()
}

// taskFIFO is a bounded FIFO queue for task updates with backpressure.
type taskFIFO struct {
	items    []unsafe.Pointer // persistent C HashTables
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	max      int
	closed   bool
	aborted  bool // true if closed because the background worker exited without fclose
}

func newTaskFIFO(max int) *taskFIFO {
	f := &taskFIFO{max: max}
	f.notFull = sync.NewCond(&f.mu)
	f.notEmpty = sync.NewCond(&f.mu)
	return f
}

func (f *taskFIFO) push(item unsafe.Pointer) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for len(f.items) >= f.max && !f.closed {
		f.notFull.Wait()
	}
	if f.closed {
		return false
	}
	f.items = append(f.items, item)
	f.notEmpty.Signal()
	return true
}

// pop returns (item, true, false) on success, (nil, false, false) on clean close,
// or (nil, false, true) if the FIFO was aborted (background worker exited without fclose).
func (f *taskFIFO) pop() (unsafe.Pointer, bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for len(f.items) == 0 && !f.closed {
		f.notEmpty.Wait()
	}
	if len(f.items) == 0 {
		return nil, false, f.aborted
	}
	item := f.items[0]
	f.items = f.items[1:]
	f.notFull.Signal()
	return item, true, false
}

func (f *taskFIFO) close() {
	f.mu.Lock()
	f.closed = true
	f.notFull.Broadcast()
	f.notEmpty.Broadcast()
	f.mu.Unlock()
}

func (f *taskFIFO) abortClose() {
	f.mu.Lock()
	f.closed = true
	f.aborted = true
	f.notFull.Broadcast()
	f.notEmpty.Broadcast()
	f.mu.Unlock()
}

// drainAndFree closes the FIFO and frees all remaining persistent items.
func (f *taskFIFO) drainAndFree() {
	f.close()
	f.mu.Lock()
	items := f.items
	f.items = nil
	f.mu.Unlock()
	for _, item := range items {
		C.frankenphp_worker_free_persistent_ht(item)
	}
}

// Task management: tasks are identified by an opaque ID (index into a global slice).
// This avoids the problem of mapping file descriptors back to task state.
var (
	tasksMu    sync.Mutex
	tasksSlice []*taskRequest
	tasksFree  []int
)

func registerTask(t *taskRequest) int {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	if len(tasksFree) > 0 {
		id := tasksFree[len(tasksFree)-1]
		tasksFree = tasksFree[:len(tasksFree)-1]
		tasksSlice[id] = t
		return id
	}
	tasksSlice = append(tasksSlice, t)
	return len(tasksSlice) - 1
}

func getTask(id int) *taskRequest {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	if id < 0 || id >= len(tasksSlice) {
		return nil
	}
	return tasksSlice[id]
}

func freeTask(id int) {
	tasksMu.Lock()
	defer tasksMu.Unlock()
	if id >= 0 && id < len(tasksSlice) {
		tasksSlice[id] = nil
		tasksFree = append(tasksFree, id)
	}
}

//export go_frankenphp_worker_task_send
func go_frankenphp_worker_task_send(threadIndex C.uintptr_t, name *C.char, nameLen C.size_t, payload unsafe.Pointer, timeoutMs C.int, outTaskId *C.int, outReadFd *C.int) *C.char {
	bgWorkerName := C.GoStringN(name, C.int(nameLen))

	thread := phpThreads[threadIndex]
	if err := startBackgroundWorker(thread, bgWorkerName); err != nil {
		return C.CString(err.Error())
	}

	lookup := getLookup(thread)
	if lookup == nil {
		return C.CString("no background worker configured in this php_server")
	}

	registry := lookup.Resolve(bgWorkerName)
	if registry == nil {
		return C.CString("background worker not found: " + bgWorkerName)
	}

	registry.mu.Lock()
	sk := registry.workers[bgWorkerName]
	registry.mu.Unlock()
	if sk == nil {
		return C.CString("background worker not found: " + bgWorkerName)
	}

	// Create pipe for result stream (C helper handles platform differences)
	var pipeFds [2]C.int
	pipeFds[0] = -1
	pipeFds[1] = -1
	C.frankenphp_worker_create_pipe(&pipeFds[0])
	if pipeFds[0] < 0 {
		return C.CString("failed to create task pipe")
	}

	task := &taskRequest{
		payload: payload,
		fifo:    newTaskFIFO(16),
		pipeFds: [2]int{int(pipeFds[0]), int(pipeFds[1])},
	}

	taskId := registerTask(task)
	task.id = taskId

	// Block until the task is enqueued (buffer=1 provides backpressure),
	// the worker dies, or the timeout expires. The sender blocks here if
	// the previous task hasn't been picked up by task_receive yet.
	timeout := time.Duration(timeoutMs) * time.Millisecond
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case sk.tasks <- task:
		// enqueued - now signal the receiver via "task\n" on the signaling stream
	case <-sk.dead:
		freeTask(taskId)
		C.frankenphp_worker_close_fd(pipeFds[0])
		C.frankenphp_worker_close_fd(pipeFds[1])
		return C.CString(fmt.Sprintf("background worker %q exited before receiving the task", bgWorkerName))
	case <-timer.C:
		freeTask(taskId)
		C.frankenphp_worker_close_fd(pipeFds[0])
		C.frankenphp_worker_close_fd(pipeFds[1])
		return C.CString(fmt.Sprintf("timeout waiting for background worker %q to accept task", bgWorkerName))
	}

	// Signal all background worker threads via "task\n" - wakes up stream_select
	if sk.fds != nil {
		sk.fds.writeAll(func(fd int32) {
			C.frankenphp_worker_write_task_signal(C.int(fd))
		})
	}

	*outTaskId = C.int(taskId)
	*outReadFd = pipeFds[0]

	return nil
}

//export go_frankenphp_worker_task_receive
func go_frankenphp_worker_task_receive(threadIndex C.uintptr_t, outPayload *unsafe.Pointer, outTaskId *C.int) C.int {
	thread := phpThreads[threadIndex]
	handler, ok := thread.handler.(*backgroundWorkerThread)
	if !ok || handler.worker.backgroundWorker == nil {
		return -1 // error: not a background worker
	}

	sk := handler.worker.backgroundWorker

	// Non-blocking: return immediately if no task available.
	// The PHP side uses stream_select on the signaling stream to wait for
	// "task\n", then calls task_receive. Returns false on spurious signals.
	select {
	case task, ok := <-sk.tasks:
		if !ok {
			return 0 // channel closed, shutting down
		}
		if task.cancelled.Load() {
			task.cancelBeforeDelivery()
			return 0 // cancelled before delivery
		}
		handler.currentTask = task
		*outPayload = task.payload
		*outTaskId = C.int(task.id)
		return 1 // success
	default:
		return 0 // no task available (spurious signal or fan-out contention)
	}
}

//export go_frankenphp_worker_task_update
func go_frankenphp_worker_task_update(taskId C.int, data unsafe.Pointer) *C.char {
	task := getTask(int(taskId))
	if task == nil {
		return C.CString("invalid task ID")
	}

	if !task.fifo.push(data) {
		return C.CString("task closed")
	}

	// Write 1 byte to pipe to wake up sender's stream_select
	C.frankenphp_worker_pipe_nudge(C.int(task.pipeFds[1]))

	return nil
}

// go_frankenphp_worker_task_close is called when the background worker's task stream is closed.
// This fires on both explicit fclose() and implicit resource cleanup during script exit.
// Marks the task as cleanly closed.
//
//export go_frankenphp_worker_task_close
func go_frankenphp_worker_task_close(threadIndex C.uintptr_t, taskId C.int) {
	task := getTask(int(taskId))
	if task == nil {
		return
	}

	task.closeBackgroundWorker()
}

// go_frankenphp_worker_task_cancel is called when the sender fcloses the task stream.
// Drains the FIFO and retires the sender side.
//
//export go_frankenphp_worker_task_cancel
func go_frankenphp_worker_task_cancel(taskId C.int) {
	task := getTask(int(taskId))
	if task == nil {
		return
	}

	task.closeSender()
}

//export go_frankenphp_worker_task_read
func go_frankenphp_worker_task_read(taskId C.int, outData *unsafe.Pointer) C.int {
	task := getTask(int(taskId))
	if task == nil {
		return -1
	}

	// If outData is nil, caller just wants to drain remaining items (pipe EOF path).
	// Don't retire here - task_cancel (stream close handler) is the sole sender-side retire.
	if outData == nil {
		task.fifo.drainAndFree()
		if task.fifo.aborted {
			return -2 // background worker exited without closing the task
		}
		return -1
	}

	data, ok, aborted := task.fifo.pop()
	if !ok {
		if aborted {
			return -2 // background worker exited without closing the task
		}
		return -1 // FIFO closed and empty - clean EOF
	}

	*outData = data
	return 0
}
