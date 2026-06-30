package frankenphp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNextScopeIsDistinct verifies the scope counter
// hands out unique values on consecutive calls.
func TestNextScopeIsDistinct(t *testing.T) {
	a := NextScope()
	b := NextScope()
	assert.NotEqual(t, a, b, "consecutive scopes must differ")
	assert.NotZero(t, a, "scopes must be non-zero (zero is the global scope)")
	assert.NotZero(t, b, "scopes must be non-zero (zero is the global scope)")
}

// TestBackgroundWorkerSameNameDifferentScope declares two named bg
// workers with the same user-facing name in distinct scopes. Both must
// Init successfully (the global workersByName collision check must
// recognize bg workers as scope-isolated).
func TestBackgroundWorkerSameNameDifferentScope(t *testing.T) {
	scopeA := NextScope()
	scopeB := NextScope()

	tmp := t.TempDir()

	setupBgWorker(t,
		WithWorkers("shared", "testdata/bgworker/named.php", 1,
			WithWorkerBackground(),
			WithWorkerScope(scopeA),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp + "/a"}),
		),
		WithWorkers("shared", "testdata/bgworker/named.php", 1,
			WithWorkerBackground(),
			WithWorkerScope(scopeB),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": tmp + "/b"}),
		),
		WithNumThreads(4),
	)

	// Both lookups must exist and resolve "shared" to a *worker.
	require.NotNil(t, backgroundLookups[scopeA], "scope A lookup missing")
	require.NotNil(t, backgroundLookups[scopeB], "scope B lookup missing")
	assert.NotNil(t, backgroundLookups[scopeA].byName["shared"], "scope A must resolve 'shared'")
	assert.NotNil(t, backgroundLookups[scopeB].byName["shared"], "scope B must resolve 'shared'")
	// And they must be distinct *worker instances (not the same pointer).
	assert.NotSame(t,
		backgroundLookups[scopeA].byName["shared"],
		backgroundLookups[scopeB].byName["shared"],
		"each scope must own a distinct *worker for the same name")
}

// TestBackgroundWorkerCatchAllPerScope declares a catch-all in two
// distinct scopes and verifies that ensure() in scope A consumes a slot
// from scope A's catch-all only, leaving scope B's catch-all untouched.
func TestBackgroundWorkerCatchAllPerScope(t *testing.T) {
	scopeA := NextScope()
	scopeB := NextScope()

	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "a")
	dirB := filepath.Join(tmp, "b")
	require.NoError(t, os.MkdirAll(dirA, 0o755))
	require.NoError(t, os.MkdirAll(dirB, 0o755))

	setupBgWorker(t,
		WithWorkers("", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerScope(scopeA),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dirA}),
		),
		WithWorkers("", "testdata/bgworker/named.php", 0,
			WithWorkerBackground(),
			WithWorkerScope(scopeB),
			WithWorkerEnv(map[string]string{"BG_SENTINEL_DIR": dirB}),
		),
		WithNumThreads(4),
	)

	// Pre-conditions: each scope's catch-all is empty.
	require.Empty(t, backgroundLookups[scopeA].catchAll.bg.catchAllNames, "scope A catch-all must start empty")
	require.Empty(t, backgroundLookups[scopeB].catchAll.bg.catchAllNames, "scope B catch-all must start empty")

	// Drive ensure() through a fake "request" context tagged to scope A.
	// We use a request-scope tag (rather than a worker handler) because
	// no scope-specific handler is running in the test.
	fc := newFrankenPHPContext()
	fc.scope = scopeA
	ctx := context.WithValue(context.Background(), contextKey, fc)
	thread := &phpThread{}
	thread.handler = &fakeContextThread{ctx: ctx}
	require.NoError(t, ensureBackgroundWorker(thread, "job-a", 5*time.Second))

	// The lazy-started instance must land in scope A's catch-all only.
	require.Eventually(t, func() bool {
		_, ok := backgroundLookups[scopeA].catchAll.bg.catchAllNames["job-a"]
		return ok
	}, 2*time.Second, 10*time.Millisecond, "ensure() must populate scope A's catch-all")
	assert.Empty(t, backgroundLookups[scopeB].catchAll.bg.catchAllNames, "scope B catch-all must remain untouched")

	// All catch-all threads attach to the scope's catch-all *worker, so
	// scope A's catch-all should have one thread (for "job-a") while
	// scope B's catch-all should have none.
	assert.Equal(t, 1, backgroundLookups[scopeA].catchAll.countThreads(), "scope A catch-all must host exactly one thread")
	assert.Equal(t, 0, backgroundLookups[scopeB].catchAll.countThreads(), "scope B catch-all must host zero threads")

	// Wait for the worker to write its sentinel so we know the right
	// fixture ran (sanity check that env was inherited from scope A).
	requireSentinelEventually(t, filepath.Join(dirA, "job-a"),
		"scope A catch-all instance must write its sentinel under scope A's BG_SENTINEL_DIR")
}

// fakeContextThread is a threadHandler stub that lets a test drive
// ensureBackgroundWorker via the request-context fallback path in
// getLookup, without spinning up a real PHP thread.
type fakeContextThread struct {
	ctx context.Context
}

func (h *fakeContextThread) name() string                          { return "fake" }
func (h *fakeContextThread) beforeScriptExecution() string         { return "" }
func (h *fakeContextThread) afterScriptExecution(int)              {}
func (h *fakeContextThread) frankenPHPContext() *frankenPHPContext { return nil }
func (h *fakeContextThread) drain()                                {}
func (h *fakeContextThread) context() context.Context              { return h.ctx }
func (h *fakeContextThread) scopedWorker() *worker                 { return nil }
