package frankenphp

import (
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
// ProvideBackPressure allows you to autoscale your threads from the free threads in frankenphp's thread pool. These
// methods are only called once at startup, so register them in an init() function.
//
// When a thread is activated and nearly ready, ThreadActivatedNotification will be called with an opaque threadId;
// this is a time for setting up any per-thread resources. When a thread is about to be returned to the thread pool,
// you will receive a call to ThreadDrainNotification that will inform you of the threadId.
// After the thread is returned to the thread pool, ThreadDeactivatedNotification will be called.
//
// Once you have at least one thread activated, you will receive calls to ProvideRequest where you should respond with
// a request.
//
// Note: External workers receive the lowest priority when determining thread allocations. If GetMinThreads cannot be
// allocated, then frankenphp will panic and provide this information to the user (who will need to allocation more
// total threads). Don't be greedy. Use ProvideBackPressure to indicate when you receive a request to trigger
// autoscaling.
type WorkerExtension interface {
	Name() string
	FileName() string
	Env() PreparedEnv
	GetMinThreads() int
	ThreadActivatedNotification(threadId int)
	ThreadDrainNotification(threadId int)
	ThreadDeactivatedNotification(threadId int)
	ProvideRequest() <-chan *http.Request
}

var externalWorkers = make(map[string]WorkerExtension)
var externalWorkerMutex sync.Mutex

func RegisterExternalWorker(worker WorkerExtension) {
	externalWorkerMutex.Lock()
	defer externalWorkerMutex.Unlock()

	externalWorkers[worker.Name()] = worker
}
