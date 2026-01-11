package cpu

import (
	"time"
)

// ProbeCPUs fallback that always determines that the CPU limits are not reached
func ProbeCPUs(probeTime time.Duration, _ float64, abort chan struct{}) bool {
	select {
	case <-abort:
		return false
	case <-time.After(probeTime):
		return true
	}
}
