package frankenphp

import (
	"log/slog"
	"net/http"
	"sync"
)

// PhpServer represents a PHP server instance.
// it helps to scope a request to a specific set of configurations.
// useful to represent a php_server or php block in the caddyfile.
type PhpServer struct {
	idx           int
	root          string
	splitPath     []string
	env           PreparedEnv
	workers       []*worker
	workersByPath map[string]*worker
	workerOpts    []workerOpt
	logger        *slog.Logger
	mainThread    *phpMainThread
}

// PhpServerOption instances allow to configure a PhpServer.
type PhpServerOption func(*PhpServer) error

var (
	PhpServers   = make(map[int]*PhpServer)
	phpServersMu sync.Mutex
)

func drainPhpServers() {
	phpServersMu.Lock()
	defer phpServersMu.Unlock()
	PhpServers = make(map[int]*PhpServer)
}

func newPhpServer(idx int, opts ...PhpServerOption) (*PhpServer, error) {
	phpServersMu.Lock()
	defer phpServersMu.Unlock()

	existingPhpServer, ok := PhpServers[idx]
	if ok {
		globalLogger.Debug("php server already registered, ignoring duplicate registration", "idx", idx)
		return existingPhpServer, nil
	}

	phpServer := &PhpServer{
		idx:           idx,
		env:           make(map[string]string),
		workersByPath: make(map[string]*worker),
		workerOpts:    make([]workerOpt, 0),
	}

	for _, option := range opts {
		if err := option(phpServer); err != nil {
			return phpServer, err
		}
	}

	PhpServers[phpServer.idx] = phpServer

	return phpServer, nil
}

// fallback PHP server if none could be associated with a request
func newDummyPhpServer() *PhpServer {
	return &PhpServer{
		idx:           -1,
		workersByPath: make(map[string]*worker),
		env:           make(map[string]string),
	}
}

// ServeHTTP executes a PHP script according to the given context.
func (s *PhpServer) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, opts ...RequestOption) error {
	h := responseWriter.Header()
	if h["Server"] == nil {
		h["Server"] = serverHeader
	}

	if !isRunning {
		return ErrNotRunning
	}

	fc, err := newContextFromRequest(request, responseWriter, s, opts...)
	if err != nil {
		return err
	}

	if err := fc.validate(); err != nil {
		return err
	}

	// Detect if a worker is available to handle this request
	if fc.worker != nil {
		return fc.worker.handleRequest(fc)
	}

	// If no worker was available, send the request to non-worker threads
	return handleRequestWithRegularPHPThreads(fc)
}
