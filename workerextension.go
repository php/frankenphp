package frankenphp

import (
	"net/http"
)

// EXPERIMENTAL: Workers allows you to register a worker.
type Workers interface {
	// SendRequest calls the closure passed to frankenphp_handle_request() and update the context.
	// The generated HTTP response will be written through the provided writer
	SendRequest(rw http.ResponseWriter, r *http.Request) error
	// SendMessage calls the closure passed to frankenphp_handle_request() and pass message as parameter, if the value returned by the closure is returned by the function.
	SendMessage(message any, rw http.ResponseWriter) any
	// NumThreads returns the number of available threads
	NumThreads() int
}

type extensionWorkers struct {
	name           string
	fileName       string
	num            int
	options        []WorkerOption
	internalWorker *worker
}

// EXPERIMENTAL: SendRequest sends an HTTP request to the worker and writes the response to the provided ResponseWriter.
func (w *extensionWorkers) SendRequest(rw http.ResponseWriter, r *http.Request) error {
	fr, err := NewRequestWithContext(
		r,
		WithOriginalRequest(r),
		WithWorkerName(w.name),
	)

	if err != nil {
		return err
	}

	return ServeHTTP(rw, fr)

}

func (w *extensionWorkers) NumThreads() int {
	return w.internalWorker.countThreads()
}

// EXPERIMENTAL: SendMessage sends a message to the worker and waits for a response.
func (w *extensionWorkers) SendMessage(message any, rw http.ResponseWriter) any {
	fc := newFrankenPHPContext()
	fc.logger = logger
	fc.worker = w.internalWorker
	fc.responseWriter = rw
	fc.handlerParameters = message

	w.internalWorker.handleRequest(fc)

	return fc.handlerReturn
}
