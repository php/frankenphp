// Tests for the zval.h helpers. The exercising PHP function
// frankenphp_test_persist_roundtrip is only registered when the
// FRANKENPHP_TEST preprocessor flag is set. Without the flag, the
// PHP fixture prints a SKIP line and the test skips so production builds
// stay clean.

package frankenphp_test

import (
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

func TestPersistentZvalRoundtrip(t *testing.T) {
	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	require.NoError(t, frankenphp.Init())
	t.Cleanup(frankenphp.Shutdown)

	req := httptest.NewRequest("GET", "http://example.com/persist-roundtrip.php", nil)
	fr, err := frankenphp.NewRequestWithContext(req, frankenphp.WithRequestDocumentRoot(testDataDir, false))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	err = frankenphp.ServeHTTP(w, fr)
	if err != nil {
		require.ErrorAs(t, err, &frankenphp.ErrRejected{})
	}

	body, _ := io.ReadAll(w.Result().Body)
	out := string(body)

	if strings.Contains(out, "SKIP") {
		t.Skip("FRANKENPHP_TEST not set; skipping persistent_zval roundtrip tests")
	}

	require.NotContains(t, out, "FAIL", "persist-roundtrip.php reported a failure:\n"+out)
	require.Contains(t, out, "OK null")
	require.Contains(t, out, "OK enum active")
	require.Contains(t, out, "OK stdClass rejected")
	require.Contains(t, out, "OK resource rejected")
	require.Contains(t, out, "OK nested stdClass rejected")
}
