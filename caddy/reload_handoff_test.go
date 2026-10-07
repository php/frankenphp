package caddy_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	caddy "github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/dunglas/frankenphp"
	frankenphpcaddy "github.com/dunglas/frankenphp/caddy"
	"github.com/stretchr/testify/require"
)

const handoffPort = "9085"

func TestReloadHandoffReplaysThroughTheReplacementChain(t *testing.T) {
	for _, withAuth := range []bool{true, false} {
		t.Run(fmt.Sprintf("auth=%t", withAuth), func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'];"), 0o600))

			shutdownHookFired := make(chan struct{})
			var hookOnce sync.Once
			frankenphpcaddy.RegisterWorkers("handoff", "../testdata/worker-with-counter.php", 1,
				frankenphp.WithWorkerOnServerShutdown(func() {
					hookOnce.Do(func() { close(shutdownHookFired) })
				}),
			)

			tester := caddytest.NewTester(t)
			initTestServer(t, tester, handoffConfig(root, "old", false), "json")

			conn, err := net.Dial("tcp", "localhost:"+handoffPort)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			require.NoError(t, conn.SetDeadline(time.Now().Add(30*time.Second)))
			reader := bufio.NewReader(conn)
			write := func(s string) {
				t.Helper()
				_, err := conn.Write([]byte(s))
				require.NoError(t, err)
			}
			readResponse := func() (*http.Response, string) {
				t.Helper()
				resp, err := http.ReadResponse(reader, nil)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())

				return resp, string(body)
			}

			write("GET /index.php HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n\r\n")
			firstResp, firstBody := readResponse()
			require.Equal(t, http.StatusOK, firstResp.StatusCode)
			require.Equal(t, "old", firstBody)
			require.False(t, firstResp.Close, "the connection must stay open for the parked request")

			// The head parks without its blank line; never retry it: a retry would reach the new chain and pass vacuously.
			write("GET /index.php HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")

			reloaded := make(chan error, 1)
			go func() { reloaded <- caddy.Load([]byte(handoffConfig(root, "new", withAuth)), true) }()

			select {
			case <-shutdownHookFired:
			case <-time.After(30 * time.Second):
				t.Fatal("the old runtime's shutdown hook never fired during the reload")
			}

			write("\r\n")
			replayedResp, replayedBody := readResponse()
			if withAuth {
				require.Equal(t, http.StatusUnauthorized, replayedResp.StatusCode,
					"the replayed request must pass through the replacement's basic_auth middleware")
				require.Equal(t, `Basic realm="frankenphp-test"`, replayedResp.Header.Get("WWW-Authenticate"))
				require.Empty(t, replayedBody)
			} else {
				require.Equal(t, http.StatusOK, replayedResp.StatusCode)
				require.Equal(t, "new", replayedBody, "the replayed request must be served by the replacement config")
			}

			select {
			case err := <-reloaded:
				require.NoError(t, err)
			case <-time.After(30 * time.Second):
				t.Fatal("the reload never completed")
			}

			url := "http://localhost:" + handoffPort + "/index.php"
			if withAuth {
				freshResp, _ := tester.AssertGetResponse(url, http.StatusUnauthorized, "")
				require.Equal(t, `Basic realm="frankenphp-test"`, freshResp.Header.Get("WWW-Authenticate"))
				authReq, err := http.NewRequest(http.MethodGet, url, nil)
				require.NoError(t, err)
				authReq.SetBasicAuth("bob", "s3cr3t-P4ssw0rd")
				tester.AssertResponse(authReq, http.StatusOK, "new")
			} else {
				tester.AssertGetResponse(url, http.StatusOK, "new")
			}
		})
	}
}

func handoffConfig(root, marker string, withAuth bool) string {
	authentication := ""
	if withAuth {
		authentication = `{
						"handler": "authentication",
						"providers": {
							"http_basic": {
								"accounts": [{"username": "bob", "password": "$2y$12$cgfFbRD1bWPPHhPmmrt38OPqJsBibGfhNwfl9zXWIQA4MiQp67r3e"}],
								"hash": {"algorithm": "bcrypt"},
								"realm": "frankenphp-test"
							}
						}
					},`
	}

	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							{
								"handle": [
									%[2]s
									{
										"handler": "php",
										"root": %[3]q,
										"env": {"MARKER": %[4]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, handoffPort, authentication, root, marker)
}

func TestReloadHandoffReplaysTheClientsOriginalRequest(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.php"), []byte("<?php echo $_SERVER['MARKER'] . $_SERVER['REQUEST_URI'];"), 0o600))

	shutdownHookFired := make(chan struct{})
	var hookOnce sync.Once
	frankenphpcaddy.RegisterWorkers("handoff-rewrite", "../testdata/worker-with-counter.php", 1,
		frankenphp.WithWorkerOnServerShutdown(func() {
			hookOnce.Do(func() { close(shutdownHookFired) })
		}),
	)

	tester := caddytest.NewTester(t)
	initTestServer(t, tester, rewriteHandoffConfig(root, "old", false), "json")

	dial := func() (*bufio.Reader, func(string)) {
		conn, err := net.Dial("tcp", "localhost:"+handoffPort)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, conn.SetDeadline(time.Now().Add(30*time.Second)))
		reader := bufio.NewReader(conn)
		return reader, func(s string) {
			t.Helper()
			_, err := conn.Write([]byte(s))
			require.NoError(t, err)
		}
	}
	readResponse := func(reader *bufio.Reader) (*http.Response, string) {
		t.Helper()
		resp, err := http.ReadResponse(reader, nil)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		return resp, string(body)
	}

	blockedReader, writeBlocked := dial()
	writeBlocked("GET /blocked/secret HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n\r\n")
	firstResp, firstBody := readResponse(blockedReader)
	require.Equal(t, http.StatusOK, firstResp.StatusCode)
	require.Equal(t, "old/blocked/secret", firstBody)
	require.False(t, firstResp.Close, "the connection must stay open for the parked request")

	writeBlocked("GET /blocked/secret HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")
	helloReader, writeHello := dial()
	writeHello("GET /hello HTTP/1.1\r\nHost: localhost:" + handoffPort + "\r\n")

	reloaded := make(chan error, 1)
	go func() { reloaded <- caddy.Load([]byte(rewriteHandoffConfig(root, "new", true)), true) }()

	select {
	case <-shutdownHookFired:
	case <-time.After(30 * time.Second):
		t.Fatal("the old runtime's shutdown hook never fired during the reload")
	}

	// Complete both heads while the replacement is still starting: once the old HTTP server shuts down, late heads are dropped.
	writeBlocked("\r\n")
	writeHello("\r\n")

	blockedResp, blockedBody := readResponse(blockedReader)
	require.Equal(t, http.StatusForbidden, blockedResp.StatusCode,
		"the replayed request must be re-dispatched as the client sent it, so the replacement's path matcher applies")
	require.Empty(t, blockedBody)

	helloResp, helloBody := readResponse(helloReader)
	require.Equal(t, http.StatusOK, helloResp.StatusCode)
	require.Equal(t, "new/hello", helloBody)

	select {
	case err := <-reloaded:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("the reload never completed")
	}

	url := "http://localhost:" + handoffPort
	tester.AssertGetResponse(url+"/blocked/secret", http.StatusForbidden, "")
	tester.AssertGetResponse(url+"/hello", http.StatusOK, "new/hello")
}

func rewriteHandoffConfig(root, marker string, withBlockedRoute bool) string {
	blockedRoute := ""
	if withBlockedRoute {
		blockedRoute = `{
						"handle": [
							{
								"handler": "static_response",
								"status_code": 403
							}
						],
						"match": [{"host": ["localhost"], "path": ["/blocked*"]}],
						"terminal": true
					},`
	}

	return fmt.Sprintf(`{
		"admin": {"listen": "localhost:2998"},
		"apps": {
			"frankenphp": {"num_threads": 1},
			"http": {
				"http_port": %[1]s,
				"https_port": 9443,
				"servers": {
					"srv0": {
						"listen": [":%[1]s"],
						"routes": [
							%[2]s
							{
								"handle": [
									{
										"handler": "rewrite",
										"uri": "/index.php"
									},
									{
										"handler": "php",
										"root": %[3]q,
										"env": {"MARKER": %[4]q}
									}
								],
								"match": [{"host": ["localhost"]}],
								"terminal": true
							}
						]
					}
				}
			},
			"pki": {"certificate_authorities": {"local": {"install_trust": false}}}
		}
	}`, handoffPort, blockedRoute, root, marker)
}
