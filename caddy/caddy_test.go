package caddy_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

var testPort = "9080"

// skipIfSymlinkNotValid skips the test if the given path is not a valid symlink
func skipIfSymlinkNotValid(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Skipf("symlink test skipped: cannot stat %s: %v", path, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Skipf("symlink test skipped: %s is not a symlink (git may not support symlinks on this platform)", path)
	}
}

// escapeMetricLabel escapes backslashes in label values for Prometheus text format
func escapeMetricLabel(s string) string {
	return strings.ReplaceAll(s, "\\", "\\\\")
}

func TestPHP(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	for i := range 100 {
		wg.Add(1)

		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestLargeRequest(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	tester.AssertPostResponseBody(
		"http://localhost:"+testPort+"/large-request.php",
		[]string{},
		bytes.NewBufferString(strings.Repeat("f", 1_048_576)),
		http.StatusOK,
		"Request body size: 1048576 (unknown)",
	)
}

func TestWorker(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker ../testdata/index.php 2
			}
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	for i := range 100 {
		wg.Add(1)

		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestGlobalAndModuleWorker(t *testing.T) {
	var wg sync.WaitGroup
	testPortNum, _ := strconv.Atoi(testPort)
	testPortTwo := strconv.Itoa(testPortNum + 1)
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999

			frankenphp {
				worker {
					file ../testdata/worker-with-env.php
					num 1
					env APP_ENV global
				}
			}
		}

		http://localhost:`+testPort+` {
			route {
				php {
					root ../testdata
					worker {
						file worker-with-env.php
						num 2
						env APP_ENV module
					}
				}
			}
		}

		http://localhost:`+testPortTwo+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			tester.AssertGetResponse("http://localhost:"+testPort+"/worker-with-env.php", http.StatusOK, "Worker has APP_ENV=module")
			tester.AssertGetResponse("http://localhost:"+testPortTwo+"/worker-with-env.php", http.StatusOK, "Worker has APP_ENV=global")
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestModuleWorkerInheritsEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			route {
				php {
					root ../testdata
					env APP_ENV inherit_this
					worker worker-with-env.php
				}
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-with-env.php", http.StatusOK, "Worker has APP_ENV=inherit_this")
}

func TestNamedModuleWorkers(t *testing.T) {
	var wg sync.WaitGroup
	testPortNum, _ := strconv.Atoi(testPort)
	testPortTwo := strconv.Itoa(testPortNum + 1)
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			route {
				php {
					root ../testdata
					worker {
						file worker-with-env.php
						num 2
						env APP_ENV one
						name module1
					}
				}
			}
		}

		http://localhost:`+testPortTwo+` {
			route {
				php {
					root ../testdata
					worker {
						file worker-with-env.php
						num 1
						env APP_ENV two
						name module2
					}
				}
			}
		}
		`, "caddyfile")

	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			tester.AssertGetResponse("http://localhost:"+testPort+"/worker-with-env.php", http.StatusOK, "Worker has APP_ENV=one")
			tester.AssertGetResponse("http://localhost:"+testPortTwo+"/worker-with-env.php", http.StatusOK, "Worker has APP_ENV=two")
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker {
					file ../testdata/worker-env.php
					num 1
					env FOO bar
				}
			}
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
					env FOO baz
				}
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-env.php", http.StatusOK, "bazbar")
}

func TestJsonEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
		"admin": {
			"listen": "localhost:2999"
		},
		"apps": {
			"frankenphp": {
			"workers": [
				{
				"env": {
					"FOO": "bar"
				},
				"file_name": "../testdata/worker-env.php",
				"num": 1
				}
			]
			},
			"http": {
			"http_port": `+testPort+`,
			"https_port": 9443,
			"servers": {
				"srv0": {
				"listen": [
					":`+testPort+`"
				],
				"routes": [
					{
					"handle": [
						{
						"handler": "subroute",
						"routes": [
							{
							"handle": [
								{
								"handler": "subroute",
								"routes": [
									{
									"handle": [
										{
										"env": {
											"FOO": "baz"
										},
										"handler": "php",
										"root": "../testdata"
										}
									]
									}
								]
								}
							]
							}
						]
						}
					],
					"match": [
						{
						"host": [
							"localhost"
						]
						}
					],
					"terminal": true
					}
				]
				}
			}
			},
			"pki": {
			"certificate_authorities": {
				"local": {
				"install_trust": false
				}
			}
			}
		}
		}
		`, "json")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-env.php", http.StatusOK, "bazbar")
}

func TestCustomCaddyVariablesInEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker {
					file ../testdata/worker-env.php
					num 1
					env FOO world
				}
			}
		}

		localhost:`+testPort+` {
			route {
				map 1 {my_customvar} {
					default "hello "
				}
				php {
					root ../testdata
					env FOO {my_customvar}
				}
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-env.php", http.StatusOK, "hello world")
}

func TestPHPServerDirective(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443
		}

		localhost:`+testPort+` {
			root ../testdata
			php_server
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:"+testPort+"/hello.txt", http.StatusOK, "Hello\n")
	tester.AssertGetResponse("http://localhost:"+testPort+"/not-found.txt", http.StatusOK, "I am by birth a Genevese (i not set)")
}

func TestPHPServerDirectiveDisableFileServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443
			order php_server before respond
		}

		localhost:`+testPort+` {
			root ../testdata
			php_server {
				file_server off
			}
			respond "Not found" 404
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:"+testPort+"/not-found.txt", http.StatusOK, "I am by birth a Genevese (i not set)")
}

func TestMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443
		metrics
	}

	localhost:`+testPort+` {
		route {
			mercure {
				transport local
				anonymous
				publisher_jwt !ChangeMe!
			}

			php {
				root ../testdata
			}
		}
	}

	example.com:`+testPort+` {
		route {
			mercure {
				transport local
				anonymous
				publisher_jwt !ChangeMe!
			}

			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	cpus := strconv.Itoa(getNumThreads(t, tester))

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads 0
	`

	ctx := caddy.ActiveContext()

	require.NoError(t, testutil.GatherAndCompare(ctx.GetMetricsRegistry(), strings.NewReader(expectedMetrics), "frankenphp_total_threads", "frankenphp_busy_threads"))
}

func TestWorkerMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443
		metrics

		frankenphp {
			worker ../testdata/index.php 2
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}

	example.com:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	workerName, _ := fastabs.FastAbs("../testdata/index.php")
	workerName = escapeMetricLabel(workerName)

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	cpus := strconv.Itoa(getNumThreads(t, tester))

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads 2

	# HELP frankenphp_busy_workers Number of busy PHP workers for this worker
	# TYPE frankenphp_busy_workers gauge
	frankenphp_busy_workers{worker="` + workerName + `"} 0

	# HELP frankenphp_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_total_workers gauge
	frankenphp_total_workers{worker="` + workerName + `"} 2

	# HELP frankenphp_worker_request_count
	# TYPE frankenphp_worker_request_count counter
	frankenphp_worker_request_count{worker="` + workerName + `"} 10

	# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_ready_workers gauge
	frankenphp_ready_workers{worker="` + workerName + `"} 2
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_threads",
			"frankenphp_busy_threads",
			"frankenphp_busy_workers",
			"frankenphp_total_workers",
			"frankenphp_worker_request_count",
			"frankenphp_ready_workers",
		))
}

func TestNamedWorkerMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443
		metrics

		frankenphp {
			worker {
				name my_app
				file ../testdata/index.php
				num 2
			}
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	cpus := strconv.Itoa(getNumThreads(t, tester))

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads 2

	# HELP frankenphp_busy_workers Number of busy PHP workers for this worker
        # TYPE frankenphp_busy_workers gauge
        frankenphp_busy_workers{worker="my_app"} 0

	# HELP frankenphp_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_total_workers gauge
	frankenphp_total_workers{worker="my_app"} 2

	# HELP frankenphp_worker_request_count
	# TYPE frankenphp_worker_request_count counter
	frankenphp_worker_request_count{worker="my_app"} 10

	# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_ready_workers gauge
	frankenphp_ready_workers{worker="my_app"} 2
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_threads",
			"frankenphp_busy_threads",
			"frankenphp_busy_workers",
			"frankenphp_total_workers",
			"frankenphp_worker_request_count",
			"frankenphp_ready_workers",
		),
	)
}

func TestAutoWorkerConfig(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443
		metrics

		frankenphp {
			worker ../testdata/index.php
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	workerName, _ := fastabs.FastAbs("../testdata/index.php")
	workerName = escapeMetricLabel(workerName)

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	numThreads := getNumThreads(t, tester)
	cpus := strconv.Itoa(numThreads)
	workers := strconv.Itoa(numThreads - 1)

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads ` + workers + `

	# HELP frankenphp_busy_workers Number of busy PHP workers for this worker
	# TYPE frankenphp_busy_workers gauge
	frankenphp_busy_workers{worker="` + workerName + `"} 0

	# HELP frankenphp_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_total_workers gauge
	frankenphp_total_workers{worker="` + workerName + `"} ` + workers + `

	# HELP frankenphp_worker_request_count
	# TYPE frankenphp_worker_request_count counter
	frankenphp_worker_request_count{worker="` + workerName + `"} 10

	# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_ready_workers gauge
	frankenphp_ready_workers{worker="` + workerName + `"} ` + workers + `
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_threads",
			"frankenphp_busy_threads",
			"frankenphp_busy_workers",
			"frankenphp_total_workers",
			"frankenphp_worker_request_count",
			"frankenphp_ready_workers",
		))
}

func TestAllDefinedServerVars(t *testing.T) {
	documentRoot, _ := filepath.Abs("../testdata/")
	expectedBodyFile, _ := os.ReadFile("../testdata/server-all-vars-ordered.txt")
	expectedBody := string(expectedBodyFile)
	expectedBody = strings.ReplaceAll(expectedBody, "{documentRoot}", documentRoot)
	expectedBody = strings.ReplaceAll(expectedBody, "\r\n", "\n")
	expectedBody = strings.ReplaceAll(expectedBody, "{testPort}", testPort)
	expectedBody = strings.ReplaceAll(expectedBody, documentRoot+"/", documentRoot+string(filepath.Separator))
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
			    # rewrite to test that the original path is passed as $REQUEST_URI
			    rewrite /server-all-vars-ordered.php/path
				php
			}
		}
		`, "caddyfile")
	tester.AssertPostResponseBody(
		"http://user@localhost:"+testPort+"/original-path?specialChars=%3E\\x00%00</>",
		[]string{
			"Content-Type: application/x-www-form-urlencoded",
			"Content-Length: 14", // maliciously set to 14
			"Special-Chars: <%00>",
			"Host: Malicious Host",
			"X-Empty-Header:",
		},
		bytes.NewBufferString("foo=bar"),
		http.StatusOK,
		expectedBody,
	)
}

func TestPHPIniConfiguration(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 2
				worker ../testdata/ini.php 1
				php_ini upload_max_filesize 100M
				php_ini memory_limit 10000000
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	testSingleIniConfiguration(tester, "upload_max_filesize", "100M")
	testSingleIniConfiguration(tester, "memory_limit", "10000000")
}

func TestPHPIniBlockConfiguration(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 1
				php_ini {
					upload_max_filesize 100M
					memory_limit 20000000
				}
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	testSingleIniConfiguration(tester, "upload_max_filesize", "100M")
	testSingleIniConfiguration(tester, "memory_limit", "20000000")
}

func testSingleIniConfiguration(tester *caddytest.Tester, key string, value string) {
	// test twice to ensure the ini setting is not lost
	for range 2 {
		tester.AssertGetResponse(
			"http://localhost:"+testPort+"/ini.php?key="+key,
			http.StatusOK,
			key+":"+value,
		)
	}
}

func TestOsEnv(t *testing.T) {
	os.Setenv("ENV1", "value1")
	os.Setenv("ENV2", "value2")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 2
				php_ini variables_order "EGPCS"
				worker ../testdata/env/env.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse(
		"http://localhost:"+testPort+"/env/env.php?keys[]=ENV1&keys[]=ENV2",
		http.StatusOK,
		"ENV1=value1,ENV2=value2",
	)
}

func TestMaxWaitTime(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 1
				max_wait_time 1ns
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// send 10 requests simultaneously, at least one request should be stalled longer than 1ns
	// since we only have 1 thread, this will cause a 504 Gateway Timeout
	wg := sync.WaitGroup{}
	success := atomic.Bool{}
	wg.Add(10)
	for range 10 {
		go func() {
			statusCode := getStatusCode("http://localhost:"+testPort+"/sleep.php?sleep=10", t)
			if statusCode == http.StatusServiceUnavailable {
				success.Store(true)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	require.True(t, success.Load(), "At least one request should have failed with a 503 Service Unavailable status")
}

func TestMaxWaitTimeWorker(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			metrics

			frankenphp {
				num_threads 2
				max_wait_time 1ns
				worker {
					num 1
					name service
					file ../testdata/sleep.php
				}
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// send 10 requests simultaneously, at least one request should be stalled longer than 1ns
	// since we only have 1 thread, this will cause a 504 Gateway Timeout
	wg := sync.WaitGroup{}
	success := atomic.Bool{}
	wg.Add(10)
	for range 10 {
		go func() {
			statusCode := getStatusCode("http://localhost:"+testPort+"/sleep.php?sleep=10&iteration=1", t)
			if statusCode == http.StatusServiceUnavailable {
				success.Store(true)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	require.True(t, success.Load(), "At least one request should have failed with a 503 Service Unavailable status")

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err)

	expectedMetrics := `
	# TYPE frankenphp_worker_queue_depth gauge
	frankenphp_worker_queue_depth{worker="service"} 0
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_worker_queue_depth",
		))
}

func getStatusCode(url string, t *testing.T) int {
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestMultiWorkersMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443
		metrics

		frankenphp {
			worker {
				name service1
				file ../testdata/index.php
				num 2
			}
			worker {
				name service2
				file ../testdata/ini.php
				num 3
			}
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}

	example.com:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	cpus := strconv.Itoa(getNumThreads(t, tester))

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads 5

	# HELP frankenphp_busy_workers Number of busy PHP workers for this worker
	# TYPE frankenphp_busy_workers gauge
	frankenphp_busy_workers{worker="service1"} 0

	# HELP frankenphp_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_total_workers gauge
	frankenphp_total_workers{worker="service1"} 2
	frankenphp_total_workers{worker="service2"} 3

	# HELP frankenphp_worker_request_count
	# TYPE frankenphp_worker_request_count counter
	frankenphp_worker_request_count{worker="service1"} 10

	# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_ready_workers gauge
	frankenphp_ready_workers{worker="service1"} 2
	frankenphp_ready_workers{worker="service2"} 3
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_threads",
			"frankenphp_busy_threads",
			"frankenphp_busy_workers",
			"frankenphp_total_workers",
			"frankenphp_worker_request_count",
			"frankenphp_ready_workers",
		))
}

func TestDisabledMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443

		frankenphp {
			worker {
				name service1
				file ../testdata/index.php
				num 2
			}
			worker {
				name service2
				file ../testdata/ini.php
				num 3
			}
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}

	example.com:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	ctx := caddy.ActiveContext()
	count, err := testutil.GatherAndCount(
		ctx.GetMetricsRegistry(),
		"frankenphp_busy_threads",
		"frankenphp_busy_workers",
		"frankenphp_queue_depth",
		"frankenphp_ready_workers",
		"frankenphp_total_threads",
		"frankenphp_total_workers",
		"frankenphp_worker_request_count",
		"frankenphp_worker_request_time",
	)

	require.NoError(t, err, "failed to count metrics")
	require.Zero(t, count, "metrics should be missing")
}

func TestWorkerRestart(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443

		metrics
		frankenphp {
			worker {
				name service
				file ../testdata/worker-restart.php
				num 1
				# restart every 3 requests
				env EVERY 3
			}
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	ctx := caddy.ActiveContext()

	resp, err := http.Get("http://localhost:2999/metrics")
	require.NoError(t, err, "failed to fetch metrics")
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	require.NoError(t, err, "failed to read metrics")

	// frankenphp_worker_restarts should be missing
	count, err := testutil.GatherAndCount(
		ctx.GetMetricsRegistry(),
		"frankenphp_worker_restarts",
	)
	require.NoError(t, err, "failed to count metrics")
	require.Zero(t, count, "metrics should be missing")

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_ready_workers gauge
	frankenphp_ready_workers{worker="service"} 1
	# HELP frankenphp_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_total_workers gauge
	frankenphp_total_workers{worker="service"} 1
	`

	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_workers",
			"frankenphp_ready_workers",
		))

	// Make some requests
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/worker-restart.php?i=%d", i), http.StatusOK, fmt.Sprintf("Counter (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// frankenphp_ready_workers should be back to 1 even after worker restarts
	expectedMetrics = `
	# HELP frankenphp_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_ready_workers gauge
	frankenphp_ready_workers{worker="service"} 1
	# HELP frankenphp_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_total_workers gauge
	frankenphp_total_workers{worker="service"} 1
	# HELP frankenphp_worker_restarts Number of PHP worker restarts for this worker
	# TYPE frankenphp_worker_restarts counter
	frankenphp_worker_restarts{worker="service"} 3
	`

	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_workers",
			"frankenphp_ready_workers",
			"frankenphp_worker_restarts",
		))
}

func TestWorkerMatchDirective(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			php_server {
				root ../testdata/files
				worker {
					file ../worker-with-counter.php
					match /matched-path*
					num 1
				}
			}
		}
		`, "caddyfile")

	// worker is outside public directory, match anyway
	tester.AssertGetResponse("http://localhost:"+testPort+"/matched-path", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort+"/matched-path/anywhere", http.StatusOK, "requests:2")

	// 404 on unmatched paths
	tester.AssertGetResponse("http://localhost:"+testPort+"/elsewhere", http.StatusNotFound, "")

	// static file will be served by the fileserver
	expectedFileResponse, err := os.ReadFile("../testdata/files/static.txt")
	require.NoError(t, err, "static.txt file must be readable for this test")
	tester.AssertGetResponse("http://localhost:"+testPort+"/static.txt", http.StatusOK, string(expectedFileResponse))
}

func TestWorkerMatchDirectiveWithMultipleWorkers(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}
		http://localhost:`+testPort+` {
			php_server {
				root ../testdata
				worker {
					file worker-with-counter.php
					match /counter/*
					num 1
				}
				worker {
					file index.php
					match /index/*
					num 1
				}
			}
		}
		`, "caddyfile")

	// match 2 workers respectively (in the public directory)
	tester.AssertGetResponse("http://localhost:"+testPort+"/counter/sub-path", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort+"/index/sub-path", http.StatusOK, "I am by birth a Genevese (i not set)")

	// static file will be served by the fileserver
	expectedFileResponse, err := os.ReadFile("../testdata/files/static.txt")
	require.NoError(t, err, "static.txt file must be readable for this test")
	tester.AssertGetResponse("http://localhost:"+testPort+"/files/static.txt", http.StatusOK, string(expectedFileResponse))

	// serve php file directly as fallback
	tester.AssertGetResponse("http://localhost:"+testPort+"/hello.php", http.StatusOK, "Hello from PHP")

	// serve index.php file directly as fallback
	tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:"+testPort+"/not-matched", http.StatusOK, "I am by birth a Genevese (i not set)")
}

func TestWorkerMatchDirectiveWithoutFileServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			route {
				php_server {
					index off
					file_server off
					root ../testdata/files
					worker {
						file ../worker-with-counter.php
						match /some-path
					}
				}

				respond "Request falls through" 404
			}
		}
		`, "caddyfile")

	// find the worker at some-path
	tester.AssertGetResponse("http://localhost:"+testPort+"/some-path", http.StatusOK, "requests:1")

	// do not find the file at static.txt
	// the request should completely fall through the php_server module
	tester.AssertGetResponse("http://localhost:"+testPort+"/static.txt", http.StatusNotFound, "Request falls through")
}

func TestDd(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			php {
				worker ../testdata/dd.php 1 {
					match *
				}
			}
		`, "caddyfile")

	// simulate Symfony's dd()
	tester.AssertGetResponse(
		"http://localhost:"+testPort+"/some-path?output=dump123",
		http.StatusInternalServerError,
		"dump123",
	)
}

func TestLog(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			log {
				output stdout
				format json
			}

			root ../testdata
			php_server {
				worker ../testdata/log-frankenphp_log.php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse(
		"http://localhost:"+testPort+"/log-frankenphp_log.php?i=0",
		http.StatusOK,
		"",
	)
}

// TestSymlinkWorkerPaths tests different ways to reference worker scripts in symlinked directories
func TestSymlinkWorkerPaths(t *testing.T) {
	cwd, _ := os.Getwd()
	publicDir := filepath.Join(cwd, "..", "testdata", "symlinks", "public")
	skipIfSymlinkNotValid(t, publicDir)

	t.Run("NeighboringWorkerScript", func(t *testing.T) {
		// Scenario: neighboring worker script
		// Given frankenphp located in the test folder
		// When I execute `frankenphp php-server --listen localhost:8080 -w index.php` from `public`
		// Then I expect to see the worker script executed successfully
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`

				frankenphp {
					worker `+publicDir+`/index.php 1
				}
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink true
					}
				}
			}
			`, "caddyfile")

		tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, "Request: 0\n")
	})

	t.Run("NestedWorkerScript", func(t *testing.T) {
		// Scenario: nested worker script
		// Given frankenphp located in the test folder
		// When I execute `frankenphp --listen localhost:8080 -w nested/index.php` from `public`
		// Then I expect to see the worker script executed successfully
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`

				frankenphp {
					worker `+publicDir+`/nested/index.php 1
				}
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink true
					}
				}
			}
			`, "caddyfile")

		tester.AssertGetResponse("http://localhost:"+testPort+"/nested/index.php", http.StatusOK, "Nested request: 0\n")
	})

	t.Run("OutsideSymlinkedFolder", func(t *testing.T) {
		// Scenario: outside the symlinked folder
		// Given frankenphp located in the root folder
		// When I execute `frankenphp --listen localhost:8080 -w public/index.php` from the root folder
		// Then I expect to see the worker script executed successfully
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`

				frankenphp {
					worker {
						name outside_worker
						file `+publicDir+`/index.php
						num 1
					}
				}
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink true
					}
				}
			}
			`, "caddyfile")

		tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, "Request: 0\n")
	})

	t.Run("SpecifiedRootDirectory", func(t *testing.T) {
		// Scenario: specified root directory
		// Given frankenphp located in the root folder
		// When I execute `frankenphp --listen localhost:8080 -w public/index.php -r public` from the root folder
		// Then I expect to see the worker script executed successfully
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`

				frankenphp {
					worker {
						name specified_root_worker
						file `+publicDir+`/index.php
						num 1
					}
				}
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink true
					}
				}
			}
			`, "caddyfile")

		tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, "Request: 0\n")
	})
}

// TestSymlinkResolveRoot tests the resolve_root_symlink directive behavior
func TestSymlinkResolveRoot(t *testing.T) {
	cwd, _ := os.Getwd()
	testDir := filepath.Join(cwd, "..", "testdata", "symlinks", "test")
	publicDir := filepath.Join(cwd, "..", "testdata", "symlinks", "public")
	skipIfSymlinkNotValid(t, publicDir)

	t.Run("ResolveRootSymlink", func(t *testing.T) {
		// Tests that resolve_root_symlink directive works correctly
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`

				frankenphp {
					worker `+publicDir+`/document-root.php 1
				}
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink true
					}
				}
			}
			`, "caddyfile")

		// DOCUMENT_ROOT should be the resolved path (testDir)
		tester.AssertGetResponse("http://localhost:"+testPort+"/document-root.php", http.StatusOK, "DOCUMENT_ROOT="+testDir+"\n")
	})

	t.Run("NoResolveRootSymlink", func(t *testing.T) {
		// Tests that symlinks are preserved when resolve_root_symlink is false (non-worker mode)
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink false
					}
				}
			}
			`, "caddyfile")

		// DOCUMENT_ROOT should be the symlink path (publicDir) when resolve_root_symlink is false
		tester.AssertGetResponse("http://localhost:"+testPort+"/document-root.php", http.StatusOK, "DOCUMENT_ROOT="+publicDir+"\n")
	})
}

// TestSymlinkWorkerBehavior tests worker behavior with symlinked directories
func TestSymlinkWorkerBehavior(t *testing.T) {
	cwd, _ := os.Getwd()
	publicDir := filepath.Join(cwd, "..", "testdata", "symlinks", "public")
	skipIfSymlinkNotValid(t, publicDir)

	t.Run("WorkerScriptFailsWithoutWorkerMode", func(t *testing.T) {
		// Tests that accessing a worker-only script without configuring it as a worker actually results in an error
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
					}
				}
			}
			`, "caddyfile")

		// Accessing the worker script without worker configuration MUST fail
		// The script checks $_SERVER['FRANKENPHP_WORKER'] and dies if not set
		tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, "Error: This script must be run in worker mode (FRANKENPHP_WORKER not set to '1')\n")
	})

	t.Run("MultipleRequests", func(t *testing.T) {
		// Tests that symlinked workers handle multiple requests correctly
		tester := caddytest.NewTester(t)
		tester.InitServer(`
			{
				skip_install_trust
				admin localhost:2999
				http_port `+testPort+`
			}

			localhost:`+testPort+` {
				route {
					php {
						root `+publicDir+`
						resolve_root_symlink true
						worker index.php 1
					}
				}
			}
			`, "caddyfile")

		// Make multiple requests - each should increment the counter
		for i := 0; i < 5; i++ {
			tester.AssertGetResponse("http://localhost:"+testPort+"/index.php", http.StatusOK, fmt.Sprintf("Request: %d\n", i))
		}
	})
}
