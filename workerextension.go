package frankenphp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

// EXPERIMENTAL: Worker allows you to register a worker where instead of calling FrankenPHP handlers on
// frankenphp_handle_request(), the ProvideRequest method is called. You may provide a standard
// http.Request that will be conferred to the underlying worker script.
//
// A worker script with the provided Name and FileName will be registered, along with the provided
// configuration. You can also provide any environment variables that you want through Env. GetMinThreads allows you to
// reserve a minimum number of threads from the frankenphp thread pool. This number must be positive.
// These methods are only called once at startup, so register them in an init() function.
//
// When a thread is activated and nearly ready, ThreadActivatedNotification will be called with an opaque threadId;
// this is a time for setting up any per-thread resources. When a thread is about to be returned to the thread pool,
// you will receive a call to ThreadDrainNotification that will inform you of the threadId.
// After the thread is returned to the thread pool, ThreadDeactivatedNotification will be called.
//
// Once you have at least one thread activated, you will receive calls to ProvideRequest where you should respond with
// a request. FrankenPHP will automatically pipe these requests to the worker script and handle the response.
// The piping process is designed to run indefinitely and will be gracefully shut down when FrankenPHP shuts down.
//
// Note: External workers receive the lowest priority when determining thread allocations. If GetMinThreads cannot be
// allocated, then frankenphp will panic and provide this information to the user (who will need to allocate more
// total threads). Don't be greedy.
type Worker interface {
	Name() string
	FileName() string
	Env() PreparedEnv
	GetMinThreads() int
	ThreadActivatedNotification(threadId int)
	ThreadDrainNotification(threadId int)
	ThreadDeactivatedNotification(threadId int)
	ProvideRequest() *WorkerRequest
	InjectRequest(r *WorkerRequest)
}

// EXPERIMENTAL
type WorkerRequest struct {
	// The request for your worker script to handle
	Request *http.Request
	// Response is a response writer that provides the output of the provided request, it must not be nil to access the request body
	Response http.ResponseWriter
	// CallbackParameters is an optional field that will be converted in PHP types and passed as parameter to the PHP callback
	CallbackParameters any
	// AfterFunc is an optional function that will be called after the request is processed with the original value, the return of the PHP callback, converted in Go types, is passed as parameter
	AfterFunc func(callbackReturn any)
}

var extensionWorkers = make(map[string]Worker)
var extensionWorkersMutex sync.Mutex

// EXPERIMENTAL
func RegisterWorker(worker Worker) {
	extensionWorkersMutex.Lock()
	defer extensionWorkersMutex.Unlock()

	extensionWorkers[worker.Name()] = worker
}

// startWorker creates a pipe from a worker to the main worker.
func startWorker(w *worker, extensionWorker Worker, thread *phpThread) {
	for {
		rq := extensionWorker.ProvideRequest()

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

// EXPERIMENTAL: NewWorker creates a new Worker instance.
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

func (w *defaultWorker) GetMinThreads() int {
	return w.minThreads
}

func (w *defaultWorker) ThreadActivatedNotification(_ int) {
	w.activatedCount.Add(1)
}

func (w *defaultWorker) ThreadDrainNotification(_ int) {
	w.drainCount.Add(1)
}

func (w *defaultWorker) ThreadDeactivatedNotification(_ int) {
	w.drainCount.Add(-1)
	w.activatedCount.Add(-1)
}

func (w *defaultWorker) ProvideRequest() *WorkerRequest {
	return <-w.requestChan
}

func (w *defaultWorker) InjectRequest(r *WorkerRequest) {
	w.requestChan <- r
}
