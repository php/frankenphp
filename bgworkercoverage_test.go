package frankenphp_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

// TestBackgroundWorkerCrashRestart covers the crash-recovery path: the
// worker publishes count=1, crashes, is auto-restarted, then publishes
// count=2. The reader polls get_vars() (which never blocks) and must
// eventually observe the post-restart snapshot. Along the way get_vars()
// returns the pre-crash snapshot, proving vars are kept in persistent
// memory across worker deaths.
func TestBackgroundWorkerCrashRestart(t *testing.T) {
	// Clean any stale marker from prior runs of this PID so the first boot
	// attempt is guaranteed to take the crash branch.
	matches, _ := filepath.Glob(os.TempDir() + "/bg-worker-crash-*")
	for _, m := range matches {
		_ = os.Remove(m)
	}

	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("crash-worker", "testdata/bgworker/crash.php", 0,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
frankenphp_ensure_background_worker('crash-worker', 5.0);
$vars = frankenphp_get_vars('crash-worker');
echo 'count=', $vars['count'] ?? 'MISSING', ' phase=', $vars['phase'] ?? 'MISSING';
`
	deadline := time.Now().Add(5 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		last = serveInlinePHP(t, testDataDir, "bg-crash-reader.php", php)
		if strings.Contains(last, "count=2") && strings.Contains(last, "phase=post-restart") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("did not observe post-restart snapshot within 5s; last=%q", last)
}

// TestBackgroundWorkerTypeValidation drives the set_vars() allow-list from
// inside a bg worker: int values and keys, nested arrays, and enum cases
// are accepted; objects and references are rejected with ValueError.
// The final published RESULTS string carries every branch outcome, and
// the enum case round-trips to the same ::class::$name on the reader.
func TestBackgroundWorkerTypeValidation(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("type-worker", "testdata/bgworker/types.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
enum BgTestStatus { case Active; case Inactive; }
for ($i = 0; $i < 200; $i++) {
    try {
        $vars = frankenphp_get_vars('type-worker');
        if (isset($vars['RESULTS'])) break;
    } catch (\RuntimeException $e) {}
    usleep(10000);
}
echo 'RESULTS=', $vars['RESULTS'] ?? 'MISSING', "\n";
echo 'ENUM_ROUNDTRIP=', (isset($vars['status']) && $vars['status'] === BgTestStatus::Active) ? 'match' : 'mismatch', "\n";
`
	out := serveInlinePHP(t, testDataDir, "bg-type-reader.php", php)

	assert.Contains(t, out, "INT_VAL:allowed")
	assert.Contains(t, out, "INT_KEY:allowed")
	assert.Contains(t, out, "NESTED:allowed")
	assert.Contains(t, out, "OBJECT:blocked")
	assert.Contains(t, out, "REFERENCE:blocked")
	assert.Contains(t, out, "ENUM_ROUNDTRIP=match", "enum case should restore to the same instance:\n"+out)
}

// TestBackgroundWorkerBinarySafe verifies that values pass through
// set_vars/get_vars byte-for-byte: embedded NUL, multibyte UTF-8 and
// empty string all survive the persistent-memory deep copy without
// truncation or re-encoding.
func TestBackgroundWorkerBinarySafe(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("binary-worker", "testdata/bgworker/binary.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
for ($i = 0; $i < 200; $i++) {
    try {
        $vars = frankenphp_get_vars('binary-worker');
        if (isset($vars['BINARY'])) break;
    } catch (\RuntimeException $e) {}
    usleep(10000);
}
echo 'BINARY_HEX=', bin2hex($vars['BINARY'] ?? ''), "\n";
echo 'BINARY_LEN=', strlen($vars['BINARY'] ?? ''), "\n";
echo 'UTF8=', $vars['UTF8'] ?? '', "\n";
echo 'EMPTY_EXISTS=', array_key_exists('EMPTY', $vars) ? '1' : '0', "\n";
echo 'EMPTY_LEN=', strlen($vars['EMPTY'] ?? 'missing'), "\n";
`
	out := serveInlinePHP(t, testDataDir, "bg-binary-reader.php", php)

	assert.Contains(t, out, "BINARY_HEX="+hex.EncodeToString([]byte("hello\x00world")))
	assert.Contains(t, out, "BINARY_LEN=11")
	assert.Contains(t, out, "UTF8=héllo wörld 🚀")
	assert.Contains(t, out, "EMPTY_EXISTS=1")
	assert.Contains(t, out, "EMPTY_LEN=0")
}

// TestBackgroundWorkerEnumMissing guards the generational deserializer:
// when the bg worker publishes an enum whose class is absent from the
// reader's process image, get_vars() must throw a LogicException rather
// than return a corrupt zval. The enum class is only declared in the bg
// worker entrypoint, so the reader's request has no way to reconstruct
// the case.
func TestBackgroundWorkerEnumMissing(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("enum-worker", "testdata/bgworker/enum-only.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
// WorkerOnlyEnum is intentionally NOT declared here.
for ($i = 0; $i < 200; $i++) {
    try {
        $vars = frankenphp_get_vars('enum-worker');
        echo 'NO_ERROR val_type=', get_debug_type($vars['val'] ?? null);
        return;
    } catch (\LogicException $e) {
        echo 'LogicException: ', $e->getMessage();
        return;
    } catch (\RuntimeException $e) {
        usleep(10000);
    }
}
echo 'TIMEOUT';
`
	out := serveInlinePHP(t, testDataDir, "bg-enum-missing-reader.php", php)

	assert.NotContains(t, out, "NO_ERROR", "enum should not have materialized:\n"+out)
	assert.NotContains(t, out, "TIMEOUT", "worker never published:\n"+out)
	assert.Contains(t, out, "LogicException")
	assert.Contains(t, out, "WorkerOnlyEnum", "missing class name must appear in the error:\n"+out)
}

// TestBackgroundWorkerSignalingStreamResource confirms that the value
// returned by frankenphp_get_worker_handle() is a real PHP stream
// resource. Complements the bounded-wall-clock force-kill test: that
// one proves the pipe closes on shutdown, this one proves the handle
// is a proper resource in the first place (not null, not an int, not
// a user object).
func TestBackgroundWorkerSignalingStreamResource(t *testing.T) {
	testDataDir := setupFrankenPHP(t,
		frankenphp.WithWorkers("stream-worker", "testdata/bgworker/stream-probe.php", 1,
			frankenphp.WithWorkerBackground()),
		frankenphp.WithNumThreads(3),
	)

	php := `<?php
for ($i = 0; $i < 200; $i++) {
    try {
        $vars = frankenphp_get_vars('stream-worker');
        if (isset($vars['stream_type'])) break;
    } catch (\RuntimeException $e) {}
    usleep(10000);
}
echo 'stream_type=', $vars['stream_type'] ?? 'MISSING', "\n";
echo 'is_resource=', var_export($vars['is_resource'] ?? 'MISSING', true), "\n";
`
	out := serveInlinePHP(t, testDataDir, "bg-stream-reader.php", php)

	assert.Contains(t, out, "stream_type=stream", "get_worker_handle() must return a stream resource:\n"+out)
	assert.Contains(t, out, "is_resource=true")
}
