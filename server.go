package frankenphp

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/dunglas/frankenphp/internal/fastabs"
)

// Server represents a preconfigured server block
// requests and workers can be scoped to a Server
type Server struct {
	idx                       int
	root                      string
	splitPath                 []string
	env                       PreparedEnv
	logger                    *slog.Logger
	workers                   []*worker
	workersByPath             map[string]*worker
	workersWithRequestMatcher []*worker
	workerOpts                []workerOpt
	isRegistered              bool
}

var (
	servers        = []*Server{} // currently unused, but useful down the line
	fallbackServer = &Server{
		idx:           -1,
		workersByPath: make(map[string]*worker),
		env:           make(map[string]string),
	}
)

func registerServers(newServers []*Server) {
	servers = newServers
	fallbackServer.isRegistered = true
	for i, s := range servers {
		s.isRegistered = true
		s.idx = i
	}
}

func unregisterServers() {
	fallbackServer.isRegistered = false
	for _, server := range servers {
		server.isRegistered = false
	}
}

func NewServer(root string, splitPath []string, env map[string]string, logger *slog.Logger) (*Server, error) {
	root, err := fastabs.FastAbs(root)
	if err != nil {
		return nil, err
	}

	if err := normalizeSplitPath(splitPath); err != nil {
		return nil, err
	}

	s := &Server{
		root:          root,
		splitPath:     splitPath,
		env:           PrepareEnv(env),
		logger:        logger,
		workersByPath: make(map[string]*worker),
		workerOpts:    make([]workerOpt, 0),
	}

	if len(s.splitPath) == 0 {
		s.splitPath = []string{".php"}
	}

	if s.env == nil {
		s.env = PrepareEnv(nil)
	}

	if s.logger == nil {
		s.logger = globalLogger
	}

	return s, nil
}

func (s *Server) addWorker(w *worker) error {
	s.workers = append(s.workers, w)
	if w.matchRequest != nil {
		s.workersWithRequestMatcher = append(s.workersWithRequestMatcher, w)
		return nil
	}

	if _, exists := s.workersByPath[w.fileName]; exists {
		return fmt.Errorf("two workers in a server cannot have the same filename: %q", w.fileName)
	}
	s.workersByPath[w.fileName] = w

	return nil
}

// ServeHTTP executes a PHP script on the registered server.
// The request will be scoped to the server instance that was registered via WithServer().
// Otherwise, it is equivalent to calling ServeHTTP.
func (s *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, opts ...RequestOption) error {
	if !s.isRegistered {
		return ErrNotRunning
	}

	h := responseWriter.Header()
	if h["Server"] == nil {
		h["Server"] = serverHeader
	}

	fc, err := newContextFromRequest(request, responseWriter, s, opts...)
	if err != nil {
		return err
	}

	if err := fc.validate(); err != nil {
		return err
	}

	// Handle request with a worker if one is assigned
	if fc.worker != nil {
		return fc.worker.handleRequest(fc)
	}

	// If no worker was available, send the request to non-worker threads
	return handleRequestWithRegularPHPThreads(fc)
}
