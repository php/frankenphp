package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegularRequestChanIsCreatedBeforeInitRuns(t *testing.T) {
	require.NotNil(t, regularRequestChan,
		"a request dispatched between the start of Init and the first thread being attached "+
			"must queue on a real channel: sending on a nil channel blocks forever, "+
			"even after the channel variable is later assigned")
}

func TestRequestsQueuedBeforeThreadsAreReadyAreHandedOver(t *testing.T) {
	regularThreadMu.Lock()
	savedThreads := regularThreads
	regularThreads = nil
	regularThreadMu.Unlock()
	savedMaxWaitTime := maxWaitTime.Swap(0)
	savedScaleChan := scaleChan
	scaleChan = nil
	t.Cleanup(func() {
		regularThreadMu.Lock()
		regularThreads = savedThreads
		regularThreadMu.Unlock()
		maxWaitTime.Store(savedMaxWaitTime)
		scaleChan = savedScaleChan
	})

	const requests = 5
	errChans := make([]chan error, requests)
	for i := range errChans {
		fc := &frankenPHPContext{done: make(chan any)}
		errChan := make(chan error, 1)
		go func() {
			errChan <- handleRequestWithRegularPHPThreads(fc)
		}()
		errChans[i] = errChan
	}

	for i := range requests {
		select {
		case fc := <-regularRequestChan:
			fc.closeContext()
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of %d queued requests were handed over", i, requests)
		}
	}

	for i, errChan := range errChans {
		select {
		case err := <-errChan:
			assert.NoError(t, err, "request %d", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("dispatcher for request %d did not return after the request was handled", i)
		}
	}
}
