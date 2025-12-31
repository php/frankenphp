package frankenphp_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"sync"
	"testing"
)

func newTestLogger(t *testing.T) (*slog.Logger, fmt.Stringer) {
	t.Helper()

	var buf syncBuffer

	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

// SyncBuffer is a thread-safe buffer for capturing logs in tests.
type syncBuffer struct {
	b  bytes.Buffer
	mu sync.RWMutex
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.b.String()
}
