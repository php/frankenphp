package caddy_test

import (
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
)

func TestRootBehavesTheSameOutsideAndInsidePhpServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	testPortNum, _ := strconv.Atoi(testPort)
	testPortTwo := strconv.Itoa(testPortNum + 1)
	expectedFileResponse, _ := os.ReadFile("../testdata/files/static.txt")
	hostWithRootOutside := "http://localhost:" + testPort
	hostWithRootInside := "http://localhost:" + testPortTwo
	tester.InitServer(`
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
