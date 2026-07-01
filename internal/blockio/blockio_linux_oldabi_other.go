//go:build linux && (arm64 || riscv64 || loong64)

package blockio

// These newer ABIs never had dedicated poll/epoll_wait syscalls: the libc
// implements poll() via ppoll and epoll_wait() via epoll_pwait, which
// Abort matches directly. Impossible values keep the dedicated
// branches dead; readBlockingSyscall never returns a negative number.
const (
	sysPoll      int64 = -1
	sysEpollWait int64 = -1
)
