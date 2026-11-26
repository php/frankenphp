package memory

import (
	"math"
	"runtime/debug"
)

// Available returns the memory the process may use, 0 if it cannot be
// determined: the total system memory, or the memory limit of the Go runtime
// when it is lower. In a container, the system memory is that of the host,
// while the limit comes from the container: Caddy derives it from the cgroup,
// GOMEMLIMIT sets it explicitly.
func Available() uint64 {
	total := TotalSysMemory()
	limit := debug.SetMemoryLimit(-1)
	if limit <= 0 || limit == math.MaxInt64 {
		return total
	}
	if total == 0 || uint64(limit) < total {
		return uint64(limit)
	}

	return total
}
