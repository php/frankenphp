package memory

import "golang.org/x/sys/unix"

// TotalSysMemory returns the total physical memory in bytes via sysctl hw.memsize
func TotalSysMemory() uint64 {
	memsize, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}

	return memsize
}
