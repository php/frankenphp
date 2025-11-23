//go:build !nomercure

package frankenphp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/dunglas/mercure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMercurePublish_module(t *testing.T) { testMercurePublish(t, &testOptions{}) }
func TestMercurePublish_worker(t *testing.T) {
	testMercurePublish(t, &testOptions{workerScript: "index.php"})
}
func testMercurePublish(t *testing.T, opts *testOptions) {
	h, err := mercure.NewHub(t.Context(), mercure.WithTransport(mercure.NewLocalTransport(mercure.NewSubscriberList(0))))
	require.NoError(t, err)

	opts.requestOpts = []frankenphp.RequestOption{frankenphp.WithMercureHub(h)}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("https://example.com/mercure-publish.php?i=%d", i), handler, t)
		assert.Contains(t, body, "update 1: ")
		assert.Contains(t, body, "update 2: ")
	}, opts)
}
