//go:build linux && !arm64 && !riscv64 && !loong64

package blockio

import "syscall"

// Older Linux ABIs ship dedicated poll and epoll_wait syscalls next to
// ppoll/epoll_pwait, and the libc prefers them: glibc and musl implement
// poll() via SYS_POLL and epoll_wait() via SYS_EPOLL_WAIT on these arches
// (e.g. amd64, 386, arm), so the watchdog must match them — a PHP thread
// parked in a stream poll on amd64 sits in SYS_POLL, not SYS_PPOLL.
const (
	sysPoll      int64 = syscall.SYS_POLL
	sysEpollWait int64 = syscall.SYS_EPOLL_WAIT
)
