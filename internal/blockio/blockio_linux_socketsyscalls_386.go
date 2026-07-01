//go:build linux && 386

package blockio

// linux/386 multiplexes socket operations through socketcall(2), so connect,
// recvfrom and recvmsg are not distinct syscalls and Go's syscall package does
// not define their numbers. Set them to an impossible value so those branches
// never match; readBlockingSyscall never returns a negative number. This loses
// little in practice: PHP's stream layer (and the DB/Redis/HTTP clients built on
// it) always polls before reading, so the poll/ppoll path still aborts blocking
// reads on 386.
const (
	sysConnect  int64 = -1
	sysRecvfrom int64 = -1
	sysRecvmsg  int64 = -1
)
