package memory

import (
	"math"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setMemoryLimit(t *testing.T, limit int64) {
	previous := debug.SetMemoryLimit(limit)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
}

func TestAvailableIsTheSystemMemoryWithoutLimit(t *testing.T) {
	setMemoryLimit(t, math.MaxInt64)

	assert.Equal(t, TotalSysMemory(), Available())
}

func TestAvailableIsTheMemoryLimitWhenLower(t *testing.T) {
	setMemoryLimit(t, 512<<20)

	assert.Equal(t, uint64(512<<20), Available())
}

func TestAvailableIsTheSystemMemoryWhenTheLimitIsHigher(t *testing.T) {
	total := TotalSysMemory()
	if total == 0 || total > math.MaxInt64/2 {
		t.Skip("the system memory is unknown")
	}
	setMemoryLimit(t, int64(total)*2)

	assert.Equal(t, total, Available())
}
