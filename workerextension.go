package frankenphp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

// EXPERIMENTAL: Worker allows you to register a worker where, instead of calling FrankenPHP handlers on
// frankenphp_handle_request(), the GetRequest method is called.
//
// You may provide an http.Request that will be conferred to the underlying worker script,
// or custom parameters that will be passed to frankenphp_handle_request().
//
// After the execution of frankenphp_handle_request(), the return value WorkerRequest.AfterFunc will be called,
// with the optional return value of the callback passed as parameter.
//
// A worker script with the provided Name and FileName will be registered, along with the provided
// configuration. You can also provide any environment variables that you want through Env.
//
// Name() and FileName() are only called once at startup, so register them in an init() function.
//
// Workers are designed to run indefinitely and will be gracefully shut down when FrankenPHP shuts down.
//
// Extension workers receive the lowest priority when determining thread allocations. If MinThreads cannot be
// allocated, then FrankenPHP will panic and provide this information to the user (who will need to allocate more
// total threads). Don't be greedy.
type Worker interface {
	// Name returns the worker name
	Name() string
	// FileName returns the PHP script filename
	FileName() string
	// Env returns the environment variables available in the worker script.
	Env() PreparedEnv
	// MinThreads returns the minimum number of threads to reserve from the FrankenPHP thread pool.
	// This number must be positive.
	MinThreads() int
	// OnReady is called when the worker is assigned to a thread and receives an opaque thread ID as parameter.
	// This is a time for setting up any per-thread resources.
	OnReady(threadId int)
	// OnShutdown is called when the worker is shutting down and receives an opaque thread ID as parameter.
	// This is a time for cleaning up any per-thread resources.
	OnShutdown(threadId int)
	// OnServerShutdown is called when FrankenPHP is shutting down.
	OnServerShutdown(threadId int)
	// GetRequest is called once at least one thread is ready.
	// The returned request will be passed to the worker script.
	GetRequest() *WorkerRequest
	// SendRequest sends a request to the worker script. The callback function of frankenphp_handle_request() will be called.
	SendRequest(r *WorkerRequest)
}

// EXPERIMENTAL: WorkerRequest represents a request to pass to a worker script.
type WorkerRequest struct {
	// Request is an optional HTTP request for your worker script to handle
	Request *http.Request
	// Response is an optional response writer that provides the output of the provided request, it must not be nil to access the request body
	Response http.ResponseWriter
	// CallbackParameters is an optional field that will be converted in PHP types or left as-is if it's an unsafe.Pointer and passed as parameter to the PHP callback
	CallbackParameters any
	// AfterFunc is an optional function that will be called after the request is processed with the original value, the return of the PHP callback, converted in Go types, is passed as parameter
	AfterFunc func(callbackReturn any)
}

var extensionWorkers = make(map[string]Worker)
var extensionWorkersMutex sync.Mutex

// EXPERIMENTAL: RegisterWorker registers a custom worker script.
func RegisterWorker(worker Worker) {
	extensionWorkersMutex.Lock()
	defer extensionWorkersMutex.Unlock()

	extensionWorkers[worker.Name()] = worker
}

// startWorker creates a pipe from a worker to the main worker.
func startWorker(w *worker, extensionWorker Worker, thread *phpThread) {
	for {
		rq := extensionWorker.GetRequest()

		var fc *frankenPHPContext
		if rq.Request == nil {
			fc = newFrankenPHPContext()
			fc.logger = logger
		} else {
			fr, err := NewRequestWithContext(rq.Request, WithOriginalRequest(rq.Request))
			if err != nil {
				logger.LogAttrs(context.Background(), slog.LevelError, "error creating request for external worker", slog.String("worker", w.name), slog.Int("thread", thread.threadIndex), slog.Any("error", err))
				continue
			}

			var ok bool
			if fc, ok = fromContext(fr.Context()); !ok {
				continue
			}
		}

		fc.worker = w

		fc.responseWriter = rq.Response
		fc.handlerParameters = rq.CallbackParameters

		// Queue the request and wait for completion if Done channel was provided
		logger.LogAttrs(context.Background(), slog.LevelInfo, "queue the external worker request", slog.String("worker", w.name), slog.Int("thread", thread.threadIndex))

		w.requestChan <- fc
		if rq.AfterFunc != nil {
			go func() {
				<-fc.done

				if rq.AfterFunc != nil {
					rq.AfterFunc(fc.handlerReturn)
				}
			}()
		}
	}
}

// EXPERIMENTAL: NewWorker creates a Worker instance to embed in a custom struct implementing the Worker interface.
// The returned instance may be sufficient on its own for simple use cases.
func NewWorker(name, fileName string, minThreads int, env PreparedEnv) Worker {
	return &defaultWorker{
		name:           name,
		fileName:       fileName,
		env:            env,
		minThreads:     minThreads,
		requestChan:    make(chan *WorkerRequest),
		activatedCount: atomic.Int32{},
		drainCount:     atomic.Int32{},
	}
}

type defaultWorker struct {
	name           string
	fileName       string
	env            PreparedEnv
	minThreads     int
	requestChan    chan *WorkerRequest
	activatedCount atomic.Int32
	drainCount     atomic.Int32
}

func (w *defaultWorker) Name() string {
	return w.name
}

func (w *defaultWorker) FileName() string {
	return w.fileName
}

func (w *defaultWorker) Env() PreparedEnv {
	return w.env
}

func (w *defaultWorker) MinThreads() int {
	return w.minThreads
}

func (w *defaultWorker) OnReady(_ int) {
	w.activatedCount.Add(1)
}

func (w *defaultWorker) OnShutdown(_ int) {
	w.drainCount.Add(1)
}

func (w *defaultWorker) OnServerShutdown(_ int) {
	w.drainCount.Add(-1)
	w.activatedCount.Add(-1)
}

func (w *defaultWorker) GetRequest() *WorkerRequest {
	return <-w.requestChan
}

func (w *defaultWorker) SendRequest(r *WorkerRequest) {
	w.requestChan <- r
}
