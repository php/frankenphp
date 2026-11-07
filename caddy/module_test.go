package caddy_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootBehavesTheSameOutsideAndInsidePhpServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	testPortNum, _ := strconv.Atoi(testPort)
	testPortTwo := strconv.Itoa(testPortNum + 1)
	expectedFileResponse, _ := os.ReadFile("../testdata/files/static.txt")
	hostWithRootOutside := "http://localhost:" + testPort
	hostWithRootInside := "http://localhost:" + testPortTwo
	initTestServer(t, tester, `
		{
			skip_install_trust
			admin localhost:2999
		}

		`+hostWithRootOutside+` {
			root ../testdata
			php_server
		}

		`+hostWithRootInside+` {
			php_server {
				root ../testdata
			}
		}
		`, "caddyfile")

	// serve a static file
	tester.AssertGetResponse(hostWithRootOutside+"/files/static.txt", http.StatusOK, string(expectedFileResponse))
	tester.AssertGetResponse(hostWithRootInside+"/files/static.txt", http.StatusOK, string(expectedFileResponse))

	// serve a php file
	tester.AssertGetResponse(hostWithRootOutside+"/hello.php", http.StatusOK, "Hello from PHP")
	tester.AssertGetResponse(hostWithRootInside+"/hello.php", http.StatusOK, "Hello from PHP")

	// fallback to index.php
	tester.AssertGetResponse(hostWithRootOutside+"/some-path", http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse(hostWithRootInside+"/some-path", http.StatusOK, "I am by birth a Genevese (i not set)")

	// fallback to directory index ('dirIndex' in module.go)
	tester.AssertGetResponse(hostWithRootOutside+"/dirindex/", http.StatusOK, "Hello from directory index.php")
	tester.AssertGetResponse(hostWithRootInside+"/dirindex/", http.StatusOK, "Hello from directory index.php")
}

func TestCanonicalPathRedirectStaysOnSameOrigin(t *testing.T) {
	tester := caddytest.NewTester(t)
	testPortNum, _ := strconv.Atoi(testPort)
	phpServerHost := "http://localhost:" + testPort
	manualHost := "http://localhost:" + strconv.Itoa(testPortNum+1)
	initTestServer(t, tester, `
		{
			skip_install_trust
			admin localhost:2999
		}

		`+phpServerHost+` {
			root ../testdata
			php_server
		}

		`+manualHost+` {
			root ../testdata
			route {
				@canonicalPath {
					file {path}/index.php
					not path */
				}
				redir @canonicalPath ./{http.request.orig_uri.path.file_escaped}/{http.request.orig_uri.prefixed_query} 308
				php
			}
		}
		`, "caddyfile")

	tester.Client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for _, host := range []string{phpServerHost, manualHost} {
		for path, location := range map[string]string{
			"/dirindex":                              "./dirindex/",
			"/dirindex?a=b":                          "./dirindex/?a=b",
			"//example.com/..%2fdirindex":            "./..%2fdirindex/",
			"/%5cexample.com/..%2fdirindex":          "./..%2fdirindex/",
			"/%09/example.com/..%2f..%2fdirindex":    "./..%2f..%2fdirindex/",
			"/%0a%09%0d%5cexample.com/..%2fdirindex": "./..%2fdirindex/",
		} {
			resp, err := tester.Client.Get(host + path)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, http.StatusPermanentRedirect, resp.StatusCode, path)
			assert.Equal(t, location, resp.Header.Get("Location"), path)
		}
	}
}

func TestRequestOptionsIsolation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		directory := filepath.Join(root, name)
		require.NoError(t, os.Mkdir(directory, 0o755))
		script := fmt.Sprintf(`<?php
echo '%s', "\n", getenv('TOKEN'), "\n", $_SERVER['REQUEST_URI'], "\n", $_SERVER['SCRIPT_NAME'], "\n";
`, name)
		require.NoError(t, os.WriteFile(filepath.Join(directory, "index.php"), []byte(script), 0o600))
	}

	for _, dynamicRoot := range []bool{false, true} {
		for _, dynamicEnv := range []bool{false, true} {
			t.Run(fmt.Sprintf("dynamic_root=%t/dynamic_env=%t", dynamicRoot, dynamicEnv), func(t *testing.T) {
				documentRoot := filepath.ToSlash(filepath.Join(root, "a"))
				if dynamicRoot {
					documentRoot = filepath.ToSlash(root) + "/{http.request.header.X-Test-Root}"
				}
				env := "static"
				if dynamicEnv {
					env = "{http.request.header.X-Test-Token}"
				}

				tester := caddytest.NewTester(t)
				initTestServer(t, tester, fmt.Sprintf(`
					{
						skip_install_trust
						admin localhost:2999
					}
					http://localhost:%s {
						root %q
						route {
							rewrite * /index.php
							php {
								root %s
								resolve_root_symlink false
								env TOKEN %s
							}
						}
					}
				`, testPort, filepath.Join(root, "a"), strconv.Quote(documentRoot), env), "caddyfile")

				var wg sync.WaitGroup
				for i := range 24 {
					wg.Go(func() {
						directory := "a"
						if i%2 != 0 {
							directory = "b"
						}
						token := strconv.Itoa(i)
						uri := fmt.Sprintf("/before%%2Frewrite/%d?value=a%%20b&plus=%%2B", i)
						if i == 0 {
							uri = "/before?"
						}
						req, err := http.NewRequest(http.MethodGet, "http://localhost:"+testPort+uri, nil)
						require.NoError(t, err)
						req.Header.Set("X-Test-Root", directory)
						req.Header.Set("X-Test-Token", token)

						resp, err := tester.Client.Do(req)
						require.NoError(t, err)
						defer func() {
							require.NoError(t, resp.Body.Close())
						}()
						require.Equal(t, http.StatusOK, resp.StatusCode)
						body, err := io.ReadAll(resp.Body)
						require.NoError(t, err)

						if !dynamicRoot {
							directory = "a"
						}
						if !dynamicEnv {
							token = "static"
						}
						require.Equal(t, directory+"\n"+token+"\n"+uri+"\n/index.php\n", string(body))
					})
				}
				wg.Wait()
			})
		}
	}
}
