package caddy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
)

const reloadRequestKey = "frankenphp.original_request"

type reloadRequest struct {
	context       context.Context
	header        http.Header
	host          string
	body          io.ReadCloser
	contentLength int64
	writer        http.ResponseWriter
	guard         *reloadResponseWriter
}

// replayBody returns the reader the replay must use, and whether that reader
// still carries the whole body the client sent.
func (r *reloadRequest) replayBody(current io.ReadCloser) (io.ReadCloser, bool) {
	if body, ok := r.body.(*reloadBody); ok {
		if body.replacement != nil {
			return body.replacement, true
		}
		// Inspecting middleware may restore a reader holding everything the
		// client sent; the captured source is already consumed.
		if body.replay == nil && body.read > 0 {
			if !body.closed && body.restoresSource(current) {
				return current, true
			}
			return nil, false
		}
	}
	return r.body, true
}

func (b *reloadBody) restoresSource(body io.ReadCloser) bool {
	switch body := body.(type) {
	case struct {
		io.Reader
		io.WriterTo
		io.Closer
	}:
		return body.Closer == b
	case struct {
		io.Reader
		io.Closer
	}:
		return body.Closer == b
	}
	return false
}

// reloadBody shields the client's body from the old chain's Set and MaxSize:
// it keeps a replayable copy of whatever middleware buffered, without ever
// buffering ordinary streaming reads itself.
type reloadBody struct {
	io.ReadCloser
	request     *http.Request
	replay      io.Reader
	replacement io.ReadCloser
	read        int64
	lastByte    byte
	closed      bool
}

func (b *reloadBody) Read(p []byte) (int, error) {
	if b.replay != nil {
		return b.replay.Read(p)
	}
	n, err := b.ReadCloser.Read(p)
	b.read += int64(n)
	if n > 0 {
		b.lastByte = p[n-1]
	}
	return n, err
}

// Placeholders buffer into w; keep a separate reader over those bytes so later
// reads cannot exhaust the replay.
func (b *reloadBody) WriteTo(w io.Writer) (int64, error) {
	buf, buffered := w.(*bytes.Buffer)
	start := 0
	if buffered {
		start = buf.Len()
		if b.restoresSource(b.request.Body) {
			// A restored MultiReader copies its buffered prefix again before
			// reaching the source.
			start = 0
		}
	}
	n, err := io.Copy(w, struct{ io.Reader }{b})
	if buffered {
		b.replay = io.MultiReader(bytes.NewReader(buf.Bytes()[start:]), b.ReadCloser)
	}
	return n, err
}

func (b *reloadBody) Close() error {
	// net/http owns the source body; this Close only marks and adopts.
	b.closed = true
	if b.replay != nil || b.replacement != nil || b.read == 0 || b.request.Body == b {
		return nil
	}
	// A middleware reader that restores its buffered input plus the unread
	// source holds the whole body: keep it before a later Set replaces it.
	if b.restoresSource(b.request.Body) {
		b.replacement = b.request.Body
		return nil
	}
	// The buffered reader left installed on the request is the only copy;
	// drain it before a later Set discards it.
	buffered, ok := b.request.Body.(io.WriterTo)
	if !ok {
		return nil
	}
	var buf bytes.Buffer
	if _, err := buffered.WriteTo(&buf); err != nil {
		return err
	}
	b.request.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	var overflow []byte
	if b.read == int64(buf.Len())+1 {
		// A size limit consumes one extra source byte the buffer excludes.
		overflow = []byte{b.lastByte}
	}
	b.replay = io.MultiReader(bytes.NewReader(buf.Bytes()), bytes.NewReader(overflow), b.ReadCloser)
	return nil
}

// reloadCaptureHandler records what the client sent and the ingress writer, so
// a later handoff can replay the request through the replacement chain and
// silence the retiring one.
type reloadCaptureHandler struct{}

func (reloadCaptureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if r.Body != nil {
		r.Body = &reloadBody{ReadCloser: r.Body, request: r}
	}
	guard := &reloadResponseWriter{ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{ResponseWriter: w}}
	caddyhttp.SetVar(r.Context(), reloadRequestKey, &reloadRequest{
		context: r.Context(), header: r.Header.Clone(), host: r.Host,
		body: r.Body, contentLength: r.ContentLength,
		writer: w, guard: guard,
	})
	return next.ServeHTTP(guard, r)
}

// Track irreversible writes while keeping the active chain streaming.
type reloadResponseWriter struct {
	*caddyhttp.ResponseWriterWrapper
	committed bool
}

func (w *reloadResponseWriter) WriteHeader(status int) {
	if status == http.StatusSwitchingProtocols || status >= 200 {
		w.committed = true
	}
	w.ResponseWriterWrapper.WriteHeader(status)
}

func (w *reloadResponseWriter) Write(p []byte) (int, error) {
	w.committed = true
	return w.ResponseWriterWrapper.Write(p)
}

func (w *reloadResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	w.committed = true
	return w.ResponseWriterWrapper.ReadFrom(r)
}

func (w *reloadResponseWriter) FlushError() error {
	w.committed = true
	return http.NewResponseController(w.ResponseWriterWrapper).Flush()
}

func (w *reloadResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriterWrapper).Hijack()
	if err == nil {
		w.committed = true
	}
	return conn, rw, err
}

// replayResponseWriter peels the old chain's response middleware off the
// writer, keeping only its pass-through recorder so its logs stay accurate.
func replayResponseWriter(w http.ResponseWriter) http.ResponseWriter {
	var recorder http.ResponseWriter
	var wrappers []*caddyhttp.ResponseWriterWrapper
	var wrappersBeforeRecorder int
	for range 64 { // third-party wrappers cannot be trusted to terminate
		if rec, ok := w.(caddyhttp.ResponseRecorder); ok && rec.Buffer() == nil {
			recorder = w
			wrappersBeforeRecorder = len(wrappers)
		}
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		next := unwrapper.Unwrap()
		if next == nil {
			break
		}
		if wrapper, ok := w.(*caddyhttp.ResponseWriterWrapper); ok {
			wrappers = append(wrappers, wrapper)
		}
		w = next
	}
	if recorder != nil {
		w = recorder
		wrappers = wrappers[:wrappersBeforeRecorder]
	}
	// Retired wrappers still run their deferred Close; discard whatever they
	// write while keeping the recorder's connection to the client.
	for _, wrapper := range wrappers {
		wrapper.ResponseWriter = discardedResponseWriter{make(http.Header)}
	}
	clear(w.Header())
	return w
}

type discardedResponseWriter struct{ header http.Header }

func (w discardedResponseWriter) Header() http.Header       { return w.header }
func (discardedResponseWriter) WriteHeader(int)             {}
func (discardedResponseWriter) Write(p []byte) (int, error) { return len(p), nil }

// replayReloadWriter serves the replay through the captured ingress writer and
// points the retiring chain's guard at a discard sink. A committed response
// cannot be replaced, so refusing a replay leaves the old writer intact.
func replayReloadWriter(original *reloadRequest, w http.ResponseWriter) (http.ResponseWriter, bool) {
	if original == nil || original.guard == nil {
		return replayResponseWriter(w), true
	}
	if original.guard.committed {
		return nil, false
	}
	original.guard.ResponseWriter = discardedResponseWriter{header: make(http.Header)}
	clear(original.writer.Header())
	return original.writer, true
}

// Mount a capture route in front of everything the user configured, so it
// runs for every request even when no route matches.
func captureReloadRequest(ctx caddy.Context) {
	srv, ok := ctx.Value(caddyhttp.ServerCtxKey).(*caddyhttp.Server)
	if !ok || len(srv.Routes) == 0 {
		return
	}
	if captureReloadInstalled(srv.Routes) {
		return
	}
	// Error- or named-route-only PHP is never a replay target.
	provisioning := false
	for _, route := range srv.Routes {
		provisioning = provisioning || route.HandlersRaw != nil
	}
	if !provisioning {
		return
	}
	needed := routesChangeRequest(srv.Routes)
	if srv.Errors != nil {
		needed = needed || routesChangeRequest(srv.Errors.Routes)
	}
	for _, route := range srv.NamedRoutes {
		needed = needed || routesChangeRequest(caddyhttp.RouteList{*route})
	}
	if !needed {
		return
	}
	// Wrap in place: PHP is still provisioning the original backing array,
	// and the chain is compiled only after that loop ends.
	routes := srv.Routes
	srv.Routes = caddyhttp.RouteList{
		{Handlers: []caddyhttp.MiddlewareHandler{reloadCaptureHandler{}, &reloadRoutes{routes: routes}}},
	}
	// The route carries no raw modules; this only compiles its chain.
	_ = srv.Routes[0].ProvisionHandlers(ctx, nil)
}

func captureReloadInstalled(routes caddyhttp.RouteList) bool {
	if len(routes) != 1 || len(routes[0].Handlers) != 2 {
		return false
	}
	if _, ok := routes[0].Handlers[0].(reloadCaptureHandler); !ok {
		return false
	}
	_, ok := routes[0].Handlers[1].(*reloadRoutes)
	return ok
}

// reloadRoutes runs the configured routes for every request entering the
// server, compiling its chain once instead of per request.
type reloadRoutes struct {
	routes caddyhttp.RouteList
	once   sync.Once
	chain  caddyhttp.Handler
}

func (h *reloadRoutes) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	h.once.Do(func() { h.chain = h.routes.Compile(next) })
	return h.chain.ServeHTTP(w, r)
}

func routesChangeRequest(routes caddyhttp.RouteList) bool {
	for _, route := range routes {
		if matchersChangeRequest(route.MatcherSetsRaw, route.MatcherSets) {
			return true
		}
		for _, raw := range route.HandlersRaw {
			var handler struct {
				caddyhttp.Subroute
				Handler string          `json:"handler"`
				Request json.RawMessage `json:"request"`
			}
			if json.Unmarshal(raw, &handler) != nil {
				return true
			}
			switch handler.Handler {
			case "php", "rewrite", "vars", "file_server", "static_response":
			case "headers":
				if len(handler.Request) > 0 && string(handler.Request) != "null" {
					return true
				}
			case "subroute":
				if routesChangeRequest(handler.Routes) || errorsChangeRequest(handler.Errors) {
					return true
				}
			default:
				return true
			}
		}
		for _, handler := range route.Handlers {
			switch handler := handler.(type) {
			case *caddyhttp.Subroute:
				if routesChangeRequest(handler.Routes) || errorsChangeRequest(handler.Errors) {
					return true
				}
			case *headers.Handler:
				if handler.Request != nil {
					return true
				}
			default:
				switch caddy.GetModuleName(handler) {
				case "php", "rewrite", "vars", "file_server", "static_response":
				default:
					return true
				}
			}
		}
	}
	return false
}

func errorsChangeRequest(errors *caddyhttp.HTTPErrorConfig) bool {
	return errors != nil && routesChangeRequest(errors.Routes)
}

// Unknown matchers are conservative: plugins can rewrite requests, even
// inside a negation.
func matchersChangeRequest(raw caddyhttp.RawMatcherSets, sets caddyhttp.MatcherSets) bool {
	for _, set := range raw {
		for name, config := range set {
			if name == "not" {
				var nested caddyhttp.RawMatcherSets
				if json.Unmarshal(config, &nested) != nil || matchersChangeRequest(nested, nil) {
					return true
				}
			} else if !readOnlyMatcher(name) {
				return true
			}
		}
	}
	for _, set := range sets {
		for _, matcher := range set {
			switch matcher := matcher.(type) {
			case *caddyhttp.MatchNot:
				if matchersChangeRequest(matcher.MatcherSetsRaw, matcher.MatcherSets) {
					return true
				}
			default:
				if !readOnlyMatcher(caddy.GetModuleName(matcher)) {
					return true
				}
			}
		}
	}
	return false
}

func readOnlyMatcher(name string) bool {
	switch name {
	case "host", "path", "path_regexp", "method", "query", "header", "header_regexp",
		"protocol", "tls", "remote_ip", "client_ip", "vars", "vars_regexp", "file":
		return true
	}
	return false
}
