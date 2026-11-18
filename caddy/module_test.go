package caddy_test

import (
	"net/http"
	"os"
	"strconv"
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
