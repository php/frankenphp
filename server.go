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
	// set by WithServerName(), a run names an unnamed server after its index
	name      string
	root      string
	splitPath []string
	env       PreparedEnv
	logger    *slog.Logger
	// set while FrankenPHP runs with this server; a request loads it once,
	// so a reload cannot change it between the check and the worker lookup
	state atomic.Pointer[serverState]
}

// runState is what a run of FrankenPHP assigns to the servers.
// Init() builds a new one and never modifies its server states once published.
type runState struct {
	servers             map[*Server]*serverState
	globalWorkersByPath map[string]*worker
}

type serverState struct {
	name                      string
	logger                    *slog.Logger
	workersByPath             map[string]*worker
	globalWorkersByPath       map[string]*worker
	workersWithRequestMatcher []*worker
}

var (
	// nil while FrankenPHP is not running, only accessed under startupMu
	currentRun     *runState
	fallbackServer = &Server{env: make(map[string]string)}
)

// newRunState gives every server its identity for a run; initWorkers() then adds the workers
func newRunState(servers []*Server) *runState {
	r := &runState{
		servers:             make(map[*Server]*serverState, len(servers)+1),
		globalWorkersByPath: make(map[string]*worker),
	}

	r.servers[fallbackServer] = r.newServerState("", globalLogger)
	for i, s := range servers {
		name := s.name
		if name == "" {
			name = "server_" + strconv.Itoa(i)
		}

		r.servers[s] = r.newServerState(name, s.logger)
	}

	return r
}

func (r *runState) newServerState(name string, logger *slog.Logger) *serverState {
	return &serverState{
		name:                name,
		logger:              logger,
		workersByPath:       make(map[string]*worker),
		globalWorkersByPath: r.globalWorkersByPath,
	}
}

// publish lets the servers of the run accept requests
func (r *runState) publish() {
	for s, state := range r.servers {
		s.state.Store(state)
	}
}

func (r *runState) unpublish() {
	for s := range r.servers {
		s.state.Store(nil)
	}
}

func (r *runState) addWorker(w *worker) error {
	if w.server == nil {
		r.globalWorkersByPath[w.fileName] = w
		r.globalWorkersByPath[w.declaredFileName] = w

		return nil
	}

	// a server not passed to WithServer() never routes requests
	s := r.servers[w.server]
	if s == nil {
		return nil
	}

	if w.matchRequest != nil {
		s.workersWithRequestMatcher = append(s.workersWithRequestMatcher, w)
		return nil
	}

	if _, exists := s.workersByPath[w.fileName]; exists {
		return fmt.Errorf("two workers in a server cannot have the same filename: %q", w.fileName)
	}
	s.workersByPath[w.fileName] = w
	// a document root whose symlinks are kept yields the declared path, not the resolved one
	s.workersByPath[w.declaredFileName] = w

	return nil
}

// workerByPath finds the worker of the server, or else the global one, running a script
func (s *serverState) workerByPath(path string) *worker {
	if w := s.workersByPath[path]; w != nil {
		return w
	}

	return s.globalWorkersByPath[path]
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

	s := &Server{root: root}

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
// It is empty while not running if none was passed to NewServer().
func (s *Server) Name() string {
	if state := s.state.Load(); state != nil {
		return state.name
	}

	return s.name
}

// ServeHTTP executes a PHP script on the registered server.
// The request will be scoped to the server instance that was registered via WithServer().
// Otherwise, it is equivalent to calling ServeHTTP.
func (s *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, opts ...RequestOption) error {
	state := s.state.Load()
	if state == nil {
		return ErrNotRunning
	}

	h := responseWriter.Header()
	if h["Server"] == nil {
		h["Server"] = serverHeader
	}

	fc, err := newContextFromRequest(request, responseWriter, s, state, opts...)
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
