package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNextAlignedTick(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 34, 56, 0, time.UTC)

	next := nextAlignedTick(time.Minute, now)
	assert.Equal(t, time.Date(2026, 7, 10, 12, 35, 0, 0, time.UTC), next)

	next = nextAlignedTick(time.Hour, now)
	assert.Equal(t, time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC), next)
}
