//go:build !linux

package blockio

// Abort is a no-op off Linux: it relies on /proc/<tid>/syscall to
// find the fd a thread is blocked on. Elsewhere worker_timeout falls back to the
// VM interrupt plus the wake-up signal, which catches CPU-bound and
// EINTR-abortable waits but cannot unblock a socket read already in progress.
func Abort(tid int) {}
