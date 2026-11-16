package frankenphp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A worker uses opcache's shared memory for as long as its script runs, so
// its script restarts when opcache schedules a restart of that memory.
func TestOpcacheRestartRestartsWorkers(t *testing.T) {
	if frankenphp.Version().VersionID < 80400 {
		t.Skip("PHP reports opcache restarts since 8.4")
	}

	dir := t.TempDir()

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		type status struct {
			Token    string
			Cached   bool
			Pending  bool
			Restarts int
			Corrupt  bool
		}
		get := func(batch int) (s status) {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest("GET", "http://example.com/opcache/worker.php?dir="+dir+"&batch="+strconv.Itoa(batch), nil))
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &s), w.Body.String())

			return s
		}

		before := get(0)
		if !before.Cached {
			t.Skip("opcache does not cache the worker")
		}

		scheduled := before
		for batch := 1; batch <= 20 && !scheduled.Pending; batch++ {
			scheduled = get(batch)
		}
		if !scheduled.Pending {
			t.Skip("could not make opcache schedule a restart")
		}

		var after status
		if !assert.Eventually(t, func() bool {
			after = get(0)

			return after.Restarts > 0 && after.Cached && after.Token != before.Token
		}, 10*time.Second, 10*time.Millisecond) {
			t.Logf("before: %+v, after: %+v", before, after)
		}
		assert.False(t, after.Corrupt)
	}, &testOptions{
		workerScript:       "opcache/worker.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		phpIni: map[string]string{
			"opcache.enable":                  "1",
			"opcache.memory_consumption":      "8",
			"opcache.interned_strings_buffer": "1",
			"opcache.max_accelerated_files":   "200",
			"opcache.max_wasted_percentage":   "1",
			"opcache.validate_timestamps":     "1",
			"opcache.revalidate_freq":         "0",
			"opcache.file_update_protection":  "0",
		},
	})
}
