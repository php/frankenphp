package caddy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/requestbody"
	"github.com/dunglas/frankenphp"
	testserver "github.com/dunglas/frankenphp/caddy/internal/testserver"
	"github.com/stretchr/testify/require"
)

func init() {
	caddy.RegisterModule(reloadWaitHandler{})
	caddy.RegisterModule(reloadOpaqueWAF{})
	caddy.RegisterModule(reloadWriteHandler{})
}

type reloadWriteHandler struct {
	Mode string `json:"mode"`
}

func (reloadWriteHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.reload_test_write", New: func() caddy.Module { return new(reloadWriteHandler) }}
}

func (h reloadWriteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	w.Header().Set("X-Release", "old")
	switch h.Mode {
	case "header":
		w.WriteHeader(http.StatusAccepted)
	case "body":
		if _, err := w.Write([]byte("old:")); err != nil {
			return err
		}
	case "read_from":
		if _, err := io.Copy(w, struct{ io.Reader }{strings.NewReader("old:")}); err != nil {
			return err
		}
	case "flush":
		if err := http.NewResponseController(w).Flush(); err != nil {
			return err
		}
	}
	return next.ServeHTTP(w, r)
}

// reloadWaitHandler parks the request in PHP's wait for the replacement app
// until the test releases it, proving the whole request head was parsed before
// the old HTTP server starts shutting down.
type reloadWaitState struct{ waiting chan struct{} }

var activeReloadWait atomic.Pointer[reloadWaitState]

type reloadWaitHandler struct{}

func (reloadWaitHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.reload_test_wait", New: func() caddy.Module { return new(reloadWaitHandler) }}
}

func (reloadWaitHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if r.Header.Get("X-Reload-Test-Wait") == "1" {
		if state := activeReloadWait.Load(); state != nil {
			r = r.WithContext(&reloadHandoffWaitingContext{Context: r.Context(), waiting: state.waiting})
		}
	}
	return next.ServeHTTP(w, r)
}

type reloadHandoffWaitingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *reloadHandoffWaitingContext) Done() <-chan struct{} {
	c.once.Do(func() { c.waiting <- struct{}{} })
	return c.Context.Done()
}

// reloadOpaqueWAF mimics the coraza-caddy response writer stack, the remaining
// known way a retired middleware can sit between the client and the replay:
// the response writer is wrapped in an anonymous struct embedding an
// interface, without Unwrap(), so the original writer beneath it cannot be
// recovered; response rules run when the old chain completes and can still
// reject or filter the response the replacement produced.
type reloadOpaqueWAF struct {
	Deny string `json:"deny,omitempty"`
}

func (reloadOpaqueWAF) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.reload_test_opaque_waf",
		New: func() caddy.Module { return new(reloadOpaqueWAF) },
	}
}

type reloadOpaqueResponseWriter interface {
	http.ResponseWriter
	io.ReaderFrom
	http.Flusher
}

type reloadOpaqueInterceptor struct {
	w      http.ResponseWriter
	deny   string
	status int
	buf    bytes.Buffer
	wrote  bool
}

func (i *reloadOpaqueInterceptor) Header() http.Header { return i.w.Header() }

func (i *reloadOpaqueInterceptor) WriteHeader(status int) {
	if i.wrote {
		return
	}
	i.status = status
}

func (i *reloadOpaqueInterceptor) Write(p []byte) (int, error) {
	if !i.wrote {
		i.WriteHeader(http.StatusOK)
	}
	i.wrote = true
	return i.buf.Write(p)
}

func (i *reloadOpaqueInterceptor) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{i}, r)
}

// Flush is suppressed while the body is buffered for inspection, like the
// interceptor of a WAF with response body rules.
func (i *reloadOpaqueInterceptor) Flush() {}

// processResponse mirrors the deferred response processor: the retired rules
// inspect the buffered response once the chain completes, and can still
// replace the status and body the replacement already decided on.
func (i *reloadOpaqueInterceptor) processResponse() error {
	if i.deny != "" && strings.Contains(i.buf.String(), i.deny) {
		for k := range i.w.Header() {
			i.w.Header().Del(k)
		}
		i.w.Header().Set("Content-Length", "0")
		i.w.WriteHeader(http.StatusForbidden)
		return caddyhttp.HandlerError{StatusCode: http.StatusForbidden, Err: errReloadOpaqueBlocked}
	}
	i.w.WriteHeader(i.status)
	_, err := i.w.Write(i.buf.Bytes())
	return err
}

var errReloadOpaqueBlocked = errors.New("opaque waf: response blocked by the retired rules")

func (h reloadOpaqueWAF) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	i := &reloadOpaqueInterceptor{w: w, deny: h.Deny, status: http.StatusOK}
	// Like coraza's wrap(): the exported writer is an anonymous struct
	// embedding an interface; it hides the writer beneath it and has no
	// Unwrap method.
	var wrapped reloadOpaqueResponseWriter = struct{ reloadOpaqueResponseWriter }{i}
	if err := next.ServeHTTP(wrapped, r); err != nil {
		return err
	}
	return i.processResponse()
}

type reloadWaitingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *reloadWaitingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestReloadUsesNewPHPOptions(t *testing.T) {
	for _, worker := range []bool{false, true} {
		t.Run(fmt.Sprintf("worker=%t", worker), func(t *testing.T) {
			newApp := func(body string) *FrankenPHPApp {
				root, err := filepath.EvalSymlinks(t.TempDir())
				require.NoError(t, err)
				script := fmt.Sprintf("<?php $handle = function () { echo %q . file_get_contents('php://input'); };", body)
				if worker {
					script += "while (frankenphp_handle_request($handle)) {}"
				} else {
					script += "$handle();"
				}
				require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(script), 0600))
				app := &FrankenPHPApp{NumThreads: 1, MaxThreads: 3, ctx: context.Background(), logger: slog.New(slog.DiscardHandler), started: make(chan any)}
				module := &FrankenPHPModule{Name: "site", resolvedDocumentRoot: root, app: app}
				if worker {
					module.Workers = []workerConfig{{Name: "site-worker", FileName: filepath.Join(root, "index.php"), Num: 1}}
				}
				app.modules = []*FrankenPHPModule{module}
				return app
			}
			old, current := newApp("old:"), newApp("new:")
			draining, resume := make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(resume) })
			t.Cleanup(func() { release(); frankenphp.Shutdown(); activeApp.Store(nil) })
			var external frankenphp.Workers
			var hookErr error
			external, option := frankenphp.WithExtensionWorkers("hook", "../testdata/worker-with-counter.php", 1,
				frankenphp.WithWorkerOnServerShutdown(func() {
					hookErr = external.SendRequest(httptest.NewRecorder(), httptest.NewRequest("GET", "http://localhost/", nil))
					close(draining)
					<-resume
				}))
			old.provisionOpts = []frankenphp.Option{option}
			require.NoError(t, old.Start())
			reloaded := make(chan error, 1)
			go func() { reloaded <- current.Start() }()
			<-draining
			r := httptest.NewRequest("POST", "http://site/index.php", strings.NewReader("payload"))
			ctx := context.WithValue(r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer())
			ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, *r)
			waiting := &reloadWaitingContext{Context: ctx, waiting: make(chan struct{})}
			response := httptest.NewRecorder()
			response.Header().Set("X-Existing", "preserved")
			served := make(chan error, 1)
			go func() { served <- old.modules[0].ServeHTTP(response, r.WithContext(waiting), nil) }()
			<-waiting.waiting
			release()
			require.NoError(t, <-reloaded)
			require.NoError(t, <-served)
			require.NoError(t, hookErr)
			require.Equal(t, "new:payload", response.Body.String())
			require.Equal(t, "preserved", response.Header().Get("X-Existing"))
		})
	}
}

func TestNewModuleRequestStallsUntilAppStarts(t *testing.T) {
	newApp := func(body string) *FrankenPHPApp {
		root, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		script := fmt.Sprintf("<?php $handle = function () { echo %q . file_get_contents('php://input'); }; $handle();", body)
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte(script), 0600))
		app := &FrankenPHPApp{NumThreads: 1, MaxThreads: 3, ctx: context.Background(), logger: slog.New(slog.DiscardHandler), started: make(chan any)}
		module := &FrankenPHPModule{Name: "site", resolvedDocumentRoot: root, app: app}
		app.modules = []*FrankenPHPModule{module}
		return app
	}
	old, current := newApp("old:"), newApp("new:")
	t.Cleanup(func() { frankenphp.Shutdown(); activeApp.Store(nil) })
	require.NoError(t, old.Start())

	r := httptest.NewRequest("POST", "http://site/index.php", strings.NewReader("payload"))
	ctx := context.WithValue(r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer())
	ctx = context.WithValue(ctx, caddyhttp.OriginalRequestCtxKey, *r)
	response := httptest.NewRecorder()
	served := make(chan error, 1)
	go func() { served <- current.modules[0].ServeHTTP(response, r.WithContext(ctx), nil) }()

	select {
	case err := <-served:
		t.Fatalf("a request to a module whose app has not started must stall, got: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, current.Start())
	require.NoError(t, <-served)
	require.Equal(t, "new:payload", response.Body.String())
}

func captureTestServer(routes caddyhttp.RouteList) (*caddyhttp.Server, caddy.Context) {
	srv := &caddyhttp.Server{Routes: routes}
	return srv, caddy.Context{Context: context.WithValue(context.Background(), caddyhttp.ServerCtxKey, srv)}
}

// captureTestChain serves req through the restructured routes, as the server
// would after provisioning finished.
func captureTestChain(t *testing.T, srv *caddyhttp.Server, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	chain := srv.Routes.Compile(caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil }))
	require.NoError(t, chain.ServeHTTP(rec, req))

	return rec
}

func TestReloadCapturePreservesMatcherResults(t *testing.T) {
	for _, test := range []struct {
		name string
		sets caddyhttp.MatcherSets
		want bool
	}{
		{name: "unconditional", want: true},
		{name: "second_alternative", sets: caddyhttp.MatcherSets{
			{new(caddyhttp.MatchPath{"/other"})}, {new(caddyhttp.MatchPath{"/index.php"})},
		}, want: true},
		{name: "no_match", sets: caddyhttp.MatcherSets{
			{new(caddyhttp.MatchPath{"/other"})}, {new(caddyhttp.MatchPath{"/also-other"})},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalRoute := caddyhttp.Route{
				MatcherSets: test.sets,
				HandlersRaw: []json.RawMessage{json.RawMessage(`{"handler":"headers","request":{"set":{"Authorization":["injected"]}}}`)},
			}
			srv, ctx := captureTestServer(caddyhttp.RouteList{originalRoute})
			captureReloadRequest(ctx)
			captureReloadRequest(ctx)

			require.Len(t, srv.Routes, 1, "the restructure is idempotent")
			var captureHandlers int
			for _, handler := range srv.Routes[0].Handlers {
				if _, ok := handler.(reloadCaptureHandler); ok {
					captureHandlers++
				}
			}
			require.Equal(t, 1, captureHandlers, "the capture handler is mounted exactly once")

			// the user's route, matchers untouched, moved inside the wrapped routes
			wrapped, ok := srv.Routes[0].Handlers[1].(*reloadRoutes)
			require.True(t, ok)
			require.Len(t, wrapped.routes, 1)
			inner := wrapped.routes[0]
			require.Equal(t, originalRoute.MatcherSetsRaw, inner.MatcherSetsRaw)

			req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://localhost/index.php", nil), caddy.NewReplacer(), httptest.NewRecorder(), srv)
			req.Header.Set("Authorization", "client")
			matched, err := inner.MatcherSets.AnyMatchWithError(req)
			require.NoError(t, err)
			require.Equal(t, test.want, matched)

			// the capture runs even when no route matches
			captureTestChain(t, srv, req)
			original := caddyhttp.GetVar(req.Context(), reloadRequestKey).(*reloadRequest)
			req.Header["Authorization"][0] = "injected"
			req.Host = "trusted.example"
			require.Equal(t, "client", original.header.Get("Authorization"))
			require.Equal(t, "localhost", original.host)
			require.Same(t, req.Context(), original.context)
			require.NotNil(t, original.guard)
		})
	}
}

func TestReloadCaptureLeavesOrdinaryRoutesUnchanged(t *testing.T) {
	var routes caddyhttp.RouteList
	require.NoError(t, json.Unmarshal([]byte(`[{
		"match":[{"host":["localhost"],"not":[{"file":{"try_files":["{http.request.uri.path}"]}}]}],
		"handle":[{"handler":"subroute","routes":[{"handle":[
			{"handler":"vars","root":"/tmp"},
			{"handler":"headers","response":{"set":{"X-Test":["yes"]}}},
			{"handler":"rewrite","uri":"/index.php"},
			{"handler":"php"}, {"handler":"file_server"}
		]}]}]
	}]`), &routes))
	srv, ctx := captureTestServer(routes)
	captureReloadRequest(ctx)
	require.Empty(t, srv.Routes[0].MatcherSets)
	require.Equal(t, routes[0].MatcherSetsRaw, srv.Routes[0].MatcherSetsRaw)
}

func TestReloadCaptureDetectsLoadedAndNestedMutators(t *testing.T) {
	for _, routes := range []caddyhttp.RouteList{
		{{Handlers: []caddyhttp.MiddlewareHandler{&headers.Handler{Request: &headers.HeaderOps{}}}}},
		{{Handlers: []caddyhttp.MiddlewareHandler{&caddyhttp.Subroute{Routes: caddyhttp.RouteList{{HandlersRaw: []json.RawMessage{json.RawMessage(`{"handler":"custom"}`)}}}}}}},
		{{MatcherSetsRaw: caddyhttp.RawMatcherSets{{"not": json.RawMessage(`[{"custom":{}}]`)}}}},
	} {
		require.True(t, routesChangeRequest(routes))
	}
	// The request mutator is already loaded, while PHP is still provisioning.
	srv, ctx := captureTestServer(caddyhttp.RouteList{
		{Handlers: []caddyhttp.MiddlewareHandler{&headers.Handler{Request: &headers.HeaderOps{}}}},
		{HandlersRaw: []json.RawMessage{json.RawMessage(`{"handler":"php"}`)}},
	})
	captureReloadRequest(ctx)
	require.True(t, captureReloadInstalled(srv.Routes))
}

// resolveServerName and httpServerFor locate PHP through the wrapped routes,
// so handoffs still find their target after the restructure.
func TestReloadCaptureKeepsServerNameResolution(t *testing.T) {
	target := &headers.Handler{Request: &headers.HeaderOps{}}
	srv, ctx := captureTestServer(caddyhttp.RouteList{{
		MatcherSets: caddyhttp.MatcherSets{caddyhttp.MatcherSet{&caddyhttp.MatchHost{"localhost"}}},
		Handlers:    []caddyhttp.MiddlewareHandler{target},
		HandlersRaw: []json.RawMessage{json.RawMessage(`{"handler":"php"}`)},
	}})
	captureReloadRequest(ctx)
	require.True(t, captureReloadInstalled(srv.Routes))

	require.True(t, serverContainsHandler(srv, target))
	require.Equal(t, "localhost", findHostInRoutes(srv.Routes, target))
}

type captureTestHandler struct{ name string }

func (h captureTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	_, _ = w.Write([]byte(h.name))
	return next.ServeHTTP(w, r)
}

// The restructure happens while the app's provisioning loop is still walking
// the original routes. The loop's own writes — handlers appended and chains
// compiled, in place — must keep serving through the restructured routes: the
// wrapped routes share the loop's backing array instead of copying it.
func TestReloadCaptureKeepsProvisioningAfterRestructure(t *testing.T) {
	srv, ctx := captureTestServer(caddyhttp.RouteList{
		{HandlersRaw: []json.RawMessage{json.RawMessage(`{"handler":"php"}`)}},
		{HandlersRaw: []json.RawMessage{json.RawMessage(`{"handler":"custom"}`)}},
	})
	loop := srv.Routes

	captureReloadRequest(ctx)

	// the loop keeps provisioning its own view of the routes: php's handlers
	// finish loading and the later route loads as well
	for i := range loop {
		loop[i].HandlersRaw = nil
		loop[i].Handlers = []caddyhttp.MiddlewareHandler{captureTestHandler{fmt.Sprintf("route%d-", i)}}
		require.NoError(t, loop[i].ProvisionHandlers(ctx, nil))
	}
	require.True(t, captureReloadInstalled(srv.Routes))

	req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://localhost/", nil), caddy.NewReplacer(), httptest.NewRecorder(), srv)
	rec := captureTestChain(t, srv, req)
	require.Equal(t, "route0-route1-", rec.Body.String(),
		"routes provisioned after the restructure must still serve")
}

// An unwritten response can be handed over; the retired chain's completion
// writes, header edits and flushes must then be swallowed.
func TestReloadCaptureGuardIsolatesTheRetiredChain(t *testing.T) {
	rec := httptest.NewRecorder()
	req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://localhost/", nil), caddy.NewReplacer(), rec, nil)

	var guard *reloadResponseWriter
	err := (reloadCaptureHandler{}).ServeHTTP(rec, req, caddyhttp.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		guard = w.(*reloadResponseWriter)
		return nil
	}))
	require.NoError(t, err)

	original := caddyhttp.GetVar(req.Context(), reloadRequestKey).(*reloadRequest)
	require.Equal(t, http.ResponseWriter(rec), original.writer)
	require.Same(t, original.guard, guard)
	require.Equal(t, rec.Header(), guard.Header())

	guard.Header().Set("X-Normal", "yes")

	// the replay takes over: it goes through the saved ingress writer, and
	// the eagerly written old headers are cleared for the replacement
	writer, available := replayReloadWriter(original, guard)
	require.True(t, available)
	require.Equal(t, http.ResponseWriter(rec), writer)
	require.Empty(t, rec.Header().Get("X-Normal"))
	writer.WriteHeader(http.StatusAccepted)
	_, err = writer.Write([]byte("new"))
	require.NoError(t, err)

	guard.Header().Set("X-Retired", "no")
	guard.WriteHeader(http.StatusInternalServerError)
	_, err = guard.Write([]byte("retired"))
	require.NoError(t, err)
	require.ErrorIs(t, http.NewResponseController(guard).Flush(), http.ErrNotSupported,
		"the retired chain must not be able to flush the client's stream")

	require.Empty(t, rec.Header().Get("X-Retired"))
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "new", rec.Body.String())
	require.False(t, rec.Flushed)
}

func TestReloadCaptureRejectsCommittedWriters(t *testing.T) {
	for _, mode := range []string{"header", "body", "read_from", "flush", "switching_protocols", "empty_write"} {
		t.Run(mode, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://localhost/", nil), caddy.NewReplacer(), rec, nil)
			require.NoError(t, (reloadCaptureHandler{}).ServeHTTP(rec, req, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				switch mode {
				case "switching_protocols":
					w.WriteHeader(http.StatusSwitchingProtocols)
				case "empty_write":
					_, err := w.Write(nil)
					return err
				default:
					return (reloadWriteHandler{Mode: mode}).ServeHTTP(w, r, caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil }))
				}
				return nil
			})))
			original := caddyhttp.GetVar(req.Context(), reloadRequestKey).(*reloadRequest)
			header, body, status := rec.Header().Clone(), rec.Body.String(), rec.Code
			writer, available := replayReloadWriter(original, original.guard)
			require.False(t, available)
			require.Nil(t, writer)
			require.Equal(t, header, rec.Header(), "refusing a handoff must preserve the old headers")
			require.Equal(t, body, rec.Body.String())
			require.Equal(t, status, rec.Code)
			require.Same(t, rec, original.guard.ResponseWriter, "the old chain must keep its writer")
			if mode == "flush" {
				require.True(t, rec.Flushed)
			}
		})
	}
}

func TestReloadCaptureAllowsInformationalResponses(t *testing.T) {
	rec := httptest.NewRecorder()
	req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodGet, "http://localhost/", nil), caddy.NewReplacer(), rec, nil)
	require.NoError(t, (reloadCaptureHandler{}).ServeHTTP(rec, req, caddyhttp.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusEarlyHints)
		return nil
	})))
	original := caddyhttp.GetVar(req.Context(), reloadRequestKey).(*reloadRequest)
	writer, available := replayReloadWriter(original, original.guard)
	require.True(t, available)
	require.Same(t, rec, writer)
}

// The captured body must stay replayable even after middleware reads, limits
// or replaces it.
func TestReloadCapturePreservesBufferedClientBody(t *testing.T) {
	for _, test := range []struct {
		name        string
		maxSize     int64
		placeholder string
		set         string
	}{
		{name: "body", placeholder: "{http.request.body}"},
		{name: "body_base64", placeholder: "{http.request.body_base64}"},
		{name: "limited", maxSize: 6, placeholder: "{http.request.body}"},
		{name: "exact_limit", maxSize: 14, placeholder: "{http.request.body}"},
		{name: "buffer_then_set", placeholder: "{http.request.body}", set: "injected-payload"},
		{name: "set_from_body", set: "{http.request.body}"},
		{name: "literal_set", set: "injected-payload"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repl := caddy.NewReplacer()
			w := httptest.NewRecorder()
			req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodPost, "http://localhost/index.php", strings.NewReader("client-payload")), repl, w, nil)
			require.NoError(t, (reloadCaptureHandler{}).ServeHTTP(w, req, caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil })))
			original := caddyhttp.GetVar(req.Context(), reloadRequestKey).(*reloadRequest)
			next := caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil })
			if test.maxSize > 0 {
				require.NoError(t, (requestbody.RequestBody{MaxSize: test.maxSize}).ServeHTTP(w, req, next))
			}
			if test.placeholder != "" {
				got := repl.ReplaceAll(test.placeholder, "")
				want := "client-payload"
				if test.placeholder == "{http.request.body_base64}" {
					want = "Y2xpZW50LXBheWxvYWQ="
				}
				if test.maxSize != 6 {
					require.Equal(t, want, got)
				}
				// A second expansion must not consume the replay's saved bytes.
				require.Equal(t, got, repl.ReplaceAll(test.placeholder, ""))
			}
			if test.set != "" {
				require.NoError(t, (requestbody.RequestBody{Set: test.set}).ServeHTTP(w, req, next))
			}
			if test.maxSize == 6 {
				// a partial read leaves no complete body to replay: the
				// handoff must refuse the request
				_, complete := original.replayBody(req.Body)
				require.False(t, complete)
				require.Equal(t, int64(14), original.contentLength)

				return
			}
			body, complete := original.replayBody(req.Body)
			require.True(t, complete)
			payload, err := io.ReadAll(body)
			require.NoError(t, err)
			require.Equal(t, "client-payload", string(payload))
			require.Equal(t, int64(14), original.contentLength)
		})
	}
}

func TestReloadCapturePreservesMiddlewareBufferedBody(t *testing.T) {
	for _, test := range []struct {
		name        string
		read        int64
		readAll     bool
		set         bool
		setBefore   bool
		placeholder bool
	}{
		{name: "coraza", read: 100},
		{name: "coraza_partial", read: 6},
		{name: "coraza_then_set", read: 100, set: true},
		{name: "coraza_partial_then_set", read: 6, set: true},
		{name: "read_all", readAll: true},
		{name: "read_all_then_set", readAll: true, set: true},
		{name: "set_then_coraza", read: 100, setBefore: true},
		{name: "set_then_read_all", readAll: true, setBefore: true},
		{name: "coraza_then_placeholder", read: 100, placeholder: true},
		{name: "coraza_partial_then_placeholder", read: 6, placeholder: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			repl := caddy.NewReplacer()
			req := caddyhttp.PrepareRequest(httptest.NewRequest(http.MethodPost, "http://localhost/index.php", strings.NewReader("client-payload")), repl, w, nil)
			require.NoError(t, (reloadCaptureHandler{}).ServeHTTP(w, req, caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil })))
			original := caddyhttp.GetVar(req.Context(), reloadRequestKey).(*reloadRequest)
			source := req.Body.(*reloadBody)
			next := caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil })
			if test.setBefore {
				require.NoError(t, (requestbody.RequestBody{Set: "injected-payload"}).ServeHTTP(w, req, next))
			}
			if test.readAll {
				buffered, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				req.Body = struct {
					io.Reader
					io.Closer
				}{bytes.NewReader(buffered), req.Body}
			} else {
				// Coraza copies into its own io.Writer buffer through CopyN,
				// then restores a MultiReader and delegates Close to the source.
				var buffered bytes.Buffer
				_, err := io.CopyN(struct{ io.Writer }{&buffered}, req.Body, test.read)
				if err != nil {
					require.ErrorIs(t, err, io.EOF)
				}
				reader := io.MultiReader(bytes.NewReader(buffered.Bytes()), req.Body)
				req.Body = struct {
					io.Reader
					io.WriterTo
					io.Closer
				}{reader, reader.(io.WriterTo), req.Body}
			}
			require.Nil(t, source.replay)
			if test.placeholder {
				require.Equal(t, "client-payload", repl.ReplaceAll("{http.request.body}", ""))
			}
			readBeforeSet := source.read
			if test.set {
				require.NoError(t, (requestbody.RequestBody{Set: "injected-payload"}).ServeHTTP(w, req, next))
				// Closing the replaced body must not drain the client stream.
				require.Equal(t, readBeforeSet, source.read)
			}
			body, complete := original.replayBody(req.Body)
			require.True(t, complete)
			payload, err := io.ReadAll(body)
			require.NoError(t, err)
			require.Equal(t, "client-payload", string(payload))
		})
	}
}

const (
	handoffPort        = "9085"
	bodyHandoffPort    = "9086"
	twoServersPort     = "9087"
	responseHeaderPort = "9088"
	opaqueHandoffPort  = "9089"
	metricsHandoffPort = "9090"
)

func reloadTestDial(t *testing.T, port string) (*bufio.Reader, func(string)) {
	t.Helper()
	conn, err := net.Dial("tcp", "localhost:"+port)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(30*time.Second)))
	return bufio.NewReader(conn), func(s string) {
		t.Helper()
		_, err := conn.Write([]byte(s))
		require.NoError(t, err)
	}
}

func reloadTestReadResponse(t *testing.T, reader *bufio.Reader) (*http.Response, string) {
	t.Helper()
	resp, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	return resp, string(body)
}

func reloadTestShutdownHook(name string) chan struct{} {
	shutdownHookFired := make(chan struct{})
	var hookOnce sync.Once
	RegisterWorkers(name, "../testdata/worker-with-counter.php", 1,
		frankenphp.WithWorkerOnServerShutdown(func() {
			hookOnce.Do(func() { close(shutdownHookFired) })
		}),
	)

	return shutdownHookFired
}

func reloadTestWaitHook(t *testing.T, hook chan struct{}) {
	t.Helper()
	select {
	case <-hook:
	case <-time.After(30 * time.Second):
		t.Fatal("the old runtime's shutdown hook never fired during the reload")
	}
}

func reloadTestWaitReloaded(t *testing.T, reloaded chan error) {
	t.Helper()
	select {
	case err := <-reloaded:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("the reload never completed")
	}
}

func TestReloadHandoffRejectsCommittedResponse(t *testing.T) {
	for _, mode := range []string{"header", "body", "read_from", "flush"} {
		t.Run(mode, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'];"), 0o600))
			hook := reloadTestShutdownHook("handoff-committed-" + mode)
			tester := caddytest.NewTester(t)
			oldConfig := strings.Replace(handoffConfig(root, "old", false), `"handler": "php",`,
				`"handler": "reload_test_write", "mode": "`+mode+`"}, {"handler": "php",`, 1)
			testserver.InitTestServer(t, tester, oldConfig, "json")

			reader, write := reloadTestDial(t, handoffPort)
			write("GET /index.php HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")
			reloaded := make(chan error, 1)
			go func() { reloaded <- caddy.Load([]byte(handoffConfig(root, "new", false)), true) }()
			reloadTestWaitHook(t, hook)
			write("\r\n")
			resp, body := reloadTestReadResponse(t, reader)
			status := http.StatusOK
			if mode == "header" {
				status = http.StatusAccepted
			}
			require.Equal(t, status, resp.StatusCode)
			require.Equal(t, "old", resp.Header.Get("X-Release"))
			require.NotContains(t, body, "new", "the replacement must not append its response to committed bytes")
			if mode == "body" || mode == "read_from" {
				require.True(t, strings.HasPrefix(body, "old:"), "the active chain must keep streaming normally")
			}
			reloadTestWaitReloaded(t, reloaded)
			tester.AssertGetResponse("http://localhost:"+handoffPort+"/index.php", http.StatusOK, "new")
		})
	}
}

func TestReloadHandoffReplaysThroughTheReplacementChain(t *testing.T) {
	for _, test := range []struct {
		name       string
		withAuth   bool
		oldHeaders string
		clientAuth bool
	}{
		{name: "auth", withAuth: true},
		{name: "plain"},
		{name: "injected_auth", withAuth: true, oldHeaders: `"set":{"Authorization":["Basic Ym9iOnMzY3IzdC1QNHNzdzByZA=="]}`},
		{name: "client_auth", withAuth: true, clientAuth: true, oldHeaders: `"delete":["Authorization"]`},
		{name: "host_rewrite", oldHeaders: `"set":{"Host":["trusted.example"]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'];"), 0o600))

			shutdownHookFired := make(chan struct{})
			var hookOnce sync.Once
			RegisterWorkers("handoff", "../testdata/worker-with-counter.php", 1,
				frankenphp.WithWorkerOnServerShutdown(func() {
					hookOnce.Do(func() { close(shutdownHookFired) })
				}),
			)

			tester := caddytest.NewTester(t)
			oldConfig := handoffConfig(root, "old", false)
			if test.oldHeaders != "" {
				oldConfig = strings.Replace(oldConfig, `"handler": "php",`,
					`"handler":"headers","request":{`+test.oldHeaders+`}}, {"handler":"php",`, 1)
			}
			testserver.InitTestServer(t, tester, oldConfig, "json")

			conn, err := net.Dial("tcp", "localhost:"+handoffPort)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			require.NoError(t, conn.SetDeadline(time.Now().Add(30*time.Second)))
			reader := bufio.NewReader(conn)
			write := func(s string) {
				t.Helper()
				_, err := conn.Write([]byte(s))
				require.NoError(t, err)
			}
			readResponse := func() (*http.Response, string) {
				t.Helper()
				resp, err := http.ReadResponse(reader, nil)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())

				return resp, string(body)
			}

			// The head parks without its blank line; never retry it: a retry would reach the new chain and pass vacuously.
			write("GET /index.php HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")
			if test.clientAuth {
				write("Authorization: Basic Ym9iOnMzY3IzdC1QNHNzdzByZA==\r\n")
			}

			reloaded := make(chan error, 1)
			go func() { reloaded <- caddy.Load([]byte(handoffConfig(root, "new", test.withAuth)), true) }()

			select {
			case <-shutdownHookFired:
			case <-time.After(30 * time.Second):
				t.Fatal("the old runtime's shutdown hook never fired during the reload")
			}

			write("\r\n")
			replayedResp, replayedBody := readResponse()
			if test.withAuth && !test.clientAuth {
				require.Equal(t, http.StatusUnauthorized, replayedResp.StatusCode,
					"the replayed request must pass through the replacement's basic_auth middleware")
				require.Equal(t, `Basic realm="frankenphp-test"`, replayedResp.Header.Get("WWW-Authenticate"))
			} else {
				require.Equal(t, http.StatusOK, replayedResp.StatusCode)
			}
			if test.clientAuth {
				require.Equal(t, http.StatusOK, replayedResp.StatusCode,
					"the client's own credentials must survive the old chain's header deletion")
			}
			if !test.withAuth {
				require.Equal(t, "new", replayedBody)
			}

			reloadTestWaitReloaded(t, reloaded)

			url := "http://localhost:" + handoffPort + "/index.php"
			if test.withAuth {
				freshResp, _ := tester.AssertGetResponse(url, http.StatusUnauthorized, "")
				require.Equal(t, `Basic realm="frankenphp-test"`, freshResp.Header.Get("WWW-Authenticate"))
				authReq, err := http.NewRequest(http.MethodGet, url, nil)
				require.NoError(t, err)
				authReq.SetBasicAuth("bob", "s3cr3t-P4ssw0rd")
				tester.AssertResponse(authReq, http.StatusOK, "new")
			} else {
				tester.AssertGetResponse(url, http.StatusOK, "new")
			}
		})
	}
}

func handoffConfig(root, marker string, withAuth bool) string {
	authentication := ""
	if withAuth {
		// cost 4: cost-12 bcrypt under the race detector outlasts the client timeout
		authentication = `{
						"handler": "authentication",
						"providers": {
							"http_basic": {
								"accounts": [{"username": "bob", "password": "$2a$04$Po19wSuEx9w7UCxlAtC2yusFnjXggkThvY15lkml9D.d9/QZoKXjG"}],
								"hash": {"algorithm": "bcrypt"},
								"realm": "frankenphp-test"
							}
						}
					},`
	}

	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									%[2]s
									{
										"handler": "php",
										"root": %[3]q,
										"env": {"MARKER": %[4]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, handoffPort, authentication, root, marker)
}

func TestReloadHandoffReplaysTheClientsOriginalRequest(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'] . $_SERVER['REQUEST_URI'];"), 0o600))

	shutdownHookFired := make(chan struct{})
	var hookOnce sync.Once
	RegisterWorkers("handoff-rewrite", "../testdata/worker-with-counter.php", 1,
		frankenphp.WithWorkerOnServerShutdown(func() {
			hookOnce.Do(func() { close(shutdownHookFired) })
		}),
	)

	tester := caddytest.NewTester(t)
	testserver.InitTestServer(t, tester, rewriteHandoffConfig(root, "old", false), "json")

	dial := func() (*bufio.Reader, func(string)) {
		conn, err := net.Dial("tcp", "localhost:"+handoffPort)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, conn.SetDeadline(time.Now().Add(30*time.Second)))
		reader := bufio.NewReader(conn)
		return reader, func(s string) {
			t.Helper()
			_, err := conn.Write([]byte(s))
			require.NoError(t, err)
		}
	}
	readResponse := func(reader *bufio.Reader) (*http.Response, string) {
		t.Helper()
		resp, err := http.ReadResponse(reader, nil)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		return resp, string(body)
	}

	blockedReader, writeBlocked := dial()
	writeBlocked("GET /blocked/secret HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n\r\n")
	firstResp, firstBody := readResponse(blockedReader)
	require.Equal(t, http.StatusOK, firstResp.StatusCode)
	require.Equal(t, "old/blocked/secret", firstBody)
	require.False(t, firstResp.Close, "the connection must stay open for the parked request")

	writeBlocked("GET /blocked/secret HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")
	helloReader, writeHello := dial()
	writeHello("GET /hello HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")

	reloaded := make(chan error, 1)
	go func() { reloaded <- caddy.Load([]byte(rewriteHandoffConfig(root, "new", true)), true) }()

	select {
	case <-shutdownHookFired:
	case <-time.After(30 * time.Second):
		t.Fatal("the old runtime's shutdown hook never fired during the reload")
	}

	// Complete both heads while the replacement is still starting: once the old HTTP server shuts down, late heads are dropped.
	writeBlocked("\r\n")
	writeHello("\r\n")

	blockedResp, blockedBody := readResponse(blockedReader)
	require.Equal(t, http.StatusForbidden, blockedResp.StatusCode,
		"the replayed request must be re-dispatched as the client sent it, so the replacement's path matcher applies")
	require.Empty(t, blockedBody)

	helloResp, helloBody := readResponse(helloReader)
	require.Equal(t, http.StatusOK, helloResp.StatusCode)
	require.Equal(t, "new/hello", helloBody)

	select {
	case err := <-reloaded:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("the reload never completed")
	}

	url := "http://localhost:" + handoffPort
	tester.AssertGetResponse(url+"/blocked/secret", http.StatusForbidden, "")
	tester.AssertGetResponse(url+"/hello", http.StatusOK, "new/hello")
}

func rewriteHandoffConfig(root, marker string, withBlockedRoute bool) string {
	blockedRoute := ""
	if withBlockedRoute {
		blockedRoute = `{
						"handle": [
							{
								"handler": "static_response",
								"status_code": 403
							}
						],
						"match": [{"host": ["localhost"], "path": ["/blocked*"]}],
						"terminal": true
					},`
	}

	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							%[2]s
							{
								"handle": [
									{
										"handler": "rewrite",
										"uri": "/index.php"
									},
									{
										"handler": "php",
										"root": %[3]q,
										"env": {"MARKER": %[4]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, handoffPort, blockedRoute, root, marker)
}

// TestReloadHandoffReplaysTheClientsRequestBody proves that a parked POST
// reaches the replacement with the client's body and length: the old chain's
// request_body middleware must not leak its injected payload (Set) nor its
// old limit (MaxSize) into the replay.
func TestReloadHandoffReplaysTheClientsRequestBody(t *testing.T) {
	for _, tt := range []struct {
		name       string
		oldHandler string
		firstBody  string
		refuse     bool
	}{
		{
			name:       "set",
			oldHandler: `{"handler":"request_body","set":"injected-payload"},`,
			firstBody:  "old:injected-payload",
		},
		{
			// the limit cuts "client-payload" to its first 6 bytes, "client"
			name:       "max_size",
			oldHandler: `{"handler":"request_body","max_size":6},`,
			firstBody:  "old:client",
		},
		{
			name:       "header_body",
			oldHandler: `{"handler":"headers","request":{"set":{"X-Body":["{http.request.body}"]}}},`,
			firstBody:  "old:client-payload",
		},
		{
			name:       "log_body",
			oldHandler: `{"handler":"log_append","key":"body","value":"{http.request.body}"},`,
			firstBody:  "old:client-payload",
		},
		{
			name:       "header_body_base64",
			oldHandler: `{"handler":"headers","request":{"set":{"X-Body":["{http.request.body_base64}"]}}},`,
			firstBody:  "old:client-payload",
		},
		{
			// the old chain reads the body past its limit, leaving no complete
			// copy for the replay: the handoff must refuse
			name:       "max_size_then_header_body",
			oldHandler: `{"handler":"request_body","max_size":6},{"handler":"headers","request":{"set":{"X-Body":["{http.request.body}"]}}},`,
			refuse:     true,
		},
		{
			name:       "header_body_then_set",
			oldHandler: `{"handler":"headers","request":{"set":{"X-Body":["{http.request.body}"]}}},{"handler":"request_body","set":"injected-payload"},`,
			firstBody:  "old:injected-payload",
		},
		{
			name:       "set_from_body",
			oldHandler: `{"handler":"request_body","set":"{http.request.body}"},`,
			firstBody:  "old:client-payload",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"),
				[]byte("<?php echo $_SERVER['MARKER'] . ':' . file_get_contents('php://input');"), 0o600))

			hook := reloadTestShutdownHook("handoff-body-" + tt.name)

			tester := caddytest.NewTester(t)
			testserver.InitTestServer(t, tester, bodyHandoffConfig(root, "old", tt.oldHandler), "json")

			// the old chain's middleware proves active on ordinary requests
			firstReader, writeFirst := reloadTestDial(t, bodyHandoffPort)
			writeFirst("POST /index.php HTTP/1.1\r\nHost: localhost:" + bodyHandoffPort + "\r\nContent-Length: 14\r\n\r\nclient-payload")
			resp, body := reloadTestReadResponse(t, firstReader)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			if tt.firstBody != "" {
				require.Equal(t, tt.firstBody, body)
			}

			// the head parks without its blank line; never retry it: a retry would reach the new chain and pass vacuously
			parkedReader, writeParked := reloadTestDial(t, bodyHandoffPort)
			writeParked("POST /index.php HTTP/1.1\r\nHost: localhost:" + bodyHandoffPort + "\r\nContent-Length: 14\r\n")

			reloaded := make(chan error, 1)
			go func() { reloaded <- caddy.Load([]byte(bodyHandoffConfig(root, "new", "")), true) }()

			reloadTestWaitHook(t, hook)

			// complete the head and send the client's body: the replacement has no request_body middleware
			writeParked("\r\nclient-payload")
			replayedResp, replayedBody := reloadTestReadResponse(t, parkedReader)
			if tt.refuse {
				require.Equal(t, http.StatusServiceUnavailable, replayedResp.StatusCode,
					"the replay must refuse rather than deliver a truncated body")
			} else {
				require.Equal(t, http.StatusOK, replayedResp.StatusCode)
				require.Equal(t, "new:client-payload", replayedBody,
					"the replayed request must carry the client's body, not the old chain's replacement or limit")
			}

			reloadTestWaitReloaded(t, reloaded)

			freshReq, err := http.NewRequest(http.MethodPost,
				"http://localhost:"+bodyHandoffPort+"/index.php", strings.NewReader("client-payload"))
			require.NoError(t, err)
			tester.AssertResponse(freshReq, http.StatusOK, "new:client-payload")
		})
	}
}

func bodyHandoffConfig(root, marker, requestBodyHandler string) string {
	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									%[2]s
									{
										"handler": "php",
										"root": %[3]q,
										"env": {"MARKER": %[4]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, bodyHandoffPort, requestBodyHandler, root, marker)
}

// TestReloadHandoffServesTwoPHPServersBehindOneHTTPServer proves that two
// php_server blocks on the same host, each behind its own path matcher, hand
// off to the replacement instead of being rejected as ambiguous: the replay
// re-dispatches the original request, which the path matchers separate.
func TestReloadHandoffServesTwoPHPServersBehindOneHTTPServer(t *testing.T) {
	apiRoot, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	webRoot, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(apiRoot, "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(webRoot, "web"), 0o755))
	php := "<?php echo $_SERVER['MARKER'];"
	require.NoError(t, os.WriteFile(filepath.Join(apiRoot, "api", "hello.php"), []byte(php), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(webRoot, "web", "hello.php"), []byte(php), 0o600))

	hook, resume := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	defer release()
	var hookOnce sync.Once
	RegisterWorkers("handoff-two-servers", "../testdata/worker-with-counter.php", 1,
		frankenphp.WithWorkerOnServerShutdown(func() {
			hookOnce.Do(func() {
				close(hook)
				<-resume
			})
		}),
	)
	state := &reloadWaitState{waiting: make(chan struct{}, 2)}
	activeReloadWait.Store(state)
	t.Cleanup(func() { activeReloadWait.CompareAndSwap(state, nil) })

	tester := caddytest.NewTester(t)
	oldConfig := twoPHPServersConfig(apiRoot, webRoot, "old-api", "old-web")
	oldConfig = strings.ReplaceAll(oldConfig, `"handler": "php",`,
		`"handler": "reload_test_wait"}, {"handler": "php",`)
	testserver.InitTestServer(t, tester, oldConfig, "json")

	firstReader, writeFirst := reloadTestDial(t, twoServersPort)
	writeFirst("GET /api/hello.php HTTP/1.1\r\nHost: localhost:" + twoServersPort + "\r\n\r\n")
	_, body := reloadTestReadResponse(t, firstReader)
	require.Equal(t, "old-api", body)
	writeFirst("GET /web/hello.php HTTP/1.1\r\nHost: localhost:" + twoServersPort + "\r\n\r\n")
	_, body = reloadTestReadResponse(t, firstReader)
	require.Equal(t, "old-web", body)

	// park one head per PHP server, both without their blank line; never retry them
	apiReader, writeApi := reloadTestDial(t, twoServersPort)
	writeApi("GET /api/hello.php HTTP/1.1\r\nHost: localhost:" + twoServersPort + "\r\nX-Reload-Test-Wait: 1\r\n")
	webReader, writeWeb := reloadTestDial(t, twoServersPort)
	writeWeb("GET /web/hello.php HTTP/1.1\r\nHost: localhost:" + twoServersPort + "\r\nX-Reload-Test-Wait: 1\r\n")

	reloaded := make(chan error, 1)
	go func() {
		reloaded <- caddy.Load([]byte(twoPHPServersConfig(apiRoot, webRoot, "new-api", "new-web")), true)
	}()

	reloadTestWaitHook(t, hook)

	writeApi("\r\n")
	writeWeb("\r\n")
	for range 2 {
		select {
		case <-state.waiting:
		case <-time.After(30 * time.Second):
			t.Fatal("both old requests must enter the PHP handoff wait")
		}
	}
	release()

	apiResp, apiBody := reloadTestReadResponse(t, apiReader)
	require.Equal(t, http.StatusOK, apiResp.StatusCode,
		"two PHP servers behind one HTTP server must hand off, not answer 503")
	require.Equal(t, "new-api", apiBody)

	webResp, webBody := reloadTestReadResponse(t, webReader)
	require.Equal(t, http.StatusOK, webResp.StatusCode)
	require.Equal(t, "new-web", webBody)

	reloadTestWaitReloaded(t, reloaded)

	tester.AssertGetResponse("http://localhost:"+twoServersPort+"/api/hello.php", http.StatusOK, "new-api")
	tester.AssertGetResponse("http://localhost:"+twoServersPort+"/web/hello.php", http.StatusOK, "new-web")
}

func twoPHPServersConfig(apiRoot, webRoot, apiMarker, webMarker string) string {
	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									{
										"handler": "php",
										"root": %[2]q,
										"env": {"MARKER": %[3]q}
									}
								],
								"match": [{"host": ["localhost"], "path": ["/api/*"]}],
								"terminal": true
							},
							{
								"handle": [
									{
										"handler": "php",
										"root": %[4]q,
										"env": {"MARKER": %[5]q}
									}
								],
								"match": [{"host": ["localhost"], "path": ["/*"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, twoServersPort, apiRoot, apiMarker, webRoot, webMarker)
}

// TestReloadHandoffAppliesTheReplacementResponseHeaders proves that the old
// chain's response state does not leak into the replay: an eager Add must not
// execute twice on unchanged configs, and a deferred Set from the old config
// must not override the replacement's value.
func TestReloadHandoffAppliesTheReplacementResponseHeaders(t *testing.T) {
	for _, tt := range []struct {
		name     string
		oldOps   string
		newOps   string
		first    []string
		replayed []string
	}{
		{
			name:     "eager_add",
			oldOps:   `{"add":{"X-Release":["same"]}}`,
			newOps:   `{"add":{"X-Release":["same"]}}`,
			first:    []string{"same"},
			replayed: []string{"same"},
		},
		{
			name:     "deferred_set",
			oldOps:   `{"set":{"X-Release":["old"]},"deferred":true}`,
			newOps:   `{"set":{"X-Release":["new"]},"deferred":true}`,
			first:    []string{"old"},
			replayed: []string{"new"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'];"), 0o600))

			hook := reloadTestShutdownHook("handoff-headers-" + tt.name)

			tester := caddytest.NewTester(t)
			testserver.InitTestServer(t, tester, headerHandoffConfig(root, "old", tt.oldOps), "json")

			firstReader, writeFirst := reloadTestDial(t, responseHeaderPort)
			writeFirst("GET /index.php HTTP/1.1\r\nHost: localhost:" + responseHeaderPort + "\r\n\r\n")
			resp, body := reloadTestReadResponse(t, firstReader)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "old", body)
			require.Equal(t, tt.first, resp.Header.Values("X-Release"))

			// the head parks without its blank line; never retry it: a retry would reach the new chain and pass vacuously
			parkedReader, writeParked := reloadTestDial(t, responseHeaderPort)
			writeParked("GET /index.php HTTP/1.1\r\nHost: localhost:" + responseHeaderPort + "\r\n")

			reloaded := make(chan error, 1)
			go func() { reloaded <- caddy.Load([]byte(headerHandoffConfig(root, "new", tt.newOps)), true) }()

			reloadTestWaitHook(t, hook)

			writeParked("\r\n")
			replayedResp, replayedBody := reloadTestReadResponse(t, parkedReader)
			require.Equal(t, http.StatusOK, replayedResp.StatusCode)
			require.Equal(t, "new", replayedBody)
			require.Equal(t, tt.replayed, replayedResp.Header.Values("X-Release"),
				"the replayed response must carry only the replacement's header operations")

			reloadTestWaitReloaded(t, reloaded)

			freshResp, _ := tester.AssertGetResponse("http://localhost:"+responseHeaderPort+"/index.php", http.StatusOK, "new")
			require.Equal(t, tt.replayed, freshResp.Header.Values("X-Release"))
		})
	}
}

func headerHandoffConfig(root, marker, responseOps string) string {
	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									{
										"handler": "headers",
										"response": %[2]s
									},
									{
										"handler": "php",
										"root": %[3]q,
										"env": {"MARKER": %[4]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, responseHeaderPort, responseOps, root, marker)
}

// TestReloadHandoffBypassesTheRetiredOpaqueResponseWriter proves that a parked
// request handed to the replacement is judged by the replacement's response
// rules: the retired chain's opaque response writer, which Unwrap cannot peel,
// must neither reject the replacement's response with the removed rules nor
// append its own completion writes to the client's stream, and the retired
// chain's deferred response state must not leak into the replay either.
func TestReloadHandoffBypassesTheRetiredOpaqueResponseWriter(t *testing.T) {
	for _, tt := range []struct {
		name       string
		oldOps     string
		newOps     string
		replayedXr string
	}{
		{name: "plain"},
		{
			name:       "deferred_headers",
			oldOps:     `{"handler": "headers", "response": {"set": {"X-Release": ["old"]}, "deferred": true}},`,
			newOps:     `{"handler": "headers", "response": {"set": {"X-Release": ["new"]}, "deferred": true}},`,
			replayedXr: "new",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'];"), 0o600))

			hook := reloadTestShutdownHook("handoff-opaque-waf-" + tt.name)

			tester := caddytest.NewTester(t)
			testserver.InitTestServer(t, tester, opaqueWAFHandoffConfig(root, "old", tt.oldOps, `"deny": "new"`), "json")

			// the retired WAF is active and lets the old chain's own responses through
			firstReader, writeFirst := reloadTestDial(t, opaqueHandoffPort)
			writeFirst("GET /index.php HTTP/1.1\r\nHost: localhost:" + opaqueHandoffPort + "\r\n\r\n")
			resp, body := reloadTestReadResponse(t, firstReader)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "old", body)
			if tt.oldOps != "" {
				require.Equal(t, "old", resp.Header.Get("X-Release"))
			}

			// the head parks without its blank line; never retry it: a retry would reach the new chain and pass vacuously
			parkedReader, writeParked := reloadTestDial(t, opaqueHandoffPort)
			writeParked("GET /index.php HTTP/1.1\r\nHost: localhost:" + opaqueHandoffPort + "\r\n")

			reloaded := make(chan error, 1)
			go func() { reloaded <- caddy.Load([]byte(opaqueWAFHandoffConfig(root, "new", tt.newOps, "")), true) }()

			reloadTestWaitHook(t, hook)

			writeParked("\r\n")
			replayedResp, replayedBody := reloadTestReadResponse(t, parkedReader)
			require.Equal(t, http.StatusOK, replayedResp.StatusCode,
				"the retired WAF's response rules must not reject the replacement's response")
			require.Equal(t, "new", replayedBody,
				"the client must receive exactly the replacement's body, with no completion writes appended")
			if tt.replayedXr != "" {
				require.Equal(t, tt.replayedXr, replayedResp.Header.Get("X-Release"),
					"the replayed response must carry only the replacement's response operations")
			}

			reloadTestWaitReloaded(t, reloaded)

			tester.AssertGetResponse("http://localhost:"+opaqueHandoffPort+"/index.php", http.StatusOK, "new")
		})
	}
}

func opaqueWAFHandoffConfig(root, marker, headerOps, waf string) string {
	wafHandler := ""
	if waf != "" {
		wafHandler = fmt.Sprintf(`{"handler": "reload_test_opaque_waf", %[1]s},`, waf)
	}

	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									%[2]s
									%[3]s
									{
										"handler": "php",
										"root": %[4]q,
										"env": {"MARKER": %[5]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, opaqueHandoffPort, headerOps, wafHandler, root, marker)
}

// TestReloadCapturePreservesHandlerMetrics proves the capture route does not
// perturb Caddy's per-route metrics: the instrumented user route keeps its own
// handler label and its count, and neither the internal capture route nor the
// wrapped routes add samples of their own.
func TestReloadCapturePreservesHandlerMetrics(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo 'metrics';"), 0o600))

	tester := caddytest.NewTester(t)
	testserver.InitTestServer(t, tester, metricsHandoffConfig(root), "json")

	tester.AssertGetResponse("http://localhost:"+metricsHandoffPort+"/index.php", http.StatusOK, "metrics")

	resp, err := tester.Client.Get("http://localhost:2998/metrics")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	metrics := string(body)

	require.Contains(t, metrics, `caddy_http_requests_total{handler="headers",server="srv0"} 1`,
		"the instrumented user route must keep its own handler label and count")
	require.NotContains(t, metrics, `handler=""`,
		"the unregistered capture handler must not be instrumented")
	require.NotContains(t, metrics, `handler="subroute"`,
		"the wrapped routes must not add a second instrumented route")
}

func metricsHandoffConfig(root string) string {
	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"metrics": {},
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									{
										"handler": "headers",
										"request": {"set": {"X-Test": ["capture"]}}
									},
									{
										"handler": "php",
										"root": %[2]q
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, metricsHandoffPort, root)
}
