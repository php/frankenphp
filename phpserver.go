package frankenphp

import (
	"log/slog"
	"net/http"
)

// PhpServer represents a php_server block in the caddyfile.
// can also be used to scope workers to a specific set of configurations.
type PhpServer struct {
	idx           int
	root          string
	splitPath     []string
	env           PreparedEnv
	workers       []*worker
	workersByPath map[string]*worker
	workerOpts    []workerOpt
	logger        *slog.Logger
}

// PhpServers is a map of all registered PhpServer instances.
// instances will be accessible after frankenphp.Init() has been called.
var PhpServers = make(map[int]*PhpServer)

func drainPhpServers() {
	PhpServers = make(map[int]*PhpServer)
}

func newPhpServer(idx int, opts ...PhpServerOption) (*PhpServer, error) {
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
// the request will be scoped to the PhpServer instance.
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
