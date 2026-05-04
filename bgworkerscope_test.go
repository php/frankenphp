package frankenphp_test

import (
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

// TestBackgroundWorkerScopeIsolation declares two bg workers with the
// *same* name in two distinct scopes. Requests scoped to each block must
// resolve to their own worker, proving per-php_server isolation works.
func TestBackgroundWorkerScopeIsolation(t *testing.T) {
	scopeA := frankenphp.NextScope()
	scopeB := frankenphp.NextScope()

	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("shared", "testdata/bgworker/scope-a.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerScope(scopeA)),
		frankenphp.WithWorkers("shared", "testdata/bgworker/scope-b.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerScope(scopeB)),
		frankenphp.WithNumThreads(4),
	)

	// Each scope's worker publishes its own marker under the same name
	// ("shared"). The reader script reads get_vars('shared'); scope
	// selection on the request determines which worker is resolved.
	bodyA := serveBody(t, testDataDir, "bgworker/scope-reader.php",
		frankenphp.WithRequestScope(scopeA))
	bodyB := serveBody(t, testDataDir, "bgworker/scope-reader.php",
		frankenphp.WithRequestScope(scopeB))

	assert.Contains(t, bodyA, "scope=A", "scopeA request should resolve to worker-scope-a:\n"+bodyA)
	assert.Contains(t, bodyB, "scope=B", "scopeB request should resolve to worker-scope-b:\n"+bodyB)
}
