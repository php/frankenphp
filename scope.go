package frankenphp

import (
	"strconv"
	"sync"
	"sync/atomic"
)

// Scope is an opaque per-php_server identifier. The zero value is the
// global/embed scope. Obtain values via NextScope.
//
// Scopes let metric series carry a "server" label so workers declared
// in distinct php_server blocks stay on distinct series. Future
// per-server features (per-server isolation, dispatching) can reuse the
// same identifier.
type Scope uint64

var scopeCounter atomic.Uint64

// NextScope returns a fresh scope value. Each php_server block should
// call this once during provisioning.
func NextScope() Scope {
	return Scope(scopeCounter.Add(1))
}

// scopeLabels maps Scope -> human-readable label registered by the embedder
// (e.g. the Caddy module). Read by ScopeLabel; written by SetScopeLabel.
var scopeLabels sync.Map

// SetScopeLabel attaches a human-readable label to a scope so metric/log
// emitters can render it as e.g. server="api.example.com" instead of an
// opaque numeric id. Empty labels are ignored. Embedders (Caddy module,
// custom hosts) own the labelling policy.
func SetScopeLabel(s Scope, label string) {
	if label == "" {
		return
	}
	scopeLabels.Store(s, label)
}

// ScopeLabel returns the label registered for s. When none is set
// (including the zero/global scope), it returns the numeric id so
// callers always get a non-empty value.
func ScopeLabel(s Scope) string {
	if v, ok := scopeLabels.Load(s); ok {
		return v.(string)
	}
	return strconv.FormatUint(uint64(s), 10)
}
