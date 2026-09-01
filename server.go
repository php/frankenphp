package frankenphp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/fastabs"
)

// Server represents a preconfigured server block
// requests and workers can be scoped to a Server
type Server struct {
	name                      string
	root                      string
	splitPath                 []string
	env                       PreparedEnv
	workers                   []*worker
	workersByPath             map[string]*worker
	workersWithRequestMatcher []*worker
	maxWaitTime               time.Duration
	logger                    *slog.Logger
	isRegistered              bool
	isActive                  atomic.Bool
}

var (
	servers        []*Server
	fallbackServer = newFallbackServer()
)

func newFallbackServer() *Server {
	s := &Server{
		workersByPath: make(map[string]*worker),
		env:           make(map[string]string),
		logger:        globalLogger,
		workers: []*worker{},
		workersWithRequestMatcher: []*worker{},
	}

	return s
}

// registerServers assigns the identity of every server and clears the workers of a previous run,
func registerServers(o *opt) error {
	servers = o.servers
	fallbackServer.maxWaitTime = o.maxWaitTime
	fallbackServer.logger = globalLogger

	for i, s := range servers {
		if s.isRegistered {
			return fmt.Errorf("server %q was registered previously and cannot be registered again", s.name)
		}
		s.isRegistered = true
		s.maxWaitTime = o.maxWaitTime
		if s.name == "" {
			s.name = "server_" + strconv.Itoa(i)
		}
	}

	return nil
}

// activateServers lets registered servers accept requests
func activateServers() {
	fallbackServer.isActive.Store(true)
	for _, s := range servers {
		s.isActive.Store(true)
	}
}

func deactivateServers() {
	fallbackServer.isActive.Store(false)
	for _, server := range servers {
		server.isActive.Store(false)
	}
	servers = nil
}

// NewServer creates a Server that can be registered via WithServer().
// name is a human-readable identifier used to attribute workers, metrics
// and logs to this server; when empty, it defaults to the index the
// server gets at registration time.
func NewServer(root string, options ...ServerOption) (*Server, error) {
	root, err := fastabs.FastAbs(root)
	if err != nil {
		return nil, err
	}

	s := &Server{
		root:          root,
		workersByPath: make(map[string]*worker),
	}

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}

	if s.logger == nil {
		s.logger = globalLogger
	}

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
	if !s.isActive.Load() {
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
