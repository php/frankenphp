// Package blockio interrupts the blocking I/O a kernel thread is parked in.
//
// On Linux, Abort inspects /proc/self/task/<tid>/syscall to find the socket
// fd(s) the thread is blocked on (directly, through a poll set or through an
// epoll instance) and shuts them down, so the syscall fails terminally instead
// of being retried after an EINTR. The worker_timeout watchdog uses this to
// cut short requests stuck in an external call - a slow DB query, a hung
// Redis/HTTP read - whose socket is not reachable via PHP's resource list.
// On other platforms Abort is a no-op.
package blockio
