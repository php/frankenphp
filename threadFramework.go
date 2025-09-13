package frankenphp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
)

// WorkerExtension allows you to register an external worker where instead of calling frankenphp handlers on
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
type WorkerExtension interface {
	Name() string
	FileName() string
	Env() PreparedEnv
	GetMinThreads() int
	ThreadActivatedNotification(threadId int)
	ThreadDrainNotification(threadId int)
	ThreadDeactivatedNotification(threadId int)
	ProvideRequest() *WorkerRequest
}

type WorkerRequest struct {
	// The request for your worker script to handle
	Request *http.Request
	// Response is a response writer that provides the output of the provided request
	Response http.ResponseWriter
	// Done is an optional channel that will be closed when the request processing is complete
	Done chan struct{}
}

var externalWorkers = make(map[string]WorkerExtension)
var externalWorkerMutex sync.Mutex

func RegisterExternalWorker(worker WorkerExtension) {
	externalWorkerMutex.Lock()
	defer externalWorkerMutex.Unlock()

	externalWorkers[worker.Name()] = worker
}

// startExternalWorkerPipe creates a pipe from an external worker to the main worker.
func startExternalWorkerPipe(w *worker, externalWorker WorkerExtension) {
	for {
		rq := externalWorker.ProvideRequest()

		if rq == nil || rq.Request == nil {
			logger.LogAttrs(context.Background(), slog.LevelWarn, "external worker provided nil request", slog.String("worker", w.name))
			continue
		}

		r := rq.Request
		fr, err := NewRequestWithContext(r, WithOriginalRequest(r), WithWorkerName(w.name))
		if err != nil {
			logger.LogAttrs(context.Background(), slog.LevelError, "error creating request for external worker", slog.String("worker", w.name), slog.Any("error", err))
			continue
		}

		if fc, ok := fromContext(fr.Context()); ok {
			fc.responseWriter = rq.Response

			// Queue the request and wait for completion if Done channel was provided
			logger.LogAttrs(context.Background(), slog.LevelInfo, "queue the external worker request", slog.String("worker", w.name))

			w.requestChan <- fc
			if rq.Done != nil {
				go func() {
					<-fc.done
					close(rq.Done)
				}()
			}
		}
	}
}
