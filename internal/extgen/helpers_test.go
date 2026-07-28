package extgen

import (
	"bytes"
	"testing"
)

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	original := warnOut
	warnOut = &buf
	t.Cleanup(func() { warnOut = original })
	return &buf
}
