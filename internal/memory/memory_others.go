//go:build !linux && !darwin && !windows

package memory

// TotalSysMemory returns 0 if the total system memory cannot be determined
func TotalSysMemory() uint64 {
	return 0
}
