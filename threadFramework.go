package frankenphp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

// EXPERIMENTAL: WorkerExtensionInterface allows you to register an external worker where instead of calling frankenphp handlers on
// frankenphp_handle_request(), the ProvideRequest method is called. You are responsible for providing a standard
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
type WorkerExtensionInterface interface {
	Name() string
	FileName() string
	Env() PreparedEnv
	GetMinThreads() int
	ThreadActivatedNotification(threadId int)
	ThreadDrainNotification(threadId int)
	ThreadDeactivatedNotification(threadId int)
	ProvideRequest() *WorkerRequest[any, any]
}

// EXPERIMENTAL
type WorkerRequest[P any, R any] struct {
	// The request for your worker script to handle
	Request *http.Request
	// Response is a response writer that provides the output of the provided request, it must not be nil to access the request body
	Response http.ResponseWriter
	// CallbackParameters is an optional field that will be converted in PHP types and passed as parameter to the PHP callback
	CallbackParameters P
	// AfterFunc is an optional function that will be called after the request is processed with the original value, the return of the PHP callback, converted in Go types, is passed as parameter
	AfterFunc func(callbackReturn R)
}

var externalWorkers = make(map[string]WorkerExtensionInterface)
var externalWorkerMutex sync.Mutex

// EXPERIMENTAL
func RegisterExternalWorker(worker WorkerExtensionInterface) {
	externalWorkerMutex.Lock()
	defer externalWorkerMutex.Unlock()

	externalWorkers[worker.Name()] = worker
}

// startExternalWorkerPipe creates a pipe from an external worker to the main worker.
func startExternalWorkerPipe(w *worker, externalWorker WorkerExtensionInterface, thread *phpThread) {
	for {
		rq := externalWorker.ProvideRequest()

		if rq == nil || rq.Request == nil {
			logger.LogAttrs(context.Background(), slog.LevelWarn, "external worker provided nil request", slog.String("worker", w.name), slog.Int("thread", thread.threadIndex))
			continue
		}

		r := rq.Request
		fr, err := NewRequestWithContext(r, WithOriginalRequest(r), WithWorkerName(w.name))
		if err != nil {
			logger.LogAttrs(context.Background(), slog.LevelError, "error creating request for external worker", slog.String("worker", w.name), slog.Int("thread", thread.threadIndex), slog.Any("error", err))
			continue
		}

		if fc, ok := fromContext(fr.Context()); ok {
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
}

type WorkerExtension struct {
	ExtensionName  string
	WorkerFileName string
	WorkerEnv      PreparedEnv
	MinThreads     int
	RequestChan    chan *WorkerRequest[any, any]
	ActivatedCount atomic.Int32
	DrainCount     atomic.Int32
}

func (w *WorkerExtension) Name() string {
	return w.ExtensionName
}

func (w *WorkerExtension) FileName() string {
	return w.WorkerFileName
}

func (w *WorkerExtension) Env() PreparedEnv {
	return w.WorkerEnv
}

func (w *WorkerExtension) GetMinThreads() int {
	return w.MinThreads
}

func (w *WorkerExtension) ThreadActivatedNotification(threadId int) {
	w.ActivatedCount.Add(1)
}

func (w *WorkerExtension) ThreadDrainNotification(threadId int) {
	w.DrainCount.Add(1)
}

func (w *WorkerExtension) ThreadDeactivatedNotification(threadId int) {
	w.DrainCount.Add(-1)
	w.ActivatedCount.Add(-1)
}

func (w *WorkerExtension) ProvideRequest() *WorkerRequest[any, any] {
	return <-w.RequestChan
}
