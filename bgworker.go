package frankenphp

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Scope isolates background workers between php_server blocks; the
// zero value is the global/embed scope. Obtain values via NextScope.
type Scope uint64

var scopeCounter atomic.Uint64

// NextScope returns a fresh scope value. Each php_server block should
// call this once during provisioning.
func NextScope() Scope {
	return Scope(scopeCounter.Add(1))
}

// scopeLabels maps Scope -> human-readable label registered by the
// embedder (e.g. the Caddy module).
var scopeLabels sync.Map

// SetScopeLabel attaches a human-readable label to a scope; the bg-worker
// metric emitter renders it as e.g. server="api.example.com" instead of
// an opaque numeric id. Empty labels are ignored.
func SetScopeLabel(s Scope, label string) {
	if label == "" {
		return
	}
	scopeLabels.Store(s, label)
}

// lookupScopeLabel reports whether a label has been registered for s,
// returning ("", false) when none has. Distinguishes "unset" from
// "explicitly empty" without the numeric fallback.
func lookupScopeLabel(s Scope) (string, bool) {
	v, ok := scopeLabels.Load(s)
	if !ok {
		return "", false
	}
	return v.(string), true
}

// bgWorkerMetricName formats the metric label for a background worker:
// "m#<scopeLabel>:<runtimeName>". scopeLabel is empty when the scope
// has no registered label (embed/global, or before the embedder calls
// SetScopeLabel). The "m#" prefix mirrors the m# convention used for
// module workers; the colon keeps the format uniform so a single regex
// (m#([^:]*):(.+)) parses both labelled and unlabelled forms.
func bgWorkerMetricName(scope Scope, runtimeName string) string {
	label, _ := lookupScopeLabel(scope)
	return "m#" + label + ":" + runtimeName
}

// backgroundLookups maps scope -> name -> *worker. Scope 0 is the
// global/embed scope. nil when no background worker is declared.
var backgroundLookups map[Scope]map[string]*worker

// buildBackgroundWorkerLookups maps each declared bg worker into its scope's
// lookup. Per-scope name collisions are caught here because bg workers
// intentionally skip the global workersByName map (so two scopes can share
// a user-facing name). Names are not allowed to be empty in this minimal
// build; catch-all bg workers are deferred to a follow-up PR.
func buildBackgroundWorkerLookups(workers []*worker, opts []workerOpt) (map[Scope]map[string]*worker, error) {
	lookups := make(map[Scope]map[string]*worker)

	for i, o := range opts {
		if !o.isBackgroundWorker {
			continue
		}
		w := workers[i]
		w.scope = o.scope

		phpName := strings.TrimPrefix(w.name, "m#")
		if phpName == "" || phpName == w.fileName {
			return nil, fmt.Errorf("background worker must have an explicit name (got %q)", w.name)
		}

		byName := lookups[o.scope]
		if byName == nil {
			byName = make(map[string]*worker)
			lookups[o.scope] = byName
		}
		if _, exists := byName[phpName]; exists {
			return nil, fmt.Errorf("duplicate background worker name %q in the same scope", phpName)
		}
		byName[phpName] = w
	}

	if len(lookups) == 0 {
		return nil, nil
	}
	return lookups, nil
}

// reserveBackgroundWorkerThreads returns the thread budget to add to the
// pool for declared bg workers, and pre-registers totalWorkers so a bg-only
// deployment has the metric initialised. num must be >= 1 for bg workers
// in this build.
func reserveBackgroundWorkerThreads(opt *opt) (int, error) {
	reserved := 0
	for _, w := range opt.workers {
		if !w.isBackgroundWorker {
			continue
		}
		if w.num < 1 {
			return 0, fmt.Errorf("background worker %q must declare num >= 1 (lazy/ensure() machinery is not in this build)", w.name)
		}
		reserved += w.num
		metrics.TotalWorkers(bgWorkerMetricName(w.scope, w.name), w.num)
	}
	return reserved, nil
}
