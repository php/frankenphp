package frankenphp

import (
	"context"
	"net/http"
)

// EXPERIMENTAL: Workers allows you to register a worker.
type Workers interface {
	// SendRequest calls the closure passed to frankenphp_handle_request() and updates the PHP context .
	// The generated HTTP response will be written through the provided writer.
	SendRequest(rw http.ResponseWriter, r *http.Request) error
	// SendMessage calls the closure passed to frankenphp_handle_request(), passes message as a parameter, and returns the value produced by the closure.
	SendMessage(ctx context.Context, message any, rw http.ResponseWriter) (any, error)
	// NumThreads returns the number of available threads.
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
	// the worker only exists between Init() and Shutdown()
	if w.internalWorker == nil {
		return ErrNotRunning
	}

	opts := []RequestOption{WithOriginalRequest(r), WithWorkerName(w.name)}

	// worker names are resolved within a server, so a scoped worker is only
	// reachable through its own server
	if server := w.internalWorker.server; server != nil {
		return server.ServeHTTP(rw, r, opts...)
	}

	fr, err := NewRequestWithContext(r, opts...)
	if err != nil {
		return err
	}

	return ServeHTTP(rw, fr)
}

func (w *extensionWorkers) NumThreads() int {
	if w.internalWorker == nil {
		return 0
	}

	return w.internalWorker.countThreads()
}

// EXPERIMENTAL: SendMessage sends a message to the worker and waits for a response.
func (w *extensionWorkers) SendMessage(ctx context.Context, message any, rw http.ResponseWriter) (any, error) {
	if w.internalWorker == nil {
		return nil, ErrNotRunning
	}

	fc := newContextFromMessage(message, rw, ctx, w.internalWorker)
	err := w.internalWorker.handleRequest(fc)

	return fc.handlerReturn, err
}
