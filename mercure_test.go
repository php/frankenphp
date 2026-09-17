//go:build !nomercure

package frankenphp_test

import (
	"context"
	"errors"
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
		// Updates rejected by the protocol are reported as argument errors.
		assert.Contains(t, body, `error 1: mercure_publish(): Argument #1 ($topics) "/.well-known/mercure/subscriptions": topic value resolves into the reserved "/.well-known/mercure" namespace`)
		assert.Contains(t, body, "error 2: mercure_publish(): Argument #6 ($retry) must be greater than or equal to 0")
		assert.Contains(t, body, "error 3: mercure_publish(): Argument #1 ($topics) must only contain strings, int given")
		assert.Contains(t, body, `error 4: mercure_publish(): Argument #4 ($id) "id" field contains`)
	}, opts)
}

func TestMercurePublishWithoutHub(t *testing.T) {
	testMercurePublishError(t, nil, "error: No Mercure hub configured")
}

func TestMercurePublishFailed(t *testing.T) {
	h, err := mercure.NewHub(t.Context(), mercure.WithTransport(&failingTransport{}))
	require.NoError(t, err)

	testMercurePublishError(t, h, "error: Publish failed: dispatch failed")
}

func testMercurePublishError(t *testing.T, h *mercure.Hub, expected string) {
	t.Helper()

	opts := &testOptions{}
	if h != nil {
		opts.requestOpts = []frankenphp.RequestOption{frankenphp.WithMercureHub(h)}
	}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		body, _ := testGet("https://example.com/mercure-publish-error.php", handler, t)
		assert.Contains(t, body, expected)
	}, opts)
}

// failingTransport rejects every update, to exercise the dispatch error path.
type failingTransport struct{}

func (*failingTransport) Dispatch(context.Context, *mercure.Update) error {
	return errors.New("dispatch failed")
}

func (*failingTransport) AddSubscriber(context.Context, *mercure.LocalSubscriber) error { return nil }

func (*failingTransport) RemoveSubscriber(context.Context, *mercure.LocalSubscriber) error {
	return nil
}

func (*failingTransport) Close(context.Context) error { return nil }
