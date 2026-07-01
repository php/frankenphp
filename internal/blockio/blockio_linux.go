//go:build linux

package blockio

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// maxWatchedFDs bounds how many descriptors we will read out of a poll set or an
// epoll instance, so a misread count can never make us scan or shut down an
// unreasonable number of fds. It is generous on purpose: a client doing a
// dual-stack connect or a small fan-out poll legitimately watches a handful of
// fds (this is NOT clamped to 1 - that would only fit mysqlnd's single-socket
// poll and miss curl, gRPC and friends).
const maxWatchedFDs = 256

// Abort best-effort interrupts the blocking I/O the given kernel thread is
// parked in, by shutting down the socket fd(s) so the syscall returns
// terminally instead of being retried after the EINTR our wake-up delivers. This
// is what lets worker_timeout actually cut short a request stuck in an external
// call - a slow DB query (mysqlnd), a hung Redis/Elasticsearch/HTTP read, a
// black-holed connect - whose socket is not reachable via PHP's resource list.
//
// It reads /proc/self/task/<tid>/syscall to learn the blocked syscall and its
// arguments and handles, in order of how PHP actually blocks:
//
//   - read / recvfrom / recvmsg / connect: the fd is the first argument.
//   - poll / ppoll: arg0 points to a struct pollfd array (PHP's stream layer,
//     and thus Redis/HTTP/DB clients riding on it, always poll before recv); we
//     read the array out of the process's own memory with process_vm_readv(2).
//     Both syscalls must be matched: glibc and musl implement poll() via the
//     dedicated poll syscall on arches that have one (e.g. amd64) and via ppoll
//     only where they don't (e.g. arm64).
//   - epoll_wait / epoll_pwait: arg0 is the epoll fd; the watched fds are not in
//     the syscall arguments at all, so we enumerate them from
//     /proc/self/fdinfo/<epfd> (covers curl_multi, gRPC and other own-loop
//     clients).
//
// Every fd is confirmed to be a socket before shutdown, and after recovering a
// pointer-derived fd we re-read /proc/.../syscall to confirm the thread is still
// parked in the same syscall on the same argument before acting - so a stale
// pointer or a reused fd cannot make us shut down an unrelated descriptor. No-op
// if the thread is not in a recognised blocking syscall (CPU-bound overruns are
// handled by the VM interrupt instead).
func Abort(tid int) {
	if tid <= 0 {
		return
	}

	nr, args, ok := readBlockingSyscall(tid)
	if !ok {
		return
	}

	// switch on booleans (not constant labels): sysConnect/sysRecvfrom/sysRecvmsg,
	// sysPoll and sysEpollWait collapse to the same impossible value on arches that
	// lack the corresponding syscall, which would be a duplicate case in a value
	// switch.
	switch {
	case nr == syscall.SYS_READ || nr == sysRecvfrom || nr == sysRecvmsg || nr == sysConnect:
		// arg0 is the fd.
		fd := int(int32(args[0]))
		if isSocketFD(fd) && stillBlockedIn(tid, nr, args[0]) {
			shutdownSocket(fd)
		}
	case nr == sysPoll || nr == syscall.SYS_PPOLL:
		// arg0 -> struct pollfd array, arg1 = nfds (identical layout for both).
		// poll exists only on the older ABIs (e.g. amd64); sysPoll is an
		// impossible value elsewhere, leaving this branch ppoll-only there.
		shutdownPollFDs(tid, nr, args[0], args[1])
	case nr == sysEpollWait || nr == syscall.SYS_EPOLL_PWAIT:
		// arg0 is the epoll fd. Like poll, epoll_wait exists only on the older
		// ABIs; sysEpollWait is an impossible value elsewhere, leaving this
		// branch epoll_pwait-only there.
		shutdownEpollFDs(tid, nr, args[0])
	}
}

// readBlockingSyscall reads /proc/self/task/<tid>/syscall and returns the syscall
// number and its arguments. It reports ok == false when the thread is not
// currently in a syscall ("running", or a negative number).
func readBlockingSyscall(tid int) (nr int64, args [6]uint64, ok bool) {
	data, err := os.ReadFile("/proc/self/task/" + strconv.Itoa(tid) + "/syscall")
	if err != nil {
		return 0, args, false
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, args, false
	}

	// fields[0] is the syscall number ("running"/"-1" when not in a syscall),
	// followed by the (hex) arguments.
	nr, err = strconv.ParseInt(fields[0], 10, 64)
	if err != nil || nr < 0 {
		return 0, args, false
	}

	for i := 0; i < len(args) && i+1 < len(fields); i++ {
		args[i] = parseSyscallArg(fields[i+1])
	}

	return nr, args, true
}

// stillBlockedIn re-reads the thread's syscall and reports whether it is still
// parked in the same syscall on the same first argument. We call this after
// recovering an fd from a pointer (or from a process-wide fd table) and right
// before shutting it down, to shrink the window in which the thread could have
// returned and the fd been reused for something else.
func stillBlockedIn(tid int, nr int64, arg0 uint64) bool {
	nr2, args2, ok := readBlockingSyscall(tid)

	return ok && nr2 == nr && args2[0] == arg0
}

func parseSyscallArg(s string) uint64 {
	v, err := strconv.ParseUint(s, 0, 64) // base 0 handles the 0x prefix

	if err != nil {
		return 0
	}

	return v
}

// shutdownPollFDs reads nfds struct pollfd entries from the process's own memory
// at ptr and shuts down each socket fd. struct pollfd is { int fd; short events;
// short revents; } = 8 bytes, fd at offset 0, little-endian on the supported
// arches.
func shutdownPollFDs(tid int, nr int64, ptr, nfds uint64) {
	if nfds == 0 || nfds > maxWatchedFDs {
		return
	}

	buf := make([]byte, nfds*8)
	if !readProcessMemory(uintptr(ptr), buf) {
		return
	}

	// Confirm the thread is still parked in this poll on the same array before we
	// trust the bytes we just read and start shutting fds down.
	if !stillBlockedIn(tid, nr, ptr) {
		return
	}

	for i := uint64(0); i < nfds; i++ {
		fd := int(int32(binary.LittleEndian.Uint32(buf[i*8:])))
		if isSocketFD(fd) {
			shutdownSocket(fd)
		}
	}
}

// shutdownEpollFDs enumerates the descriptors registered in the epoll instance
// epfd (from /proc/self/fdinfo/<epfd>, which lists "tfd: <fd>" entries) and shuts
// down those that are sockets. This needs no memory read: the watched fds live
// inside the kernel epoll object, not in the syscall arguments.
func shutdownEpollFDs(tid int, nr int64, epfd uint64) {
	fd := int(int32(epfd))
	fds := epollMonitoredFDs(fd)
	if len(fds) == 0 || !stillBlockedIn(tid, nr, epfd) {
		return
	}

	for _, watched := range fds {
		if isSocketFD(watched) {
			shutdownSocket(watched)
		}
	}
}

func epollMonitoredFDs(epfd int) []int {
	if epfd < 0 {
		return nil
	}

	f, err := os.Open("/proc/self/fdinfo/" + strconv.Itoa(epfd))
	if err != nil {
		return nil
	}
	defer f.Close()

	var fds []int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// Lines look like: "tfd:        5 events:       19 data: ..."
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[0] != "tfd:" {
			continue
		}
		if fd, err := strconv.Atoi(fields[1]); err == nil {
			fds = append(fds, fd)
		}
		if len(fds) >= maxWatchedFDs {
			break
		}
	}

	return fds
}

// readProcessMemory copies len(buf) bytes from our own address space at
// remoteAddr into buf using process_vm_readv(2). We read from this process (all
// threads share the address space), so no ptrace privilege is required; the call
// simply fails closed under a seccomp policy that blocks it (and the
// degradation is reported once, see reportPolicyError).
func readProcessMemory(remoteAddr uintptr, buf []byte) bool {
	if len(buf) == 0 {
		return false
	}

	local := unix.Iovec{Base: &buf[0]}
	local.SetLen(len(buf))
	remote := unix.RemoteIovec{Base: remoteAddr, Len: len(buf)}

	n, err := unix.ProcessVMReadv(os.Getpid(), []unix.Iovec{local}, []unix.RemoteIovec{remote}, 0)
	if err != nil {
		reportPolicyError(err)

		return false
	}

	return n == len(buf)
}

// policyOnce guards the one-time warning emitted when the platform denies
// process_vm_readv altogether.
var policyOnce sync.Once

// reportPolicyError logs (once) when process_vm_readv is denied by policy
// rather than failing transiently: under a seccomp profile that blocks the
// syscall (Docker's default profile only allows it on kernels >= 4.8, and
// gVisor or stricter custom profiles may not at all) the poll-set path of the
// watchdog is unavailable, so a request blocked in a poll-based socket read
// cannot be aborted by worker_timeout. Transient errors (ESRCH when the
// thread returned, EFAULT on a stale pointer) are expected and stay silent.
func reportPolicyError(err error) {
	if !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.EACCES) && !errors.Is(err, unix.ENOSYS) {
		return
	}

	policyOnce.Do(func() {
		if logger != nil {
			logger.LogAttrs(context.Background(), slog.LevelWarn,
				"worker_timeout: process_vm_readv is denied (seccomp policy?); a request blocked in a poll-based socket read cannot be aborted",
				slog.Any("error", err),
			)
		}
	})
}

// isSocketFD reports whether fd is a socket, so a misclassified argument can
// never make us shut down an unrelated descriptor (a file, a pipe, ...).
func isSocketFD(fd int) bool {
	if fd < 0 {
		return false
	}

	target, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))

	return err == nil && strings.HasPrefix(target, "socket:")
}

func shutdownSocket(fd int) {
	_ = syscall.Shutdown(fd, syscall.SHUT_RDWR)
}
