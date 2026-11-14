package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"runtime/cgo"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// taskLinger is how often the watcher of a queue looks at what waits in it,
// see watchTasks(): a pickup takes microseconds
const taskLinger = 10 * time.Millisecond

// workerTask is a unit of work handed by a PHP thread to a thread of a
// background worker, see SentTaskHandle. The payload and the updates flowing
// back are persistent HashTables, copied into request memory on arrival. Each
// side waits on the mailbox of its thread, or on its descriptor of the task's
// channel once it asked for one, and is woken there by the other.
type workerTask struct {
	handle cgo.Handle
	worker *worker
	// the thread of the sender, woken through its mailbox while it waits without
	// a descriptor
	sender    C.uintptr_t
	namespace string
	payload   *C.HashTable // owned by the task until a thread picks it up
	pickedUp  atomic.Bool  // a thread picked the task up
	// what the watcher of the queue refuses a waiting task on, see
	// watchTasks(): the time its sender gave a thread to pick it up, none when
	// zero, the drain of the sender's thread and the shutdown, both read on that
	// thread at send time
	deadline            time.Time
	drainChan, shutdown <-chan struct{}
	// the watcher of the queue found the task waiting already; only touched by it
	lingers bool
	// set by the thread that picked the task up, read by its close
	receiver   *backgroundWorkerThread
	pickedUpAt time.Time

	mu sync.Mutex
	// why the task ended without being completed, for the sender's read() to
	// throw: no thread will pick it up, see abort(), or the worker let go of it
	failure string
	// the sender sleeps in read(), on its mailbox, or is about to: the next
	// event wakes it there, once
	senderParked bool
	// the sender's descriptor is readable, once it took it: nothing tells when
	// it waits there, a poller may keep it for good, so it is readable for as
	// long as read() has something to take, see senderReadableLocked()
	senderReadable bool
	// fds[0] is the sender's descriptor, fds[1] the receiver's: the pair is made
	// when a side first hands its own to a stream_select() or a poll context,
	// and that side is signaled there from then on. The task owns the pair and
	// closes it once both sides are done, never before: a poller may still
	// hold a descriptor of a task its side ended, and the kernel only takes it
	// off that poller when it is closed
	fds                      [2]int64
	chanOpen                 bool
	senderOnFd, receiverOnFd bool
	updates                  []*C.HashTable
	closed                   bool // the task is over for the sender to read: completed, or failed
	senderGone               bool // the sender closed its stream
	retired                  int  // sides done with the task, freed at 2
}

// taskQueue holds the tasks sent to a background worker until a thread picks
// them up: as many as their senders hold handles on, nothing else bounds it.
// Its mutex also guards the stop sockets of the worker's threads: senders
// write the wake-up line to them, so they must not be closed meanwhile.
type taskQueue struct {
	mu      sync.Mutex
	pending []*workerTask
	next    int // thread to signal first, spreads tasks over a pool
	// the watcher runs, see watchTasks()
	watching bool
}

// remove takes t out of the queue, false if a thread picked it up already
func (q *taskQueue) remove(t *workerTask) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	i := slices.Index(q.pending, t)
	if i < 0 {
		return false
	}
	q.pending = slices.Delete(q.pending, i, i+1)

	return true
}

// claimParkedThread picks one parked thread of the worker, round-robin over
// the pool, and returns its stop socket to write the wake-up line to, -1 for a
// thread to wake through its mailbox, or no thread when none is parked: the
// task then waits in the queue for a thread to drain it or to park, see
// go_frankenphp_background_worker_park. Called with tasks.mu held, the caller
// wakes the thread after releasing it, see signalThreads
func (worker *worker) claimParkedThread() (*backgroundWorkerThread, int64) {
	worker.threadMutex.RLock()
	defer worker.threadMutex.RUnlock()

	n := len(worker.threads)
	for i := range n {
		thread := worker.threads[(worker.tasks.next+i)%n]
		if handler, ok := thread.handler.(*backgroundWorkerThread); ok && handler.parked && handler.stopSock >= 0 {
			handler.parked = false
			worker.tasks.next = (worker.tasks.next + i + 1) % n
			// drain() waits for the wake-up, whichever way it goes
			handler.signaling.Add(1)
			if handler.onMailbox {
				handler.onMailbox = false

				return handler, -1
			}

			return handler, handler.stopSock
		}
	}

	return nil, -1
}

// claimAllThreads is the fallback of watchTasks(): every thread of the worker
// gets the line, parked or not, or a wake-up on its mailbox when it sleeps
// there. Called with tasks.mu held, the caller wakes the threads after
// releasing it
func (worker *worker) claimAllThreads() (handlers []*backgroundWorkerThread, socks []int64) {
	worker.threadMutex.RLock()
	for _, thread := range worker.threads {
		if handler, ok := thread.handler.(*backgroundWorkerThread); ok && handler.stopSock >= 0 {
			handlers = append(handlers, handler)
			handler.signaling.Add(1)
			if handler.parked && handler.onMailbox {
				socks = append(socks, -1)
			} else {
				socks = append(socks, handler.stopSock)
			}
			handler.parked, handler.onMailbox = false, false
		}
	}
	worker.threadMutex.RUnlock()

	return handlers, socks
}

// signalThreads wakes the threads claimed under tasks.mu, after it was
// released: the wake-up is a syscall, and a thread contending for the mutex
// meanwhile would park at the price of a scheduler hand-off
func signalThreads(handlers []*backgroundWorkerThread, socks []int64) {
	for i, s := range socks {
		if s < 0 {
			C.frankenphp_mailbox_wake(C.uintptr_t(handlers[i].thread.threadIndex))
		} else {
			C.frankenphp_worker_signal_task(C.intptr_t(s))
		}
		handlers[i].signaling.Add(-1)
	}
}

// notifySenderLocked tells the sender that read() has something for it: an
// update, or the end of the task. On its descriptor once it took it; true
// when it sleeps on its mailbox instead, for the caller to wake it there.
// Called with mu held
func (t *workerTask) notifySenderLocked() bool {
	if t.senderOnFd {
		t.senderReadableLocked(true)

		return false
	}
	parked := t.senderParked
	t.senderParked = false

	return parked
}

// senderReadableLocked makes the sender's descriptor readable, or no longer.
// With mu held from the change of what read() has to take to the write or
// the read on the descriptor, so the two never part
func (t *workerTask) senderReadableLocked(readable bool) {
	if !t.senderOnFd || t.senderReadable == readable {
		return
	}
	t.senderReadable = readable
	if readable {
		C.frankenphp_task_chan_signal(C.intptr_t(t.fds[0]), C.intptr_t(t.fds[1]), 0)
	} else {
		C.frankenphp_task_chan_consume(C.intptr_t(t.fds[0]))
	}
}

// signalReceiver wakes the receiver's stream_select(): the sender is gone
func (t *workerTask) signalReceiver() {
	C.frankenphp_task_chan_signal(C.intptr_t(t.fds[0]), C.intptr_t(t.fds[1]), 1)
}

// retire counts a side done with the task; the last one frees it
func (t *workerTask) retire() {
	t.mu.Lock()
	t.retired++
	last := t.retired == 2
	t.mu.Unlock()

	if last {
		t.free()
	}
}

// free releases what the task still holds once both sides are done with it:
// its payload went to the thread that picked it up, or was freed when it left
// the queue, and the updates nobody read went with the sender
func (t *workerTask) free() {
	if t.chanOpen {
		C.frankenphp_close_sock(C.intptr_t(t.fds[0]))
		C.frankenphp_close_sock(C.intptr_t(t.fds[1]))
	}
	t.handle.Delete()
}

// the receiver of each task namespace, per scope
var taskOwners namespaceOwners

// taskOwner resolves the receiver of a task namespace: within the sender's
// server first, then among global workers
func taskOwner(thread *phpThread, ns string) *worker {
	return taskOwners.resolve(namespaceScope(thread), ns)
}

//export go_frankenphp_send_task
func go_frankenphp_send_task(threadIndex C.uintptr_t, namespace *C.char, namespaceLen C.size_t, payload *C.HashTable, pickupMs C.int) (C.uintptr_t, *C.char, C.bool) {
	thread := phpThreads[threadIndex]
	// looked up first, and only kept once it is known
	ns := unsafe.String((*byte)(unsafe.Pointer(namespace)), int(namespaceLen))
	w := taskOwner(thread, ns)
	if w == nil {
		C.frankenphp_vars_free(payload)

		// what the configuration lacks, not a misuse: see varsOwner
		return 0, C.CString("FrankenPHP\\SentTaskHandle: no background worker receives the task namespace " + quoteNamespace(ns)), false
	}
	ns = strings.Clone(ns)
	if handler, ok := thread.handler.(*backgroundWorkerThread); ok && handler.worker == w && w.countThreads() == 1 {
		C.frankenphp_vars_free(payload)

		return 0, C.CString("FrankenPHP\\SentTaskHandle: the task namespace " + strconv.Quote(ns) + " is received by this very worker, which has a single thread"), true
	}

	t := &workerTask{
		worker:    w,
		sender:    threadIndex,
		namespace: ns,
		payload:   payload,
		fds:       [2]int64{-1, -1},
		// the target's threads are drained too, nobody would pick the task up.
		// Read here, on the PHP thread: a goroutine may only get to run after
		// Shutdown() replaced them
		drainChan: thread.drainChan,
		shutdown:  mainThread.done,
	}
	if pickupMs >= 0 {
		t.deadline = time.Now().Add(time.Duration(pickupMs) * time.Millisecond)
	}
	t.handle = cgo.NewHandle(t)

	// queued like a request would be: a background worker has no other queue
	metrics.QueuedWorkerRequest(w.name)
	q := &w.tasks
	q.mu.Lock()
	q.pending = append(q.pending, t)
	handler, sock := w.claimParkedThread()
	watch := !q.watching
	q.watching = true
	q.mu.Unlock()
	if handler != nil {
		signalThreads([]*backgroundWorkerThread{handler}, []int64{sock})
	}
	if watch {
		go w.watchTasks()
	}

	// the sender goes on: its read() waits for the pickup along with the first
	// update, on the mailbox of its thread rather than in a Go select, since
	// waking a thread parked inside a Go callback costs the scheduler a
	// hand-off and a wake-up in the kernel does not

	return C.uintptr_t(t.handle), nil, false
}

// watchTasks looks after the tasks that wait in the queue of the worker, every
// taskLinger for as long as some do: nothing else watches a waiting task, its
// sender only sleeps, and a pickup within microseconds costs neither a thing.
// A task found waiting gets every thread of the worker woken, once, parked or
// not: the one its sender woke may be busy with something else. And a task is
// refused once no thread should pick it up anymore: the time its sender gave
// has passed, or, a look later so that a pickup on its way gets through,
// FrankenPHP shuts down or the thread of its sender is drained
func (worker *worker) watchTasks() {
	q := &worker.tasks
	var tasks []*workerTask
	for {
		time.Sleep(taskLinger)

		q.mu.Lock()
		if len(q.pending) == 0 {
			q.watching = false
			q.mu.Unlock()

			return
		}
		// looked at once the mutex is released: the queue may be long
		tasks = append(tasks[:0], q.pending...)
		q.mu.Unlock()

		now, lingering := time.Now(), false
		for _, t := range tasks {
			switch {
			case !t.deadline.IsZero() && !now.Before(t.deadline):
				t.abort("FrankenPHP\\SentTaskHandle: no thread picked up the task sent to "+strconv.Quote(t.namespace)+" in time", taskTimeout)
			case !t.lingers:
				t.lingers, lingering = true, true
			// the shutdown drains the threads too: it is the reason then
			case isClosed(t.shutdown):
				t.abort("FrankenPHP\\SentTaskHandle: FrankenPHP is shutting down", taskCancelled)
			case isClosed(t.drainChan):
				t.abort("FrankenPHP\\SentTaskHandle: the calling thread is restarting or shutting down", taskCancelled)
			}
		}

		if lingering {
			q.mu.Lock()
			handlers, socks := worker.claimAllThreads()
			q.mu.Unlock()
			signalThreads(handlers, socks)
		}
	}
}

func isClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// abort refuses a task still in the queue, which it leaves at once: no thread
// takes it from there on, so what the sender is told stands. Woken wherever it
// waits, the sender finds a task that failed for that reason. A thread that
// took the task first wins, and so does a sender that left
func (t *workerTask) abort(reason, outcome string) {
	if !t.withdraw(outcome, reason) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	// under the mutex, which the sender's close takes: its thread, and the
	// mailbox with it, are there for the wake-up
	if !t.senderGone && t.notifySenderLocked() {
		C.frankenphp_mailbox_wake(t.sender)
	}
}

// withdraw takes a task nobody picked up out of the queue and releases the
// receiver's side of it, the sender's close releases the rest; false when a
// thread got the task first. reason is why, for a sender that did not ask
func (t *workerTask) withdraw(outcome, reason string) bool {
	if !t.worker.tasks.remove(t) {
		return false
	}
	metrics.DequeuedWorkerRequest(t.worker.name)

	t.mu.Lock()
	// the first to end the task settles its outcome, and a sender that left
	// meanwhile did
	settled := !t.senderGone
	if settled {
		// set along with the end of the task: read() tells a refusal from a
		// completion
		t.failure = reason
	}
	// nothing for the sender's close to settle
	t.closed = true
	t.mu.Unlock()
	if settled {
		taskMetrics().WorkerTaskOutcome(t.worker.name, outcome)
	}

	C.frankenphp_vars_free(t.payload)
	t.payload = nil
	t.retire()

	return true
}

// go_frankenphp_task_fd hands a side its descriptor of the task's channel, as
// it is about to wait on it through stream_select() or a poll context: the
// pair is made then, and what the side would have found on it is signaled.
// -1 when the pair could not be made
//
//export go_frankenphp_task_fd
func go_frankenphp_task_fd(handle C.uintptr_t, sender C.bool) C.intptr_t {
	t := cgo.Handle(handle).Value().(*workerTask)

	t.mu.Lock()
	if !t.chanOpen {
		var fds [2]C.intptr_t
		if C.frankenphp_task_chan_open(&fds[0]) != 0 {
			t.mu.Unlock()

			return -1
		}
		t.fds, t.chanOpen = [2]int64{int64(fds[0]), int64(fds[1])}, true
	}
	side := 0
	if bool(sender) {
		// what is already there to read is on the descriptor from the start
		t.senderOnFd = true
		t.senderReadableLocked(t.eventLocked())
	} else {
		side = 1
		// a sender gone already is what the descriptor is there to tell
		if !t.receiverOnFd && t.senderGone && !t.closed {
			t.signalReceiver()
		}
		t.receiverOnFd = true
	}
	fd := t.fds[side]
	t.mu.Unlock()

	return C.intptr_t(fd)
}

// go_frankenphp_task_side_gone tells a stream whether the other side closed
// its own: what feof() reports on the task streams
//
//export go_frankenphp_task_side_gone
func go_frankenphp_task_side_gone(handle C.uintptr_t, sender C.bool) C.bool {
	t := cgo.Handle(handle).Value().(*workerTask)

	t.mu.Lock()
	defer t.mu.Unlock()
	if bool(sender) {
		// the end comes behind the updates still to read
		return C.bool(t.closed && len(t.updates) == 0)
	}

	return C.bool(t.senderGone)
}

// go_frankenphp_background_worker_park is called by WorkerHandle::tick() as the
// thread is about to wait, where says on what: its handle, when the script
// waits there next, or its mailbox, when tick() waits itself. The thread parks
// unless tasks are queued: a wait in tick() is over then, and a script about
// to wait on its handle gets a wake-up written there, so that its wait returns
// at once. Under tasks.mu, so a task queued after the check finds the thread
// parked and wakes it: no wake-up is lost either way. FRANKENPHP_PARK_LEAVE
// ends the park of a thread whose wait in tick() is over. A drained thread is
// told so, ahead of the end of its handle's stream.
//
//export go_frankenphp_background_worker_park
func go_frankenphp_background_worker_park(threadIndex C.uintptr_t, where C.int) C.int {
	handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread)
	if !ok {
		// refused on the C side already
		return C.FRANKENPHP_PARK_READY
	}

	q := &handler.worker.tasks
	q.mu.Lock()
	s := handler.stopSock
	if s < 0 {
		q.mu.Unlock()

		return C.FRANKENPHP_PARK_DRAINED
	}
	if where == C.FRANKENPHP_PARK_LEAVE {
		handler.parked, handler.onMailbox = false, false
		q.mu.Unlock()

		return C.FRANKENPHP_PARK_READY
	}
	if len(q.pending) == 0 {
		handler.parked, handler.onMailbox = true, where == C.FRANKENPHP_PARK_ON_MAILBOX
		q.mu.Unlock()

		return C.FRANKENPHP_PARK_PARKED
	}
	if where == C.FRANKENPHP_PARK_ON_MAILBOX {
		// awake, whatever an earlier tick() parked the thread on
		handler.parked, handler.onMailbox = false, false
		q.mu.Unlock()

		return C.FRANKENPHP_PARK_READY
	}
	// drain() waits for the write before closing the socket
	handler.signaling.Add(1)
	q.mu.Unlock()

	// the syscall happens outside of the mutex, see signalThreads
	C.frankenphp_worker_signal_task(C.intptr_t(s))
	handler.signaling.Add(-1)

	return C.FRANKENPHP_PARK_READY
}

// go_frankenphp_receive_task dequeues a task for the calling thread, none when
// the queue is empty. It never waits, WorkerHandle::tick() does, and a park
// set there stays: the script may be about to wait on its handle, where the
// next task has to wake it
//
//export go_frankenphp_receive_task
func go_frankenphp_receive_task(threadIndex C.uintptr_t) (C.uintptr_t, *C.HashTable, *C.char, C.size_t) {
	handler, ok := phpThreads[threadIndex].handler.(*backgroundWorkerThread)
	if !ok {
		// refused on the C side already
		return 0, nil, nil, 0
	}

	q := &handler.worker.tasks
	q.mu.Lock()
	if len(q.pending) == 0 {
		q.mu.Unlock()

		return 0, nil, nil, 0
	}
	t := q.pending[0]
	q.pending = slices.Delete(q.pending, 0, 1)
	// the payload moves to request memory on the C side
	payload := t.payload
	t.payload = nil
	q.mu.Unlock()
	metrics.DequeuedWorkerRequest(handler.worker.name)
	// nothing wakes the sender for a pickup: its read() waits for what the
	// thread has to say
	t.pickedUp.Store(true)

	t.receiver = handler
	t.pickedUpAt = time.Now()
	taskMetrics().StartWorkerTask(handler.worker.name)
	// busy on the threads endpoint while it holds a task
	if handler.openTasks++; handler.openTasks == 1 {
		handler.state.MarkAsWaiting(false)
	}

	// the namespace goes as C memory, for the C side to copy and free: request
	// memory is not allocated from here, see WorkerHandle::receive()
	return C.uintptr_t(t.handle), payload, C.CString(t.namespace), C.size_t(len(t.namespace))
}

// go_frankenphp_update_task queues an update for the sender, which reads them
// at its own pace: nothing bounds what waits for it. False when the sender is
// gone, the update is freed then
//
//export go_frankenphp_update_task
func go_frankenphp_update_task(handle C.uintptr_t, update *C.HashTable) C.bool {
	t := cgo.Handle(handle).Value().(*workerTask)

	t.mu.Lock()
	if t.senderGone {
		t.mu.Unlock()
		C.frankenphp_vars_free(update)

		return false
	}
	t.updates = append(t.updates, update)
	wake := t.notifySenderLocked()
	t.mu.Unlock()
	if wake {
		C.frankenphp_mailbox_wake(t.sender)
	}

	return true
}

// go_frankenphp_read_task hands the sender its next event; with nothing to
// hand, the sender parks and the next event wakes it. A task that failed is
// over with the reason why, for the C side to throw and free
//
//export go_frankenphp_read_task
func go_frankenphp_read_task(handle C.uintptr_t) (*C.HashTable, C.int, *C.char) {
	t := cgo.Handle(handle).Value().(*workerTask)

	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.updates) > 0 {
		update := t.updates[0]
		if len(t.updates) == 1 {
			// the array serves again
			t.updates = t.updates[:0]
		} else {
			// not moved: the sender may be far behind
			t.updates = t.updates[1:]
		}
		t.senderParked = false
		// the end of the task comes behind its updates
		t.senderReadableLocked(len(t.updates) > 0 || t.closed)

		return update, C.int(C.FRANKENPHP_TASK_READ_UPDATE), nil
	}

	status := C.int(C.FRANKENPHP_TASK_READ_PENDING)
	var failure *C.char
	if t.closed {
		status = C.FRANKENPHP_TASK_READ_OVER
		if t.failure != "" {
			failure = C.CString(t.failure)
		}
	}
	// nothing is left to read, the end of the task is only told once there
	t.senderReadableLocked(false)
	t.senderParked = !t.closed

	return nil, status, failure
}

// eventLocked tells whether read() has something for the sender right now: an
// update or the end of the task. Called with mu held
func (t *workerTask) eventLocked() bool {
	return len(t.updates) > 0 || t.closed
}

// go_frankenphp_task_receiver_close ends the task on the receiver's side, and
// tells whether the sender was still there to learn it. The result of a task
// being completed is queued with its end, so the sender never finds one
// without the other; a result that goes nowhere makes it an abort, the sender
// must not take the updates it got for the whole of it
//
//export go_frankenphp_task_receiver_close
func go_frankenphp_task_receiver_close(handle C.uintptr_t, abort C.bool, result *C.HashTable) C.bool {
	t := cgo.Handle(handle).Value().(*workerTask)
	aborted := bool(abort)

	t.mu.Lock()
	if result != nil {
		if !aborted && !t.senderGone {
			t.updates = append(t.updates, result)
			result = nil
		} else {
			aborted = true
		}
	}
	t.closed = true
	if aborted {
		t.failure = "FrankenPHP\\SentTaskHandle::read(): the background worker let go of the task without completing it"
	}
	// the first side to close settles the outcome
	settled := !t.senderGone
	// the sender finds the end of the task behind the updates still queued
	wake := settled && t.notifySenderLocked()
	t.mu.Unlock()
	if wake {
		C.frankenphp_mailbox_wake(t.sender)
	}
	dropped := result != nil
	if dropped {
		C.frankenphp_vars_free(result)
	}

	m := taskMetrics()
	m.StopWorkerTask(t.worker.name, time.Since(t.pickedUpAt))
	if settled {
		outcome := taskCompleted
		if aborted {
			outcome = taskAborted
		}
		m.WorkerTaskOutcome(t.worker.name, outcome)
	}
	handler := t.receiver
	handler.openTasks--
	if handler.openTasks == 0 && !handler.isBootingScript {
		handler.state.MarkAsWaiting(true)
	}

	t.retire()

	return C.bool(settled && !dropped)
}

//export go_frankenphp_task_sender_close
func go_frankenphp_task_sender_close(handle C.uintptr_t) {
	t := cgo.Handle(handle).Value().(*workerTask)

	if !t.pickedUp.Load() {
		// no thread took the task yet: it leaves the queue with its sender,
		// unless it was refused and left it already
		t.withdraw(taskAbandoned, "")
	}

	t.mu.Lock()
	t.senderGone = true
	t.senderParked = false
	// a poller left with the descriptor of a task its side ended stays quiet
	t.senderReadableLocked(false)
	// the first side to close settles the outcome
	settled := !t.closed
	// the receiver's stream_select() and feof() see it; once the receiver
	// closed, nobody waits on its descriptor
	signal := settled && t.receiverOnFd
	updates := t.updates
	t.updates = nil
	t.mu.Unlock()

	if signal {
		t.signalReceiver()
	}
	if settled {
		taskMetrics().WorkerTaskOutcome(t.worker.name, taskAbandoned)
	}
	for _, update := range updates {
		C.frankenphp_vars_free(update)
	}
	t.retire()
}
