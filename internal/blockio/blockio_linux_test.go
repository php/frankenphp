//go:build linux

package blockio

import (
	"os"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadProcessMemory verifies the process_vm_readv path reads our own memory
// back faithfully - this is what recovers the struct pollfd array for the ppoll
// case (replacing the /proc/<tid>/mem file read).
func TestReadProcessMemory(t *testing.T) {
	want := []byte("frankenphp-process_vm_readv-roundtrip")
	got := make([]byte, len(want))

	require.True(t, readProcessMemory(uintptr(unsafe.Pointer(&want[0])), got))
	assert.Equal(t, want, got)

	// A zero-length read is rejected (nothing to shut down, no &buf[0]).
	assert.False(t, readProcessMemory(uintptr(unsafe.Pointer(&want[0])), nil))
}

// TestIsSocketFD verifies we only ever classify real sockets as shutdown
// targets, so a misread argument can never close a file or pipe.
func TestIsSocketFD(t *testing.T) {
	s, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	defer syscall.Close(s)
	assert.True(t, isSocketFD(s), "a socket fd must be recognised")

	f, err := os.CreateTemp(t.TempDir(), "notasocket")
	require.NoError(t, err)
	defer f.Close()
	assert.False(t, isSocketFD(int(f.Fd())), "a regular file must not be a socket")

	assert.False(t, isSocketFD(-1), "an invalid fd must not be a socket")
}

// TestEpollMonitoredFDs verifies we can enumerate the descriptors registered in
// an epoll instance from /proc/self/fdinfo/<epfd> - the basis for aborting
// curl_multi/gRPC-style clients parked in epoll_wait, with no memory read.
func TestEpollMonitoredFDs(t *testing.T) {
	epfd, err := syscall.EpollCreate1(0)
	require.NoError(t, err)
	defer syscall.Close(epfd)

	s, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	defer syscall.Close(s)

	ev := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(s)}
	require.NoError(t, syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, s, &ev))

	assert.Contains(t, epollMonitoredFDs(epfd), s,
		"the registered socket must be discovered via fdinfo")
	assert.Empty(t, epollMonitoredFDs(-1))
}
