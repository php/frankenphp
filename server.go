package frankenphp

import (
	"fmt"
	"log/slog"
	"net/http"
)

// server represents a server block in the caddyfile.
// can also be used to scope workers to a specific set of configurations.
type server struct {
	idx                       int
	root                      string
	splitPath                 []string
	env                       PreparedEnv
	workers                   []*worker
	workersByPath             map[string]*worker
	workersWithRequestMatcher []*worker
	workerOpts                []workerOpt
	logger                    *slog.Logger
}

var (
	servers        = make(map[int]*server)
	fallbackServer = &server{
		idx:           -1,
		workersByPath: make(map[string]*worker),
		env:           make(map[string]string),
	}
)

func resetServers() {
	servers = make(map[int]*server)
}

func newServer(idx int, root string, splitPath []string, env map[string]string) (*server, error) {
	if _, ok := servers[idx]; ok {
		return nil, fmt.Errorf("%w: duplicate registration of server with idx %d", ErrAlreadyRegistered, idx)
	}

	server := &server{
		idx:           idx,
		root:          root,
		splitPath:     splitPath,
		env:           env,
		workersByPath: make(map[string]*worker),
		workerOpts:    make([]workerOpt, 0),
	}

	if len(server.splitPath) == 0 {
		server.splitPath = []string{".php"}
	}

	if server.env == nil {
		server.env = PrepareEnv(nil)
	}

	servers[server.idx] = server

	return server, nil
}

func (s *server) addWorker(w *worker) error {
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

func (s *server) serveHTTP(responseWriter http.ResponseWriter, request *http.Request, opts ...RequestOption) error {
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
