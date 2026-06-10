//go:build linux && !386

package blockio

import "syscall"

// connect/recvfrom/recvmsg are individual syscalls on these arches, so the fd a
// thread blocks on is the syscall's first argument.
const (
	sysConnect  int64 = syscall.SYS_CONNECT
	sysRecvfrom int64 = syscall.SYS_RECVFROM
	sysRecvmsg  int64 = syscall.SYS_RECVMSG
)
