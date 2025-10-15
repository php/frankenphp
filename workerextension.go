package frankenphp

import (
	"errors"
	"net/http"
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
type Worker struct {
	Name     string
	FileName string
	Num      int
	options  []WorkerOption
}

var extensionWorkers = make(map[string]Worker)

// EXPERIMENTAL: RegisterWorker registers a custom worker script.
func RegisterWorker(worker Worker) {
	extensionWorkers[worker.Name] = worker
}

// EXPERIMENTAL: SendRequest sends an HTTP request to the worker and writes the response to the provided ResponseWriter.
func (w Worker) SendRequest(rw http.ResponseWriter, r *http.Request) error {
	worker := getWorkerByName(w.Name)

	if worker == nil {
		return errors.New("worker not found: " + w.Name)
	}

	fr, err := NewRequestWithContext(
		r,
		WithOriginalRequest(r),
		WithWorkerName(w.Name),
	)

	if err != nil {
		return err
	}

	err = ServeHTTP(rw, fr)

	if err != nil {
		return err
	}

	return nil
}

// EXPERIMENTAL: SendMessage sends a message to the worker and waits for a response.
func (w Worker) SendMessage(message any) (any, error) {
	internalWorker := getWorkerByName(w.Name)

	if internalWorker == nil {
		return nil, errors.New("worker not found: " + w.Name)
	}

	fc := newFrankenPHPContext()
	fc.logger = logger
	fc.worker = internalWorker
	fc.handlerParameters = message

	internalWorker.handleRequest(fc)

	return fc.handlerReturn, nil
}

// EXPERIMENTAL: NewWorker creates a Worker instance to embed in a custom struct implementing the Worker interface.
// The returned instance may be sufficient on its own for simple use cases.
func NewWorker(name string, fileName string, num int, options ...WorkerOption) Worker {
	return Worker{
		Name:     name,
		FileName: fileName,
		Num:      num,
		options:  options,
	}
}
