package frankenphp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"github.com/dunglas/frankenphp/internal/fastabs"
)

// Server represents a preconfigured server block
// requests and workers can be scoped to a Server
type Server struct {
	idx                       int
	name                      string
	root                      string
	splitPath                 []string
	env                       PreparedEnv
	workers                   []*worker
	workersByPath             map[string]*worker
	workersWithRequestMatcher []*worker
	workerOpts                []workerOpt

	// registered while FrankenPHP runs with this server; read by concurrent
	// ServeHTTP calls while Init()/Shutdown() flip it, hence atomic
	isRegistered atomic.Bool

	// atomic for the same reason: the fallback server's logger is replaced
	// at registration time while in-flight requests may read it
	logger atomic.Pointer[slog.Logger]
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
	fallbackServer.logger.Store(globalLogger)
	fallbackServer.isRegistered.Store(true)
	for i, s := range servers {
		s.isRegistered.Store(true)
		s.idx = i
		if s.name == "" {
			s.name = "server_" + strconv.Itoa(i)
		}
	}
}

func unregisterServers() {
	fallbackServer.isRegistered.Store(false)
	for _, server := range servers {
		server.isRegistered.Store(false)
	}
}

// NewServer creates a Server that can be registered via WithServer().
// name is a human-readable identifier used to attribute workers, metrics
// and logs to this server; when empty, it defaults to the index the
// server gets at registration time.
func NewServer(name, root string, splitPath []string, env map[string]string, logger *slog.Logger) (*Server, error) {
	root, err := fastabs.FastAbs(root)
	if err != nil {
		return nil, err
	}

	if err := normalizeSplitPath(splitPath); err != nil {
		return nil, err
	}

	s := &Server{
		name:          name,
		root:          root,
		splitPath:     splitPath,
		env:           PrepareEnv(env),
		workersByPath: make(map[string]*worker),
		workerOpts:    make([]workerOpt, 0),
	}

	if logger == nil {
		logger = globalLogger
	}
	s.logger.Store(logger)

	if len(s.splitPath) == 0 {
		s.splitPath = []string{".php"}
	}

	if s.env == nil {
		s.env = PrepareEnv(nil)
	}

	return s, nil
}

// Name returns the human-readable name of the server.
// It is empty until registration if none was passed to NewServer().
func (s *Server) Name() string {
	return s.name
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
	if !s.isRegistered.Load() {
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
