package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setlocale() must only change the locale of the current thread, and each
// request must start with the process locale.
func TestSetlocale(t *testing.T) { testSetlocale(t, &testOptions{}) }

func TestSetlocale_worker(t *testing.T) {
	testSetlocale(t, &testOptions{workerScript: "setlocale.php"})
}

func testSetlocale(t *testing.T, opts *testOptions) {
	// The process locale isn't always "C": the environment (LANG) applies to
	// it when PHP or an extension calls setlocale(LC_ALL, "")
	var processLocale string
	var once sync.Once

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// Read before any request changes its locale, other requests wait
		once.Do(func() {
			processLocale, _ = testGet("http://example.com/setlocale.php?locale=0", handler, t)
		})

		locale := "C"
		if i%2 == 0 {
			locale = "other"
		}

		body, _ := testGet("http://example.com/setlocale.php?locale="+locale, handler, t)
		parts := strings.Split(body, "|")
		require.Len(t, parts, 6, body)
		initial, set, current, float, decimalPoint, restored := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]

		assert.Equal(t, processLocale, initial, "a request starts with the process locale")
		assert.NotEmpty(t, set, "setlocale() failed")
		assert.Equal(t, set, current, "other requests don't change the locale")
		assert.Equal(t, "1"+decimalPoint+"5", float)
		assert.Equal(t, "restored", restored)

		if locale == "C" {
			assert.Equal(t, "C", set)
			assert.Equal(t, ".", decimalPoint)
		}
	}, opts)
}

// The locale the worker script sets before handling requests is restored
// after each request.
func TestSetlocaleBeforeHandlingRequests_worker(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		body, _ := testGet("http://example.com/setlocale-worker.php", handler, t)
		workerLocale, current, _ := strings.Cut(body, "|")

		assert.NotContains(t, []string{"", "C"}, workerLocale, "the worker script couldn't set a locale")
		assert.Equal(t, workerLocale, current)
	}, &testOptions{workerScript: "setlocale-worker.php"})
}
