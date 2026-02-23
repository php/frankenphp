// In all tests, headers added to requests are copied on the heap using strings.Clone.
// This was originally a workaround for https://github.com/golang/go/issues/65286#issuecomment-1920087884 (fixed in Go 1.22),
// but this allows to catch panics occurring in real life but not when the string is in the internal binary memory.

package frankenphp_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testOptions struct {
	workerScript       string
	watch              []string
	nbWorkers          int
	env                map[string]string
	nbParallelRequests int
	realServer         bool
	logger             *slog.Logger
	initOpts           []frankenphp.Option
	requestOpts        []frankenphp.RequestOption
	phpIni             map[string]string
}

func runTest(t *testing.T, test func(func(http.ResponseWriter, *http.Request), *httptest.Server, int), opts *testOptions) {
	if opts == nil {
		opts = &testOptions{}
	}
	if opts.nbParallelRequests == 0 {
		opts.nbParallelRequests = 100
	}

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	initOpts := []frankenphp.Option{frankenphp.WithLogger(opts.logger)}
	if opts.workerScript != "" {
		workerOpts := []frankenphp.WorkerOption{
			frankenphp.WithWorkerEnv(opts.env),
			frankenphp.WithWorkerWatchMode(opts.watch),
		}
		initOpts = append(initOpts, frankenphp.WithWorkers("workerName", testDataDir+opts.workerScript, opts.nbWorkers, workerOpts...))
	}
	initOpts = append(initOpts, opts.initOpts...)
	if opts.phpIni != nil {
		initOpts = append(initOpts, frankenphp.WithPhpIni(opts.phpIni))
	}

	err := frankenphp.Init(initOpts...)
	require.NoError(t, err)
	defer frankenphp.Shutdown()

	opts.requestOpts = append(opts.requestOpts, frankenphp.WithRequestDocumentRoot(testDataDir, false))

	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, opts.requestOpts...)
		assert.NoError(t, err)

		err = frankenphp.ServeHTTP(w, req)
		if err != nil && !errors.As(err, &frankenphp.ErrRejected{}) {
			assert.Fail(t, fmt.Sprintf("Received unexpected error:\n%+v", err))
		}
	}

	var ts *httptest.Server
	if opts.realServer {
		ts = httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()
	}

	var wg sync.WaitGroup
	wg.Add(opts.nbParallelRequests)
	for i := 0; i < opts.nbParallelRequests; i++ {
		go func(i int) {
			test(handler, ts, i)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func testRequest(req *http.Request, handler func(http.ResponseWriter, *http.Request), t *testing.T) (string, *http.Response) {
	t.Helper()

	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body), resp
}

func testGet(url string, handler func(http.ResponseWriter, *http.Request), t *testing.T) (string, *http.Response) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)

	return testRequest(req, handler, t)
}

func testPost(url string, body string, handler func(http.ResponseWriter, *http.Request), t *testing.T) (string, *http.Response) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Body = io.NopCloser(strings.NewReader(body))

	return testRequest(req, handler, t)
}

func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Verbose() {
		slog.SetDefault(slog.New(slog.DiscardHandler))
	}

	os.Exit(m.Run())
}

func TestHelloWorld_module(t *testing.T) { testHelloWorld(t, nil) }
func TestHelloWorld_worker(t *testing.T) {
	testHelloWorld(t, &testOptions{workerScript: "index.php"})
}
func testHelloWorld(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/index.php?i=%d", i), handler, t)
		assert.Equal(t, fmt.Sprintf("I am by birth a Genevese (%d)", i), body)
	}, opts)
}

func TestFinishRequest_module(t *testing.T) { testFinishRequest(t, nil) }
func TestFinishRequest_worker(t *testing.T) {
	testFinishRequest(t, &testOptions{workerScript: "finish-request.php"})
}
func testFinishRequest(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/finish-request.php?i=%d", i), handler, t)
		assert.Equal(t, fmt.Sprintf("This is output %d\n", i), body)
	}, opts)
}

func TestServerVariable_module(t *testing.T) {
	testServerVariable(t, nil)
}
func TestServerVariable_worker(t *testing.T) {
	testServerVariable(t, &testOptions{workerScript: "server-variable.php"})
}
func testServerVariable(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("POST", fmt.Sprintf("http://example.com/server-variable.php/baz/bat?foo=a&bar=b&i=%d#hash", i), strings.NewReader("foo"))
		req.SetBasicAuth(strings.Clone("kevin"), strings.Clone("password"))
		req.Header.Add(strings.Clone("Content-Type"), strings.Clone("text/plain"))
		body, _ := testRequest(req, handler, t)

		assert.Contains(t, body, "[REMOTE_HOST]")
		assert.Contains(t, body, "[REMOTE_USER] => kevin")
		assert.Contains(t, body, "[PHP_AUTH_USER] => kevin")
		assert.Contains(t, body, "[PHP_AUTH_PW] => password")
		assert.Contains(t, body, "[HTTP_AUTHORIZATION] => Basic a2V2aW46cGFzc3dvcmQ=")
		assert.Contains(t, body, "[DOCUMENT_ROOT]")
		assert.Contains(t, body, "[PHP_SELF] => /server-variable.php/baz/bat")
		assert.Contains(t, body, "[CONTENT_TYPE] => text/plain")
		assert.Contains(t, body, fmt.Sprintf("[QUERY_STRING] => foo=a&bar=b&i=%d#hash", i))
		assert.Contains(t, body, fmt.Sprintf("[REQUEST_URI] => /server-variable.php/baz/bat?foo=a&bar=b&i=%d#hash", i))
		assert.Contains(t, body, "[CONTENT_LENGTH]")
		assert.Contains(t, body, "[REMOTE_ADDR]")
		assert.Contains(t, body, "[REMOTE_PORT]")
		assert.Contains(t, body, "[REQUEST_SCHEME] => http")
		assert.Contains(t, body, "[DOCUMENT_URI]")
		assert.Contains(t, body, "[AUTH_TYPE]")
		assert.Contains(t, body, "[REMOTE_IDENT]")
		assert.Contains(t, body, "[REQUEST_METHOD] => POST")
		assert.Contains(t, body, "[SERVER_NAME] => example.com")
		assert.Contains(t, body, "[SERVER_PROTOCOL] => HTTP/1.1")
		assert.Contains(t, body, "[SCRIPT_FILENAME]")
		assert.Contains(t, body, "[SERVER_SOFTWARE] => FrankenPHP")
		assert.Contains(t, body, "[REQUEST_TIME_FLOAT]")
		assert.Contains(t, body, "[REQUEST_TIME]")
		assert.Contains(t, body, "[SERVER_PORT] => 80")
	}, opts)
}

func TestPathInfo_module(t *testing.T) { testPathInfo(t, nil) }
func TestPathInfo_worker(t *testing.T) {
	testPathInfo(t, &testOptions{workerScript: "server-variable.php"})
}
func testPathInfo(t *testing.T, opts *testOptions) {
	cwd, _ := os.Getwd()
	testDataDir := cwd + strings.Clone("/testdata/")
	path := strings.Clone("/server-variable.php/pathinfo")

	runTest(t, func(_ func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			requestURI := r.URL.RequestURI()
			r.URL.Path = path

			rewriteRequest, err := frankenphp.NewRequestWithContext(r,
				frankenphp.WithRequestDocumentRoot(testDataDir, false),
				frankenphp.WithRequestEnv(map[string]string{"REQUEST_URI": requestURI}),
			)
			assert.NoError(t, err)

			err = frankenphp.ServeHTTP(w, rewriteRequest)
			assert.NoError(t, err)
		}

		body, _ := testGet(fmt.Sprintf("http://example.com/pathinfo/%d", i), handler, t)

		assert.Contains(t, body, "[PATH_INFO] => /pathinfo")
		assert.Contains(t, body, fmt.Sprintf("[REQUEST_URI] => /pathinfo/%d", i))
		assert.Contains(t, body, "[PATH_TRANSLATED] =>")
		assert.Contains(t, body, "[SCRIPT_NAME] => /server-variable.php")

	}, opts)
}

func TestHeaders_module(t *testing.T) { testHeaders(t, nil) }
func TestHeaders_worker(t *testing.T) { testHeaders(t, &testOptions{workerScript: "headers.php"}) }
func testHeaders(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, resp := testGet(fmt.Sprintf("http://example.com/headers.php?i=%d", i), handler, t)

		assert.Equal(t, "Hello", body)
		assert.Equal(t, 201, resp.StatusCode)
		assert.Equal(t, "bar", resp.Header.Get("Foo"))
		assert.Equal(t, "bar2", resp.Header.Get("Foo2"))
		assert.Equal(t, "bar3", resp.Header.Get("Foo3"), "header without whitespace after colon")
		assert.Empty(t, resp.Header.Get("Invalid"))
		assert.Equal(t, fmt.Sprintf("%d", i), resp.Header.Get("I"))
	}, opts)
}

func TestResponseHeaders_module(t *testing.T) { testResponseHeaders(t, nil) }
func TestResponseHeaders_worker(t *testing.T) {
	testResponseHeaders(t, &testOptions{workerScript: "response-headers.php"})
}
func testResponseHeaders(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, resp := testGet(fmt.Sprintf("http://example.com/response-headers.php?i=%d", i), handler, t)

		if i%3 != 0 {
			assert.Equal(t, i+100, resp.StatusCode)
		} else {
			assert.Equal(t, 200, resp.StatusCode)
		}

		assert.Contains(t, body, "'X-Powered-By' => 'PH")
		assert.Contains(t, body, "'Foo' => 'bar',")
		assert.Contains(t, body, "'Foo2' => 'bar2',")
		assert.Contains(t, body, fmt.Sprintf("'I' => '%d',", i))
		assert.NotContains(t, body, "Invalid")
	}, opts)
}

func TestInput_module(t *testing.T) { testInput(t, nil) }
func TestInput_worker(t *testing.T) { testInput(t, &testOptions{workerScript: "input.php"}) }
func testInput(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, resp := testPost("http://example.com/input.php", fmt.Sprintf("post data %d", i), handler, t)

		assert.Equal(t, fmt.Sprintf("post data %d", i), body)
		assert.Equal(t, "bar", resp.Header.Get("Foo"))
	}, opts)
}

func TestPostSuperGlobals_module(t *testing.T) { testPostSuperGlobals(t, nil) }
func TestPostSuperGlobals_worker(t *testing.T) {
	testPostSuperGlobals(t, &testOptions{workerScript: "super-globals.php"})
}
func testPostSuperGlobals(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		formData := url.Values{"baz": {"bat"}, "i": {fmt.Sprintf("%d", i)}}
		req := httptest.NewRequest("POST", fmt.Sprintf("http://example.com/super-globals.php?foo=bar&iG=%d", i), strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", strings.Clone("application/x-www-form-urlencoded"))
		body, _ := testRequest(req, handler, t)

		assert.Contains(t, body, "'foo' => 'bar'")
		assert.Contains(t, body, fmt.Sprintf("'i' => '%d'", i))
		assert.Contains(t, body, "'baz' => 'bat'")
		assert.Contains(t, body, fmt.Sprintf("'iG' => '%d'", i))
	}, opts)
}

func TestRequestSuperGlobal_module(t *testing.T) { testRequestSuperGlobal(t, nil) }
func TestRequestSuperGlobal_worker(t *testing.T) {
	phpIni := make(map[string]string)
	phpIni["auto_globals_jit"] = "1"
	testRequestSuperGlobal(t, &testOptions{workerScript: "request-superglobal.php", phpIni: phpIni})
}
func testRequestSuperGlobal(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// Test with both GET and POST parameters
		// $_REQUEST should contain merged data from both
		formData := url.Values{"post_key": {fmt.Sprintf("post_value_%d", i)}}
		req := httptest.NewRequest("POST", fmt.Sprintf("http://example.com/request-superglobal.php?get_key=get_value_%d", i), strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		body, _ := testRequest(req, handler, t)

		// Verify $_REQUEST contains both GET and POST data for the current request
		assert.Contains(t, body, fmt.Sprintf("'get_key' => 'get_value_%d'", i))
		assert.Contains(t, body, fmt.Sprintf("'post_key' => 'post_value_%d'", i))
	}, opts)
}

func TestRequestSuperGlobalConditional_worker(t *testing.T) {
	// This test verifies that $_REQUEST works correctly when accessed conditionally
	// in worker mode. The first request does NOT access $_REQUEST, but subsequent
	// requests do. This tests the "re-arm" mechanism for JIT auto globals.
	//
	// The bug scenario:
	// - Request 1 (i=1): includes file, $_REQUEST initialized with val=1
	// - Request 3 (i=3): includes file from cache, $_REQUEST should have val=3
	// If the bug exists, $_REQUEST would still have val=1 from request 1.
	phpIni := make(map[string]string)
	phpIni["auto_globals_jit"] = "1"
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		if i%2 == 0 {
			// Even requests: don't use $_REQUEST
			body, _ := testGet(fmt.Sprintf("http://example.com/request-superglobal-conditional.php?val=%d", i), handler, t)
			assert.Contains(t, body, "SKIPPED")
			assert.Contains(t, body, fmt.Sprintf("'val' => '%d'", i))
		} else {
			// Odd requests: use $_REQUEST
			body, _ := testGet(fmt.Sprintf("http://example.com/request-superglobal-conditional.php?use_request=1&val=%d", i), handler, t)
			assert.Contains(t, body, "REQUEST:")
			assert.Contains(t, body, "REQUEST_COUNT:2", "$_REQUEST should have ONLY current request's data (2 keys: use_request and val)")
			assert.Contains(t, body, fmt.Sprintf("'val' => '%d'", i), "request data is not present")
			assert.Contains(t, body, "'use_request' => '1'")
			assert.Contains(t, body, "VAL_CHECK:MATCH", "BUG: $_REQUEST contains stale data from previous request! Body: "+body)
		}
	}, &testOptions{workerScript: "request-superglobal-conditional.php", phpIni: phpIni})
}

func TestCookies_module(t *testing.T) { testCookies(t, nil) }
func TestCookies_worker(t *testing.T) { testCookies(t, &testOptions{workerScript: "cookies.php"}) }
func testCookies(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/cookies.php", nil)
		req.AddCookie(&http.Cookie{Name: "foo", Value: "bar"})
		req.AddCookie(&http.Cookie{Name: "i", Value: fmt.Sprintf("%d", i)})
		body, _ := testRequest(req, handler, t)

		assert.Contains(t, body, "'foo' => 'bar'")
		assert.Contains(t, body, fmt.Sprintf("'i' => '%d'", i))
	}, opts)
}

func TestMalformedCookie(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", "http://example.com/cookies.php", nil)
		req.Header.Add("Cookie", "foo =bar; ===;;==;  .dot.=val  ;\x00 ; PHPSESSID=1234")
		// Multiple Cookie header should be joined https://www.rfc-editor.org/rfc/rfc7540#section-8.1.2.5
		req.Header.Add("Cookie", "secondCookie=test; secondCookie=overwritten")
		body, _ := testRequest(req, handler, t)

		assert.Contains(t, body, "'foo_' => 'bar'")
		assert.Contains(t, body, "'_dot_' => 'val  '")

		// PHPSESSID should still be present since we remove the null byte
		assert.Contains(t, body, "'PHPSESSID' => '1234'")

		// The cookie in the second headers should be present,
		// but it should not be overwritten by following values
		assert.Contains(t, body, "'secondCookie' => 'test'")

	}, &testOptions{nbParallelRequests: 1})
}

func TestSession_module(t *testing.T) { testSession(t, nil) }
func TestSession_worker(t *testing.T) {
	testSession(t, &testOptions{workerScript: "session.php"})
}
func testSession(t *testing.T, opts *testOptions) {
	if opts == nil {
		opts = &testOptions{}
	}
	opts.realServer = true

	runTest(t, func(_ func(http.ResponseWriter, *http.Request), ts *httptest.Server, i int) {
		jar, err := cookiejar.New(&cookiejar.Options{})
		assert.NoError(t, err)

		client := &http.Client{Jar: jar}

		resp1, err := client.Get(ts.URL + "/session.php")
		assert.NoError(t, err)

		body1, _ := io.ReadAll(resp1.Body)
		assert.Equal(t, "Count: 0\n", string(body1))

		resp2, err := client.Get(ts.URL + "/session.php")
		assert.NoError(t, err)

		body2, _ := io.ReadAll(resp2.Body)
		assert.Equal(t, "Count: 1\n", string(body2))
	}, opts)
}

func TestPhpInfo_module(t *testing.T) { testPhpInfo(t, nil) }
func TestPhpInfo_worker(t *testing.T) { testPhpInfo(t, &testOptions{workerScript: "phpinfo.php"}) }
func testPhpInfo(t *testing.T, opts *testOptions) {
	var logOnce sync.Once
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/phpinfo.php?i=%d", i), handler, t)

		logOnce.Do(func() {
			t.Log(body)
		})

		assert.Contains(t, body, "frankenphp")
		assert.Contains(t, body, fmt.Sprintf("i=%d", i))
	}, opts)
}

func TestPersistentObject_module(t *testing.T) { testPersistentObject(t, nil) }
func TestPersistentObject_worker(t *testing.T) {
	testPersistentObject(t, &testOptions{workerScript: "persistent-object.php"})
}
func testPersistentObject(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/persistent-object.php?i=%d", i), handler, t)

		assert.Equal(t, fmt.Sprintf(`request: %d
class exists: 1
id: obj1
object id: 1`, i), body)
	}, opts)
}

func TestAutoloader_module(t *testing.T) { testAutoloader(t, nil) }
func TestAutoloader_worker(t *testing.T) {
	testAutoloader(t, &testOptions{workerScript: "autoloader.php"})
}
func testAutoloader(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/autoloader.php?i=%d", i), handler, t)

		assert.Equal(t, fmt.Sprintf(`request %d
my_autoloader`, i), body)
	}, opts)
}

func TestLog_error_log_module(t *testing.T) { testLog_error_log(t, &testOptions{}) }
func TestLog_error_log_worker(t *testing.T) {
	testLog_error_log(t, &testOptions{workerScript: "log-error_log.php"})
}
func testLog_error_log(t *testing.T, opts *testOptions) {
	var buf fmt.Stringer
	opts.logger, buf = newTestLogger(t)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/log-error_log.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		assert.Contains(t, buf.String(), fmt.Sprintf("request %d", i))
	}, opts)
}

func TestLog_frankenphp_log_module(t *testing.T) { testLog_frankenphp_log(t, &testOptions{}) }
func TestLog_frankenphp_log_worker(t *testing.T) {
	testLog_frankenphp_log(t, &testOptions{workerScript: "log-frankenphp_log.php"})
}
func testLog_frankenphp_log(t *testing.T, opts *testOptions) {
	var buf fmt.Stringer
	opts.logger, buf = newTestLogger(t)

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/log-frankenphp_log.php?i=%d", i), nil)
		w := httptest.NewRecorder()
		handler(w, req)

		logs := buf.String()
		for _, message := range []string{
			`level=INFO msg="default level message"`,
			fmt.Sprintf(`level=DEBUG msg="some debug message %d" "key int"=1`, i),
			fmt.Sprintf(`level=INFO msg="some info message %d" "key string"=string`, i),
			fmt.Sprintf(`level=WARN msg="some warn message %d"`, i),
			fmt.Sprintf(`level=ERROR msg="some error message %d" err="[a v]"`, i),
		} {
			assert.Contains(t, logs, message)
		}
	}, opts)
}

func TestConnectionAbort_module(t *testing.T) { testConnectionAbort(t, &testOptions{}) }
func TestConnectionAbort_worker(t *testing.T) {
	testConnectionAbort(t, &testOptions{workerScript: "connection_status.php"})
}
func testConnectionAbort(t *testing.T, opts *testOptions) {
	testFinish := func(finish string) {
		t.Run(fmt.Sprintf("finish=%s", finish), func(t *testing.T) {
			var buf syncBuffer
			opts.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

			runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
				req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/connection_status.php?i=%d&finish=%s", i, finish), nil)
				w := httptest.NewRecorder()

				ctx, cancel := context.WithCancel(req.Context())
				req = req.WithContext(ctx)
				cancel()
				handler(w, req)

				for !strings.Contains(buf.String(), fmt.Sprintf("request %d: 1", i)) {
				}
			}, opts)
		})
	}

	testFinish("0")
	testFinish("1")
}

func TestException_module(t *testing.T) { testException(t, &testOptions{}) }
func TestException_worker(t *testing.T) {
	testException(t, &testOptions{workerScript: "exception.php"})
}
func testException(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/exception.php?i=%d", i), handler, t)

		assert.Contains(t, body, "hello")
		assert.Contains(t, body, fmt.Sprintf(`Uncaught Exception: request %d`, i))
	}, opts)
}

func TestEarlyHints_module(t *testing.T) { testEarlyHints(t, &testOptions{}) }
func TestEarlyHints_worker(t *testing.T) {
	testEarlyHints(t, &testOptions{workerScript: "early-hints.php"})
}
func testEarlyHints(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		var earlyHintReceived bool
		trace := &httptrace.ClientTrace{
			Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
				switch code {
				case http.StatusEarlyHints:
					assert.Equal(t, "</style.css>; rel=preload; as=style", header.Get("Link"))
					assert.Equal(t, strconv.Itoa(i), header.Get("Request"))

					earlyHintReceived = true
				}

				return nil
			},
		}
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/early-hints.php?i=%d", i), nil)
		w := NewRecorder()
		w.ClientTrace = trace
		handler(w, req)

		assert.Equal(t, strconv.Itoa(i), w.Header().Get("Request"))
		assert.Equal(t, "", w.Header().Get("Link"))

		assert.True(t, earlyHintReceived)
	}, opts)
}

type streamResponseRecorder struct {
	*httptest.ResponseRecorder
	writeCallback func(buf []byte)
}

func (srr *streamResponseRecorder) Write(buf []byte) (int, error) {
	srr.writeCallback(buf)

	return srr.ResponseRecorder.Write(buf)
}

func TestFlush_module(t *testing.T) { testFlush(t, &testOptions{}) }
func TestFlush_worker(t *testing.T) {
	testFlush(t, &testOptions{workerScript: "flush.php"})
}
func testFlush(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		var j int

		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/flush.php?i=%d", i), nil)
		w := &streamResponseRecorder{httptest.NewRecorder(), func(buf []byte) {
			if j == 0 {
				assert.Equal(t, []byte("He"), buf)
			} else {
				assert.Equal(t, fmt.Appendf(nil, "llo %d", i), buf)
			}

			j++
		}}
		handler(w, req)

		assert.Equal(t, 2, j)
	}, opts)
}

func TestLargeRequest_module(t *testing.T) {
	testLargeRequest(t, &testOptions{})
}
func TestLargeRequest_worker(t *testing.T) {
	testLargeRequest(t, &testOptions{workerScript: "large-request.php"})
}
func testLargeRequest(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testPost(
			fmt.Sprintf("http://example.com/large-request.php?i=%d", i),
			strings.Repeat("f", 6_048_576),
			handler,
			t,
		)

		assert.Contains(t, body, fmt.Sprintf("Request body size: 6048576 (%d)", i))
	}, opts)
}

func TestVersion(t *testing.T) {
	v := frankenphp.Version()

	assert.GreaterOrEqual(t, v.MajorVersion, 8)
	assert.GreaterOrEqual(t, v.MinorVersion, 0)
	assert.GreaterOrEqual(t, v.ReleaseVersion, 0)
	assert.GreaterOrEqual(t, v.VersionID, 0)
	assert.NotEmpty(t, v.Version, 0)
}

func TestFiberNoCgo_module(t *testing.T) { testFiberNoCgo(t, &testOptions{}) }
func TestFiberNonCgo_worker(t *testing.T) {
	testFiberNoCgo(t, &testOptions{workerScript: "fiber-no-cgo.php"})
}
func testFiberNoCgo(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/fiber-no-cgo.php?i=%d", i), handler, t)
		assert.Equal(t, body, fmt.Sprintf("Fiber %d", i))
	}, opts)
}

func TestFiberBasic_module(t *testing.T) { testFiberBasic(t, &testOptions{}) }
func TestFiberBasic_worker(t *testing.T) {
	testFiberBasic(t, &testOptions{workerScript: "fiber-basic.php"})
}
func testFiberBasic(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/fiber-basic.php?i=%d", i), handler, t)
		assert.Equal(t, body, fmt.Sprintf("Fiber %d", i))
	}, opts)
}

func TestRequestHeaders_module(t *testing.T) { testRequestHeaders(t, &testOptions{}) }
func TestRequestHeaders_worker(t *testing.T) {
	testRequestHeaders(t, &testOptions{workerScript: "request-headers.php"})
}
func testRequestHeaders(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://example.com/request-headers.php?i=%d", i), nil)
		req.Header.Add(strings.Clone("Content-Type"), strings.Clone("text/plain"))
		req.Header.Add(strings.Clone("Frankenphp-I"), strings.Clone(strconv.Itoa(i)))
		body, _ := testRequest(req, handler, t)

		assert.Contains(t, body, "[Content-Type] => text/plain")
		assert.Contains(t, body, fmt.Sprintf("[Frankenphp-I] => %d", i))
	}, opts)
}

func TestFailingWorker(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)

	err := frankenphp.Init(
		frankenphp.WithWorkers("failing worker", "testdata/failing-worker.php", 4, frankenphp.WithWorkerMaxFailures(1)),
		frankenphp.WithNumThreads(5),
	)
	assert.Error(t, err, "should return an immediate error if workers fail on startup")
}

func TestEnv(t *testing.T) {
	testEnv(t, &testOptions{nbParallelRequests: 1})
}
func TestEnvWorker(t *testing.T) {
	testEnv(t, &testOptions{nbParallelRequests: 1, workerScript: "env/test-env.php"})
}

// testEnv cannot be run in parallel due to https://github.com/golang/go/issues/63567
func testEnv(t *testing.T, opts *testOptions) {
	assert.NoError(t, os.Setenv("EMPTY", ""))

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body, _ := testGet(fmt.Sprintf("http://example.com/env/test-env.php?var=%d", i), handler, t)

		// execute the script as regular php script
		cmd := exec.Command("php", "testdata/env/test-env.php", strconv.Itoa(i))
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			// php is not installed or other issue, use the hardcoded output below:
			stdoutStderr = []byte("Set MY_VAR successfully.\nMY_VAR = HelloWorld\nUnset MY_VAR successfully.\nMY_VAR is unset.\nMY_VAR set to empty successfully.\nMY_VAR = \nUnset NON_EXISTING_VAR successfully.\n")
		}

		assert.Equal(t, string(stdoutStderr), body)
	}, opts)
}

func TestEnvIsResetInNonWorkerMode(t *testing.T) {
	assert.NoError(t, os.Setenv("test", ""))
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		putResult, _ := testGet(fmt.Sprintf("http://example.com/env/putenv.php?key=test&put=%d", i), handler, t)

		assert.Equal(t, fmt.Sprintf("test=%d", i), putResult, "putenv and then echo getenv")

		getResult, _ := testGet("http://example.com/env/putenv.php?key=test", handler, t)

		assert.Equal(t, "test=", getResult, "putenv should be reset across requests")
	}, &testOptions{})
}

// TODO: should it actually get reset in worker mode?
func TestEnvIsNotResetInWorkerMode(t *testing.T) {
	assert.NoError(t, os.Setenv("index", ""))
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		putResult, _ := testGet(fmt.Sprintf("http://example.com/env/remember-env.php?index=%d", i), handler, t)

		assert.Equal(t, "success", putResult, "putenv and then echo getenv")

		getResult, _ := testGet("http://example.com/env/remember-env.php", handler, t)

		assert.Equal(t, "success", getResult, "putenv should not be reset across worker requests")
	}, &testOptions{workerScript: "env/remember-env.php"})
}

// reproduction of https://github.com/php/frankenphp/issues/1061
func TestModificationsToEnvPersistAcrossRequests(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		for range 3 {
			result, _ := testGet("http://example.com/env/overwrite-env.php", handler, t)
			assert.Equal(t, "custom_value", result, "a var directly added to $_ENV should persist")
		}
	}, &testOptions{
		workerScript: "env/overwrite-env.php",
		phpIni:       map[string]string{"variables_order": "EGPCS"},
	})
}

func TestFileUpload_module(t *testing.T) { testFileUpload(t, &testOptions{}) }
func TestFileUpload_worker(t *testing.T) {
	testFileUpload(t, &testOptions{workerScript: "file-upload.php"})
}
func testFileUpload(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBody := &bytes.Buffer{}
		writer := multipart.NewWriter(requestBody)
		part, _ := writer.CreateFormFile("file", "foo.txt")
		_, err := part.Write([]byte("bar"))
		require.NoError(t, err)

		require.NoError(t, writer.Close())

		req := httptest.NewRequest("POST", "http://example.com/file-upload.php", requestBody)
		req.Header.Add("Content-Type", writer.FormDataContentType())

		body, _ := testRequest(req, handler, t)

		assert.Contains(t, string(body), "Upload OK")
	}, opts)
}

func TestExecuteScriptCLI(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "testdata/command.php", "foo", "bar")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.Error(t, err)

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		assert.Equal(t, 3, exitError.ExitCode())
	}

	stdoutStderrStr := string(stdoutStderr)

	assert.Contains(t, stdoutStderrStr, `"foo"`)
	assert.Contains(t, stdoutStderrStr, `"bar"`)
	assert.Contains(t, stdoutStderrStr, "From the CLI")
}

func TestExecuteCLICode(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "-r", "echo 'Hello World';")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.NoError(t, err)

	stdoutStderrStr := string(stdoutStderr)
	assert.Equal(t, stdoutStderrStr, `Hello World`)
}

func ExampleServeHTTP() {
	if err := frankenphp.Init(); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot("/path/to/document/root", false))
		if err != nil {
			panic(err)
		}

		if err := frankenphp.ServeHTTP(w, req); err != nil {
			panic(err)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func ExampleExecuteScriptCLI() {
	if len(os.Args) <= 1 {
		log.Println("Usage: my-program script.php")
		os.Exit(1)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[1], os.Args))
}

func BenchmarkHelloWorld(b *testing.B) {
	require.NoError(b, frankenphp.Init())
	b.Cleanup(frankenphp.Shutdown)

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	opt := frankenphp.WithRequestDocumentRoot(testDataDir, false)
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, opt)
		require.NoError(b, err)

		require.NoError(b, frankenphp.ServeHTTP(w, req))
	}

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()

	for b.Loop() {
		handler(w, req)
	}
}

func BenchmarkEcho(b *testing.B) {
	require.NoError(b, frankenphp.Init())
	b.Cleanup(frankenphp.Shutdown)

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	opt := frankenphp.WithRequestDocumentRoot(testDataDir, false)
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, opt)
		require.NoError(b, err)

		require.NoError(b, frankenphp.ServeHTTP(w, req))
	}

	const body = `{
		"squadName": "Super hero squad",
		"homeTown": "Metro City",
		"formed": 2016,
		"secretBase": "Super tower",
		"active": true,
		"members": [
		  {
			"name": "Molecule Man",
			"age": 29,
			"secretIdentity": "Dan Jukes",
			"powers": ["Radiation resistance", "Turning tiny", "Radiation blast"]
		  },
		  {
			"name": "Madame Uppercut",
			"age": 39,
			"secretIdentity": "Jane Wilson",
			"powers": [
			  "Million tonne punch",
			  "Damage resistance",
			  "Superhuman reflexes"
			]
		  },
		  {
			"name": "Eternal Flame",
			"age": 1000000,
			"secretIdentity": "Unknown",
			"powers": [
			  "Immortality",
			  "Heat Immunity",
			  "Inferno",
			  "Teleportation",
			  "Interdimensional travel"
			]
		  }
		]
	  }`

	r := strings.NewReader(body)
	req := httptest.NewRequest("POST", "http://example.com/echo.php", r)
	w := httptest.NewRecorder()

	for b.Loop() {
		r.Reset(body)
		handler(w, req)
	}
}

func BenchmarkServerSuperGlobal(b *testing.B) {
	require.NoError(b, frankenphp.Init())
	b.Cleanup(frankenphp.Shutdown)

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	// Mimics headers of a request sent by Firefox to GitHub
	headers := http.Header{}
	headers.Add(strings.Clone("Accept"), strings.Clone("text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"))
	headers.Add(strings.Clone("Accept-Encoding"), strings.Clone("gzip, deflate, br"))
	headers.Add(strings.Clone("Accept-Language"), strings.Clone("fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3"))
	headers.Add(strings.Clone("Cache-Control"), strings.Clone("no-cache"))
	headers.Add(strings.Clone("Connection"), strings.Clone("keep-alive"))
	headers.Add(strings.Clone("Cookie"), strings.Clone("user_session=myrandomuuid; __Host-user_session_same_site=myotherrandomuuid; dotcom_user=dunglas; logged_in=yes; _foo=barbarbarbarbarbar; _device_id=anotherrandomuuid; color_mode=foobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobar; preferred_color_mode=light; tz=Europe%2FParis; has_recent_activity=1"))
	headers.Add(strings.Clone("DNT"), strings.Clone("1"))
	headers.Add(strings.Clone("Host"), strings.Clone("example.com"))
	headers.Add(strings.Clone("Pragma"), strings.Clone("no-cache"))
	headers.Add(strings.Clone("Sec-Fetch-Dest"), strings.Clone("document"))
	headers.Add(strings.Clone("Sec-Fetch-Mode"), strings.Clone("navigate"))
	headers.Add(strings.Clone("Sec-Fetch-Site"), strings.Clone("cross-site"))
	headers.Add(strings.Clone("Sec-GPC"), strings.Clone("1"))
	headers.Add(strings.Clone("Upgrade-Insecure-Requests"), strings.Clone("1"))
	headers.Add(strings.Clone("User-Agent"), strings.Clone("Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:122.0) Gecko/20100101 Firefox/122.0"))

	// Env vars available in a typical Docker container
	env := map[string]string{
		"HOSTNAME":        "a88e81aa22e4",
		"PHP_INI_DIR":     "/usr/local/etc/php",
		"HOME":            "/root",
		"GODEBUG":         "cgocheck=0",
		"PHP_LDFLAGS":     "-Wl,-O1 -pie",
		"PHP_CFLAGS":      "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64",
		"PHP_VERSION":     "8.3.2",
		"GPG_KEYS":        "1198C0117593497A5EC5C199286AF1F9897469DC C28D937575603EB4ABB725861C0779DC5C0A9DE4 AFD8691FDAEDF03BDF6E460563F15A9B715376CA",
		"PHP_CPPFLAGS":    "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64",
		"PHP_ASC_URL":     "https://www.php.net/distributions/php-8.3.2.tar.xz.asc",
		"PHP_URL":         "https://www.php.net/distributions/php-8.3.2.tar.xz",
		"PATH":            "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"XDG_CONFIG_HOME": "/config",
		"XDG_DATA_HOME":   "/data",
		"PHPIZE_DEPS":     "autoconf dpkg-dev file g++ gcc libc-dev make pkg-config re2c",
		"PWD":             "/app",
		"PHP_SHA256":      "4ffa3e44afc9c590e28dc0d2d31fc61f0139f8b335f11880a121b9f9b9f0634e",
	}

	preparedEnv := frankenphp.PrepareEnv(env)

	opts := []frankenphp.RequestOption{frankenphp.WithRequestDocumentRoot(testDataDir, false), frankenphp.WithRequestPreparedEnv(preparedEnv)}
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, opts...)
		require.NoError(b, err)

		r.Header = headers

		require.NoError(b, frankenphp.ServeHTTP(w, req))
	}

	req := httptest.NewRequest("GET", "http://example.com/server-variable.php", nil)
	w := httptest.NewRecorder()

	for b.Loop() {
		handler(w, req)
	}
}

func BenchmarkUncommonHeaders(b *testing.B) {
	require.NoError(b, frankenphp.Init())
	b.Cleanup(frankenphp.Shutdown)

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	// Mimics headers of a request sent by Firefox to GitHub
	headers := http.Header{}
	headers.Add(strings.Clone("Accept"), strings.Clone("text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"))
	headers.Add(strings.Clone("Accept-Encoding"), strings.Clone("gzip, deflate, br"))
	headers.Add(strings.Clone("Accept-Language"), strings.Clone("fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3"))
	headers.Add(strings.Clone("Cache-Control"), strings.Clone("no-cache"))
	headers.Add(strings.Clone("Connection"), strings.Clone("keep-alive"))
	headers.Add(strings.Clone("Cookie"), strings.Clone("user_session=myrandomuuid; __Host-user_session_same_site=myotherrandomuuid; dotcom_user=dunglas; logged_in=yes; _foo=barbarbarbarbarbar; _device_id=anotherrandomuuid; color_mode=foobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobar; preferred_color_mode=light; tz=Europe%2FParis; has_recent_activity=1"))
	headers.Add(strings.Clone("DNT"), strings.Clone("1"))
	headers.Add(strings.Clone("Host"), strings.Clone("example.com"))
	headers.Add(strings.Clone("Pragma"), strings.Clone("no-cache"))
	headers.Add(strings.Clone("Sec-Fetch-Dest"), strings.Clone("document"))
	headers.Add(strings.Clone("Sec-Fetch-Mode"), strings.Clone("navigate"))
	headers.Add(strings.Clone("Sec-Fetch-Site"), strings.Clone("cross-site"))
	headers.Add(strings.Clone("Sec-GPC"), strings.Clone("1"))
	headers.Add(strings.Clone("Upgrade-Insecure-Requests"), strings.Clone("1"))
	headers.Add(strings.Clone("User-Agent"), strings.Clone("Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:122.0) Gecko/20100101 Firefox/122.0"))
	// Some uncommon headers
	headers.Add(strings.Clone("X-Super-Custom"), strings.Clone("Foo"))
	headers.Add(strings.Clone("Super-Super-Custom"), strings.Clone("Foo"))
	headers.Add(strings.Clone("Super-Super-Custom"), strings.Clone("Bar"))
	headers.Add(strings.Clone("Very-Custom"), strings.Clone("1"))

	opt := frankenphp.WithRequestDocumentRoot(testDataDir, false)
	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, opt)
		require.NoError(b, err)

		r.Header = headers

		require.NoError(b, frankenphp.ServeHTTP(w, req))
	}

	req := httptest.NewRequest("GET", "http://example.com/server-variable.php", nil)
	w := httptest.NewRecorder()

	for b.Loop() {
		handler(w, req)
	}
}

func TestRejectInvalidHeaders_module(t *testing.T) { testRejectInvalidHeaders(t, &testOptions{}) }
func TestRejectInvalidHeaders_worker(t *testing.T) {
	testRejectInvalidHeaders(t, &testOptions{workerScript: "headers.php"})
}
func testRejectInvalidHeaders(t *testing.T, opts *testOptions) {
	invalidHeaders := [][]string{
		{"Content-Length", "-1"},
		{"Content-Length", "something"},
	}
	for _, header := range invalidHeaders {
		runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
			req := httptest.NewRequest("GET", "http://example.com/headers.php", nil)
			req.Header.Add(header[0], header[1])
			body, resp := testRequest(req, handler, t)

			assert.Equal(t, 400, resp.StatusCode)
			assert.Contains(t, body, "invalid")
		}, opts)
	}
}

func TestFlushEmptyResponse_module(t *testing.T) { testFlushEmptyResponse(t, &testOptions{}) }
func TestFlushEmptyResponse_worker(t *testing.T) {
	testFlushEmptyResponse(t, &testOptions{workerScript: "only-headers.php"})
}

func testFlushEmptyResponse(t *testing.T, opts *testOptions) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		_, resp := testGet("http://example.com/only-headers.php", handler, t)
		assert.Equal(t, 204, resp.StatusCode)
	}, opts)
}

// Worker mode will clean up unreferenced streams between requests
// Make sure referenced streams are not cleaned up
func TestFileStreamInWorkerMode(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		resp1, _ := testGet("http://example.com/file-stream.php", handler, t)
		assert.Equal(t, resp1, "word1")

		resp2, _ := testGet("http://example.com/file-stream.php", handler, t)
		assert.Equal(t, resp2, "word2")

		resp3, _ := testGet("http://example.com/file-stream.php", handler, t)
		assert.Equal(t, resp3, "word3")
	}, &testOptions{workerScript: "file-stream.php", nbParallelRequests: 1, nbWorkers: 1})
}

// To run this fuzzing test use: go test -fuzz FuzzRequest
// TODO: Cover more potential cases
func FuzzRequest(f *testing.F) {
	absPath, _ := fastabs.FastAbs("./testdata/")

	f.Add("hello world")
	f.Add("😀😅🙃🤩🥲🤪😘😇😉🐘🧟")
	f.Add("%00%11%%22%%33%%44%%55%%66%%77%%88%%99%%aa%%bb%%cc%%dd%%ee%%ff")
	f.Add("\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f")
	f.Fuzz(func(t *testing.T, fuzzedString string) {
		runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
			req := httptest.NewRequest("GET", "http://example.com/server-variable", nil)
			req.URL = &url.URL{RawQuery: "test=" + fuzzedString, Path: "/server-variable.php/" + fuzzedString}
			req.Header.Add(strings.Clone("Fuzzed"), strings.Clone(fuzzedString))
			req.Header.Add(strings.Clone("Content-Type"), fuzzedString)
			body, resp := testRequest(req, handler, t)

			// The response status must be 400 if the request path contains null bytes
			if strings.Contains(req.URL.Path, "\x00") {
				assert.Equal(t, 400, resp.StatusCode)
				assert.Contains(t, body, "invalid request path")

				return
			}

			// The fuzzed string must be present in the path
			assert.Contains(t, body, fmt.Sprintf("[PATH_INFO] => /%s", fuzzedString))
			assert.Contains(t, body, fmt.Sprintf("[PATH_TRANSLATED] => %s", filepath.Join(absPath, fuzzedString)))

			// Headers should always be present even if empty
			assert.Contains(t, body, fmt.Sprintf("[CONTENT_TYPE] => %s", fuzzedString))
			assert.Contains(t, body, fmt.Sprintf("[HTTP_FUZZED] => %s", fuzzedString))
		}, &testOptions{workerScript: "request-headers.php"})
	})
}

func TestSessionHandlerReset_worker(t *testing.T) {
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), ts *httptest.Server, i int) {
		// Request 1: Set a custom session handler and start session
		resp1, err := http.Get(ts.URL + "/session-handler.php?action=set_handler_and_start&value=test1")
		assert.NoError(t, err)
		body1, _ := io.ReadAll(resp1.Body)
		_ = resp1.Body.Close()

		body1Str := string(body1)
		assert.Contains(t, body1Str, "HANDLER_SET_AND_STARTED")
		assert.Contains(t, body1Str, "session.save_handler=user")

		// Request 2: Start session without setting a custom handler
		// The user handler from request 1 is preserved (mod_user_names persist),
		// so session_start() should work without crashing.
		resp2, err := http.Get(ts.URL + "/session-handler.php?action=start_without_handler")
		assert.NoError(t, err)
		body2, _ := io.ReadAll(resp2.Body)
		_ = resp2.Body.Close()

		body2Str := string(body2)

		// session_start() should succeed (handlers are preserved)
		assert.Contains(t, body2Str, "SESSION_START_RESULT=true",
			"session_start() should succeed.\nResponse: %s", body2Str)

		// No errors or exceptions should occur
		assert.NotContains(t, body2Str, "ERROR:",
			"No errors expected.\nResponse: %s", body2Str)
		assert.NotContains(t, body2Str, "EXCEPTION:",
			"No exceptions expected.\nResponse: %s", body2Str)

	}, &testOptions{
		workerScript:       "session-handler.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		realServer:         true,
	})
}

func TestSessionHandlerPreLoopPreserved_worker(t *testing.T) {
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), ts *httptest.Server, i int) {
		// Request 1: Check that the pre-loop session handler is preserved
		resp1, err := http.Get(ts.URL + "/worker-with-session-handler.php?action=check")
		assert.NoError(t, err)
		body1, _ := io.ReadAll(resp1.Body)
		_ = resp1.Body.Close()

		body1Str := string(body1)
		t.Logf("Request 1 response: %s", body1Str)
		assert.Contains(t, body1Str, "HANDLER_PRESERVED",
			"Session handler set before loop should be preserved")
		assert.Contains(t, body1Str, "save_handler=user",
			"session.save_handler should remain 'user'")

		// Request 2: Use the session - should work with pre-loop handler
		resp2, err := http.Get(ts.URL + "/worker-with-session-handler.php?action=use_session")
		assert.NoError(t, err)
		body2, _ := io.ReadAll(resp2.Body)
		_ = resp2.Body.Close()

		body2Str := string(body2)
		t.Logf("Request 2 response: %s", body2Str)
		assert.Contains(t, body2Str, "SESSION_OK",
			"Session should work with pre-loop handler.\nResponse: %s", body2Str)
		assert.NotContains(t, body2Str, "ERROR:",
			"No errors expected.\nResponse: %s", body2Str)
		assert.NotContains(t, body2Str, "EXCEPTION:",
			"No exceptions expected.\nResponse: %s", body2Str)

		// Request 3: Check handler is still preserved after using session
		resp3, err := http.Get(ts.URL + "/worker-with-session-handler.php?action=check")
		assert.NoError(t, err)
		body3, _ := io.ReadAll(resp3.Body)
		_ = resp3.Body.Close()

		body3Str := string(body3)
		t.Logf("Request 3 response: %s", body3Str)
		assert.Contains(t, body3Str, "HANDLER_PRESERVED",
			"Session handler should still be preserved after use")

	}, &testOptions{
		workerScript:       "worker-with-session-handler.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		realServer:         true,
	})
}

func TestSessionNoLeakBetweenRequests_worker(t *testing.T) {
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), ts *httptest.Server, i int) {
		// Client A: Set a secret value in session
		clientA := &http.Client{}
		resp1, err := clientA.Get(ts.URL + "/session-leak.php?action=set&value=secret_A&client_id=clientA")
		assert.NoError(t, err)
		body1, _ := io.ReadAll(resp1.Body)
		_ = resp1.Body.Close()

		body1Str := string(body1)
		t.Logf("Client A set session: %s", body1Str)
		assert.Contains(t, body1Str, "SESSION_SET")
		assert.Contains(t, body1Str, "secret=secret_A")

		// Client B: Check that session is empty (no cookie, should not see Client A's data)
		clientB := &http.Client{}
		resp2, err := clientB.Get(ts.URL + "/session-leak.php?action=check_empty")
		assert.NoError(t, err)
		body2, _ := io.ReadAll(resp2.Body)
		_ = resp2.Body.Close()

		body2Str := string(body2)
		t.Logf("Client B check empty: %s", body2Str)
		assert.Contains(t, body2Str, "SESSION_CHECK")
		assert.Contains(t, body2Str, "SESSION_EMPTY=true",
			"Client B should have empty session, not see Client A's data.\nResponse: %s", body2Str)
		assert.NotContains(t, body2Str, "secret_A",
			"Client A's secret should not leak to Client B.\nResponse: %s", body2Str)

		// Client C: Read session without cookie (should also be empty)
		clientC := &http.Client{}
		resp3, err := clientC.Get(ts.URL + "/session-leak.php?action=get")
		assert.NoError(t, err)
		body3, _ := io.ReadAll(resp3.Body)
		_ = resp3.Body.Close()

		body3Str := string(body3)
		t.Logf("Client C get session: %s", body3Str)
		assert.Contains(t, body3Str, "SESSION_READ")
		assert.Contains(t, body3Str, "secret=NOT_FOUND",
			"Client C should not find any secret.\nResponse: %s", body3Str)
		assert.Contains(t, body3Str, "client_id=NOT_FOUND",
			"Client C should not find any client_id.\nResponse: %s", body3Str)

	}, &testOptions{
		workerScript:       "session-leak.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		realServer:         true,
	})
}

func TestSessionNoLeakAfterExit_worker(t *testing.T) {
	runTest(t, func(_ func(http.ResponseWriter, *http.Request), ts *httptest.Server, i int) {
		// Client A: Set a secret value in session and call exit(1)
		clientA := &http.Client{}
		resp1, err := clientA.Get(ts.URL + "/session-leak.php?action=set_and_exit&value=exit_secret&client_id=exitClient")
		assert.NoError(t, err)
		body1, _ := io.ReadAll(resp1.Body)
		_ = resp1.Body.Close()

		body1Str := string(body1)
		t.Logf("Client A set and exit: %s", body1Str)
		// The response may be incomplete due to exit(1)
		assert.Contains(t, body1Str, "BEFORE_EXIT")

		// Client B: Check that session is empty (should not see Client A's data)
		// Retry until the worker has restarted after exit(1)
		clientB := &http.Client{}
		var body2Str string
		assert.Eventually(t, func() bool {
			resp2, err := clientB.Get(ts.URL + "/session-leak.php?action=check_empty")
			if err != nil {
				return false
			}
			body2, _ := io.ReadAll(resp2.Body)
			_ = resp2.Body.Close()
			body2Str = string(body2)
			return strings.Contains(body2Str, "SESSION_CHECK")
		}, 2*time.Second, 10*time.Millisecond, "Worker did not restart in time after exit(1)")

		t.Logf("Client B check empty after exit: %s", body2Str)
		assert.Contains(t, body2Str, "SESSION_EMPTY=true",
			"Client B should have empty session after Client A's exit(1).\nResponse: %s", body2Str)
		assert.NotContains(t, body2Str, "exit_secret",
			"Client A's secret should not leak to Client B after exit(1).\nResponse: %s", body2Str)

		// Client C: Try to read session (should also be empty)
		clientC := &http.Client{}
		resp3, err := clientC.Get(ts.URL + "/session-leak.php?action=get")
		assert.NoError(t, err)
		body3, _ := io.ReadAll(resp3.Body)
		_ = resp3.Body.Close()

		body3Str := string(body3)
		t.Logf("Client C get session after exit: %s", body3Str)
		assert.Contains(t, body3Str, "SESSION_READ")
		assert.Contains(t, body3Str, "secret=NOT_FOUND",
			"Client C should not find any secret after exit(1).\nResponse: %s", body3Str)

	}, &testOptions{
		workerScript:       "session-leak.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		realServer:         true,
	})
}
