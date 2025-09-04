package caddy_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func TestRestartWorkerViaAdminApi(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				worker ../testdata/worker-with-counter.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-counter.php
				php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")

	assertAdminResponse(t, tester, "POST", "workers/restart", http.StatusOK, "workers restarted successfully\n")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
}

func TestShowTheCorrectThreadDebugStatus(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 3
				max_threads 6
				worker ../testdata/worker-with-counter.php 1
				worker ../testdata/index.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-counter.php
				php
			}
		}
		`, "caddyfile")

	debugState := getDebugState(t, tester)

	// assert that the correct threads are present in the thread info
	assert.Equal(t, debugState.ThreadDebugStates[0].State, "ready")
	assert.Contains(t, debugState.ThreadDebugStates[1].Name, "worker-with-counter.php")
	assert.Contains(t, debugState.ThreadDebugStates[2].Name, "index.php")
	assert.Equal(t, debugState.ReservedThreadCount, 3)
	assert.Len(t, debugState.ThreadDebugStates, 3)
}

func TestAutoScaleWorkerThreads(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 10
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads 10
				num_threads 2
				worker ../testdata/sleep.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite sleep.php
				php
			}
		}
		`, "caddyfile")

	// spam an endpoint that simulates IO
	endpoint := "http://localhost:" + testPort + "/?sleep=2&work=1000"
	amountOfThreads := getNumThreads(t, tester)

	// try to spawn the additional threads by spamming the server
	for range maxTries {
		wg.Add(requestsPerTry)
		for range requestsPerTry {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 2 ms and worked for 1000 iterations")
				wg.Done()
			}()
		}
		wg.Wait()

		amountOfThreads = getNumThreads(t, tester)
		if amountOfThreads > 2 {
			break
		}
	}

	// assert that there are now more threads than before
	assert.NotEqual(t, amountOfThreads, 2)
}

// Note this test requires at least 2x40MB available memory for the process
func TestAutoScaleRegularThreadsOnAutomaticThreadLimit(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 10
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads auto
				num_threads 1
				php_ini memory_limit 40M # a reasonable limit for the test
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// spam an endpoint that simulates IO
	endpoint := "http://localhost:" + testPort + "/sleep.php?sleep=2&work=1000"
	amountOfThreads := getNumThreads(t, tester)

	// try to spawn the additional threads by spamming the server
	for range maxTries {
		wg.Add(requestsPerTry)
		for range requestsPerTry {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 2 ms and worked for 1000 iterations")
				wg.Done()
			}()
		}
		wg.Wait()

		amountOfThreads = getNumThreads(t, tester)
		if amountOfThreads > 1 {
			break
		}
	}

	// assert that there are now more threads present
	assert.NotEqual(t, amountOfThreads, 1)
}

func assertAdminResponse(t *testing.T, tester *caddytest.Tester, method string, path string, expectedStatus int, expectedBody string) {
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	assert.NoError(t, err)
	if expectedBody == "" {
		_ = tester.AssertResponseCode(r, expectedStatus)
		return
	}
	_, _ = tester.AssertResponse(r, expectedStatus, expectedBody)
}

func getAdminResponseBody(t *testing.T, tester *caddytest.Tester, method string, path string) string {
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	assert.NoError(t, err)
	resp := tester.AssertResponseCode(r, http.StatusOK)
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	return string(bytes)
}

func getDebugState(t *testing.T, tester *caddytest.Tester) frankenphp.FrankenPHPDebugState {
	t.Helper()
	threadStates := getAdminResponseBody(t, tester, "GET", "threads")

	var debugStates frankenphp.FrankenPHPDebugState
	err := json.Unmarshal([]byte(threadStates), &debugStates)
	assert.NoError(t, err)

	return debugStates
}

func getNumThreads(t *testing.T, tester *caddytest.Tester) int {
	t.Helper()
	return len(getDebugState(t, tester).ThreadDebugStates)
}

func TestAddModuleWorkerViaAdminApi(t *testing.T) {
	// Initialize a server with admin API enabled
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// Get initial debug state to check number of workers
	initialDebugState := getDebugState(t, tester)
	initialWorkerCount := 0
	for _, thread := range initialDebugState.ThreadDebugStates {
		if thread.Name != "" && thread.Name != "ready" {
			initialWorkerCount++
		}
	}

	// Create a Caddyfile configuration with a module worker
	workerConfig := `
	{
		skip_install_trust
		admin localhost:2999
		http_port ` + testPort + `
	}

	localhost:` + testPort + ` {
		route {
			root ../testdata
			php {
				worker ../testdata/worker-with-counter.php 1
			}
		}
	}
	`

	// Send the configuration to the admin API
	adminUrl := "http://localhost:2999/load"
	r, err := http.NewRequest("POST", adminUrl, bytes.NewBufferString(workerConfig))
	assert.NoError(t, err)
	r.Header.Set("Content-Type", "text/caddyfile")
	resp := tester.AssertResponseCode(r, http.StatusOK)
	defer resp.Body.Close()

	// Get the updated debug state to check if the worker was added
	updatedDebugState := getDebugState(t, tester)
	updatedWorkerCount := 0
	workerFound := false
	filename, _ := fastabs.FastAbs("../testdata/worker-with-counter.php")
	for _, thread := range updatedDebugState.ThreadDebugStates {
		if thread.Name != "" && thread.Name != "ready" {
			updatedWorkerCount++
			if thread.Name == "Worker PHP Thread - "+filename {
				workerFound = true
			}
		}
	}

	// Assert that the worker was added
	assert.Greater(t, updatedWorkerCount, initialWorkerCount, "Worker count should have increased")
	assert.True(t, workerFound, fmt.Sprintf("Worker with name %q should be found", "Worker PHP Thread - "+filename))

	// Make a request to the worker to verify it's working
	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-with-counter.php", http.StatusOK, "requests:1")
}
