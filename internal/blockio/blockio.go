package blockio

import "log/slog"

// logger, when set, is used to report (once) that the platform denies a
// syscall Abort relies on, so a silently degraded worker_timeout (e.g. under
// a seccomp policy that blocks process_vm_readv) is visible in the logs.
var logger *slog.Logger

// SetLogger sets the logger used for the one-time degradation warning.
// Optional: without it, degradation stays silent. Not safe for concurrent use
// with Abort; call it during initialization.
func SetLogger(l *slog.Logger) {
	logger = l
}
