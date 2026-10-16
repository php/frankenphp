package caddy_test

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// https://github.com/php/frankenphp/issues/2469 — queued requests must survive
// a Caddy reload that restarts FrankenPHP with the same configuration.
func TestQueuedRequestSurvivesReload_module(t *testing.T) {
	testQueuedRequestSurvivesReload(t, `
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			metrics

			frankenphp {
				num_threads 1
				max_threads 1
				max_wait_time 5s
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite sleep.php
				php
			}
		}
	`)
}

func TestQueuedRequestSurvivesReload_worker(t *testing.T) {
	testQueuedRequestSurvivesReload(t, `
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			metrics

			frankenphp {
				num_threads 1
				max_threads 2
				max_wait_time 5s
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
	`)
}

func testQueuedRequestSurvivesReload(t *testing.T, config string) {
	tester := caddytest.NewTester(t)
	initServer(t, tester, config, "caddyfile")

	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := "http://localhost:" + testPort + "/sleep.php"
	workerMode := strings.Contains(config, "worker ../testdata/sleep.php")

	var wg sync.WaitGroup
	done := make(chan struct{})

	doRequest := func(query string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodGet, baseURL+"?"+query, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err, "request must not error")
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusOK, resp.StatusCode, "request must not be rejected")
		}()
	}

	// Occupy the only PHP thread so the next request has to wait in the queue.
	doRequest("sleep=10")
	doRequest("sleep=10")

	maxTries := 2000
	queued := false
	for i := 0; i <= maxTries; i++ {
		depth, err := requestQueueDepth(workerMode)
		require.NoError(t, err)
		if depth > 0 {
			queued = true
			break
		}

		doRequest("sleep=" + strconv.Itoa(i))
		time.Sleep(time.Millisecond)
		if !assert.NotEqual(t, i, maxTries, "request was never queued") {
			return
		}
	}
	require.True(t, queued)

	// Reload while the request is queued, exactly like the Caddy module does.
	reloadCaddyConfig(t, config)

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the queued request never completed after the reload")
	}
}

func TestQueuedRequestAreRejectedWhenFrankenPHPConfigChanges(t *testing.T) {
	memoryLimit := 5000000 // ~5MB
	initial := `
		{
			skip_install_trust
			admin localhost:2999
			http_port ` + testPort + `
			metrics

			frankenphp {
				num_threads 1
				max_threads 1
				php_ini memory_limit ` + strconv.Itoa(memoryLimit) + `
			}
		}

		localhost:` + testPort + ` {
			root ../testdata
			rewrite sleep.php
			php
		}
	`

	tester := caddytest.NewTester(t)
	initServer(t, tester, initial, "caddyfile")

	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := "http://localhost:" + testPort + "/sleep.php"
	requestWasRejected := atomic.Bool{}

	doRequest := func(query string) {
		go func() {
			req, err := http.NewRequest(http.MethodGet, baseURL+"?"+query, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err, "request must not error")
			require.NoError(t, resp.Body.Close())
			if resp.StatusCode != http.StatusOK {
				requestWasRejected.Store(true)
			}
		}()
	}

	sleepMilliseconds := 0
	require.Eventually(t, func() bool {
		sleepMilliseconds++
		changedCaddyConfig := strings.Replace(initial, strconv.Itoa(memoryLimit), strconv.Itoa(memoryLimit+sleepMilliseconds), 1)
		doRequest("sleep=" + strconv.Itoa(sleepMilliseconds))
		doRequest("sleep=" + strconv.Itoa(sleepMilliseconds))
		reloadCaddyConfig(t, changedCaddyConfig)
		return requestWasRejected.Load()
	}, 10*time.Second, time.Millisecond, "no queued request was ever rejected")
}

func TestQueuedRequestsAreRejectedOnChangedModulePHPConfig(t *testing.T) {
	requestBodyTimeout := 1000
	initial := `
		{
			skip_install_trust
			admin localhost:2999
			http_port ` + testPort + `
			metrics

			frankenphp {
				num_threads 1
				max_threads 1
			}
		}

		localhost:9163 {
			root ../testdata
			rewrite sleep.php
			php
		}

		localhost:` + testPort + ` {
			root ../testdata
			rewrite sleep.php
			php {
				request_body_timeout ` + strconv.Itoa(requestBodyTimeout) + `ms
			}
		}
	`

	tester := caddytest.NewTester(t)
	initServer(t, tester, initial, "caddyfile")

	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := "http://localhost:" + testPort + "/sleep.php"
	requestWasRejected := atomic.Bool{}

	doRequest := func(query string) {
		go func() {
			req, err := http.NewRequest(http.MethodGet, baseURL+"?"+query, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err, "request must not error")
			require.NoError(t, resp.Body.Close())
			if resp.StatusCode != http.StatusOK {
				requestWasRejected.Store(true)
			}
		}()
	}

	sleepMilliseconds := 0
	require.Eventually(t, func() bool {
		sleepMilliseconds++
		changedCaddyConfig := strings.Replace(initial, strconv.Itoa(requestBodyTimeout), strconv.Itoa(requestBodyTimeout+sleepMilliseconds), 1)
		doRequest("sleep=" + strconv.Itoa(sleepMilliseconds))
		doRequest("sleep=" + strconv.Itoa(sleepMilliseconds))
		reloadCaddyConfig(t, changedCaddyConfig)
		return requestWasRejected.Load()
	}, 10*time.Second, time.Millisecond, "no queued request was ever rejected")
}

func reloadCaddyConfig(t *testing.T, config string) {
	t.Helper()

	r, err := http.NewRequest(http.MethodPost, "http://localhost:2999/load?force=true", bytes.NewBufferString(config))
	require.NoError(t, err)
	r.Header.Set("Content-Type", "text/caddyfile")
	r.Header.Set("Cache-Control", "must-revalidate")

	resp, err := http.DefaultClient.Do(r)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode, "reload failed: %s", body)
}

func requestQueueDepth(workerMode bool) (float64, error) {
	resp, err := http.Get("http://localhost:2999/metrics")
	if err != nil {
		return 0, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return 0, err
	}
	if err := resp.Body.Close(); err != nil {
		return 0, err
	}

	text := string(body)
	if workerMode {
		return sumPrometheusGauge(text, "frankenphp_worker_queue_depth"), nil
	}

	return prometheusGaugeValue(text, "frankenphp_queue_depth"), nil
}

func prometheusGaugeValue(body, name string) float64 {
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, name+" ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0
			}
			v, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return 0
			}
			return v
		}
	}

	return 0
}

func sumPrometheusGauge(body, prefix string) float64 {
	var total float64
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			continue
		}
		total += v
	}

	return total
}
