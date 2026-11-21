package frankenphp_test

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type formParserCase struct {
	name        string
	method      string
	query       string
	contentType string
	body        string
	// hides the length, so only the bytes read can exceed post_max_size
	chunked bool
}

const formBoundary = "frankenphpboundary"

// multipartBody joins raw parts, so tests control every byte sent
func multipartBody(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		fmt.Fprintf(&b, "--%s\r\n%s\r\n", formBoundary, p)
	}
	b.WriteString("--" + formBoundary + "--\r\n")

	return b.String()
}

func field(name, value string) string {
	return `Content-Disposition: form-data; name="` + name + "\"\r\n\r\n" + value
}

func file(name, filename, contentType, content string) string {
	return `Content-Disposition: form-data; name="` + name + `"; filename="` + filename + "\"\r\nContent-Type: " + contentType + "\r\n\r\n" + content
}

func formParserCases() []formParserCase {
	multipartType := "multipart/form-data; boundary=" + formBoundary
	urlencodedType := "application/x-www-form-urlencoded"

	cases := []formParserCase{
		{name: "fields and file", contentType: multipartType, body: multipartBody(
			field("foo", "bar"),
			file("upload", "foo.txt", "text/plain", "hello"),
		)},
		{name: "nested names", contentType: multipartType, body: multipartBody(
			field("a[b][c]", "1"),
			field("a.b c", "2"),
			field("list[]", "3"),
			field("list[]", "4"),
			file("files[]", "a.txt", "text/plain", "a"),
			file("files[]", "b.txt", "text/plain", "b"),
			file("deep[x][y]", "c.txt", "text/plain", "c"),
		)},
		{name: "MAX_FILE_SIZE before the file", contentType: multipartType, body: multipartBody(
			field("MAX_FILE_SIZE", "3"),
			file("upload", "foo.txt", "text/plain", "too long"),
			file("small", "s.txt", "text/plain", "ok"),
		)},
		{name: "MAX_FILE_SIZE after the file", contentType: multipartType, body: multipartBody(
			file("upload", "foo.txt", "text/plain", "too long"),
			field("MAX_FILE_SIZE", "3"),
		)},
		{name: "upload_max_filesize", contentType: multipartType, body: multipartBody(
			file("upload", "big.txt", "text/plain", strings.Repeat("x", 21)),
		)},
		{name: "empty file input", contentType: multipartType, body: multipartBody(
			file("upload", "", "application/octet-stream", ""),
		)},
		{name: "empty file", contentType: multipartType, body: multipartBody(
			file("upload", "empty.txt", "text/plain", ""),
		)},
		{name: "paths", contentType: multipartType, body: multipartBody(
			file("unix", "dir/sub/a.txt", "text/plain", "a"),
			file("windows", `C:\dir\b.txt`, "text/plain", "b"),
		)},
		{name: "content type parameters", contentType: multipartType, body: multipartBody(
			file("upload", "a.txt", "text/plain; charset=utf-8", "a"),
		)},
		{name: "invalid names", contentType: multipartType, body: multipartBody(
			file("bad[", "a.txt", "text/plain", "a"),
			file("bad]x", "b.txt", "text/plain", "b"),
		)},
		{name: "empty file inputs don't count as uploads", contentType: multipartType, body: multipartBody(
			file("e1", "", "application/octet-stream", ""),
			file("e2", "", "application/octet-stream", ""),
			file("e3", "", "application/octet-stream", ""),
			file("e4", "", "application/octet-stream", ""),
			file("e5", "", "application/octet-stream", ""),
			file("upload", "a.txt", "text/plain", "a"),
		)},
		{name: "protected file variables", contentType: multipartType, body: multipartBody(
			file("f[name]", "", "application/octet-stream", ""),
			file("f", "", "application/octet-stream", ""),
			file("g", "a.txt", "text/plain", "a"),
			file("g[tmp_name]", "b.txt", "text/plain", "b"),
			file(" h.x", "c.txt", "text/plain", "c"),
			file("h_x", "", "application/octet-stream", ""),
		)},
		{name: "urlencoded", contentType: urlencodedType, body: "a=1&b[]=2&b[]=3&c=%ZZ%41&d=x;y&e+f=g+h&&=z&k&nul=a%00b&%61%5Bx%5D=y"},
		{name: "urlencoded trailing separator", contentType: urlencodedType, body: "a=1&"},
	}

	if frankenphp.Version().VersionID >= 80400 {
		cases = append(cases,
			formParserCase{name: "request_parse_body multipart", method: http.MethodPut, query: "parse_body=" + url.QueryEscape(`{"max_file_uploads":1}`), contentType: multipartType, body: multipartBody(
				field("foo", "bar"),
				file("f1", "1.txt", "text/plain", "1"),
				file("f2", "2.txt", "text/plain", "2"),
			)},
			formParserCase{name: "request_parse_body post_max_size", method: http.MethodPut, query: "parse_body=" + url.QueryEscape(`{"post_max_size":10}`), contentType: multipartType, body: multipartBody(
				field("foo", "bar"),
			)},
			formParserCase{name: "request_parse_body urlencoded", method: http.MethodPut, query: "parse_body=null", contentType: urlencodedType, body: "a=1&b[c]=2"},
			formParserCase{name: "request_parse_body post_max_size PHP_INT_MAX", method: http.MethodPut, query: "parse_body=" + url.QueryEscape(`{"post_max_size":9223372036854775807}`), contentType: multipartType, body: multipartBody(
				field("foo", "bar"),
				file("upload", "a.txt", "text/plain", "a"),
			)},
		)
	}

	return cases
}

func dumpForms(t *testing.T, cases []formParserCase, goParser bool, workerScript string, phpIni map[string]string) map[string]string {
	t.Helper()

	results := make(map[string]string, len(cases))
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for _, c := range cases {
			method := c.method
			if method == "" {
				method = http.MethodPost
			}

			req := httptest.NewRequest(method, "http://example.com/form-dump.php?"+c.query, strings.NewReader(c.body))
			req.Header.Set("Content-Type", c.contentType)
			if c.chunked {
				req.ContentLength = -1
			}

			results[c.name], _ = testRequest(req, handler, t)
		}
	}, &testOptions{
		workerScript:       workerScript,
		nbParallelRequests: 1,
		phpIni:             phpIni,
		initOpts:           []frankenphp.Option{frankenphp.WithGoFormParser(goParser)},
	})

	return results
}

func TestGoFormParserMatchesPHP(t *testing.T) {
	tmpDir := t.TempDir()
	phpIni := maps.Clone(formParserIni)
	phpIni["upload_tmp_dir"] = iniString(tmpDir)

	cases := formParserCases()
	for _, worker := range []string{"", "form-dump.php"} {
		expected := dumpForms(t, cases, false, worker, phpIni)
		actual := dumpForms(t, cases, true, worker, phpIni)
		for _, c := range cases {
			assert.Equal(t, expected[c.name], actual[c.name], "case %q (worker script: %q)", c.name, worker)
		}
	}

	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "uploaded files must be removed after the request")
}

// iniString quotes paths: Windows temp dirs contain "~", an INI operator.
func iniString(s string) string {
	return `"` + s + `"`
}

var formParserIni = map[string]string{
	"display_errors":           "0",
	"upload_max_filesize":      "20",
	"max_file_uploads":         "4",
	"max_input_vars":           "30",
	"max_multipart_body_parts": "40",
}

// Bodies PHP would partially parse: 400 or 413 and the script doesn't run, or request_parse_body() throws
func TestGoFormParserRejections(t *testing.T) {
	multipartType := "multipart/form-data; boundary=" + formBoundary

	repeat := func(n int, part string) []string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = part
		}

		return parts
	}

	cases := []struct {
		name, contentType, body string
		status                  int
		reason                  string
	}{
		{"missing boundary", "multipart/form-data", multipartBody(field("foo", "bar")), http.StatusBadRequest, "Missing boundary in multipart/form-data POST data"},
		{"empty body", multipartType, "", http.StatusBadRequest, "Invalid multipart/form-data POST data"},
		{"truncated body", multipartType, "--" + formBoundary + "\r\n" + field("foo", "bar") + "\r\n--" + formBoundary + "\r\n" + file("upload", "a.txt", "text/plain", "partial"), http.StatusBadRequest, "Invalid multipart/form-data POST data"},
		{"part without disposition", multipartType, multipartBody("Content-Type: text/plain\r\n\r\nignored"), http.StatusBadRequest, "File Upload Mime headers garbled"},
		{"part without name", multipartType, multipartBody("Content-Disposition: form-data; filename=\"anon.txt\"\r\n\r\nanon"), http.StatusBadRequest, "File Upload Mime headers garbled"},
		{"disposition other than form-data", multipartType, multipartBody("Content-Disposition: attachment; name=\"foo\"\r\n\r\nbar"), http.StatusBadRequest, "File Upload Mime headers garbled"},
		{"max_input_vars", multipartType, multipartBody(repeat(31, field("f[]", "v"))...), http.StatusRequestEntityTooLarge, "Input variables exceeded 30"},
		{"max_file_uploads", multipartType, multipartBody(repeat(5, file("f[]", "a.txt", "text/plain", "a"))...), http.StatusRequestEntityTooLarge, "Maximum number of allowable file uploads has been exceeded"},
		// files with names PHP never registers still count as parts
		{"max_multipart_body_parts", multipartType, multipartBody(repeat(41, file("bad[", "a.txt", "text/plain", "a"))...), http.StatusRequestEntityTooLarge, "Multipart body parts limit exceeded 40"},
		{"urlencoded max_input_vars", "application/x-www-form-urlencoded", strings.Repeat("a[]=1&", 31), http.StatusRequestEntityTooLarge, "Input variables exceeded 30"},
		// empty pairs count too, and must not size the parser's buffers
		{"urlencoded empty pairs", "application/x-www-form-urlencoded", strings.Repeat("&", 1<<20), http.StatusRequestEntityTooLarge, "Input variables exceeded 30"},
	}

	tmpDir := t.TempDir()
	phpIni := maps.Clone(formParserIni)
	phpIni["upload_tmp_dir"] = iniString(tmpDir)

	for _, worker := range []string{"", "form-dump.php"} {
		runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
			for _, c := range cases {
				req := httptest.NewRequest(http.MethodPost, "http://example.com/form-dump.php", strings.NewReader(c.body))
				req.Header.Set("Content-Type", c.contentType)
				body, resp := testRequest(req, handler, t)

				assert.Equal(t, c.status, resp.StatusCode, "case %q (worker script: %q)", c.name, worker)
				assert.NotContains(t, body, "array (", "case %q (worker script: %q)", c.name, worker)

				if frankenphp.Version().VersionID < 80400 {
					continue
				}

				req = httptest.NewRequest(http.MethodPut, "http://example.com/form-dump.php?parse_body=null", strings.NewReader(c.body))
				req.Header.Set("Content-Type", c.contentType)
				body, resp = testRequest(req, handler, t)

				assert.Equal(t, http.StatusOK, resp.StatusCode, "case %q (worker script: %q)", c.name, worker)
				assert.Contains(t, body, "RequestParseBodyException: "+c.reason, "case %q (worker script: %q)", c.name, worker)
			}
		}, &testOptions{
			workerScript:       worker,
			nbParallelRequests: 1,
			phpIni:             phpIni,
			initOpts:           []frankenphp.Option{frankenphp.WithGoFormParser(true)},
		})
	}

	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "files of a rejected body must be removed")
}

// A worker runs in its script's directory: PHP resolves a relative upload_tmp_dir from there, not from the process'
func TestGoFormParserRelativeUploadTmpDir(t *testing.T) {
	dir, err := os.MkdirTemp("testdata", "uploads-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	phpIni := maps.Clone(formParserIni)
	phpIni["upload_tmp_dir"] = filepath.Base(dir)

	cases := []formParserCase{{name: "relative upload_tmp_dir", contentType: "multipart/form-data; boundary=" + formBoundary, body: multipartBody(file("upload", "a.txt", "text/plain", "content"))}}
	expected := dumpForms(t, cases, false, "form-dump.php", phpIni)
	actual := dumpForms(t, cases, true, "form-dump.php", phpIni)

	assert.Contains(t, expected["relative upload_tmp_dir"], "0 => 'content'")
	assert.Equal(t, expected["relative upload_tmp_dir"], actual["relative upload_tmp_dir"])

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "uploaded files must be removed after the request")
}

// PHP only warns, and parses what fits
func TestGoFormParserURLEncodedRequestParseBodyPostMaxSize(t *testing.T) {
	if frankenphp.Version().VersionID < 80400 {
		t.Skip("request_parse_body() requires PHP 8.4")
	}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for _, chunked := range []bool{false, true} {
			req := httptest.NewRequest(http.MethodPut, "http://example.com/form-dump.php?parse_body="+url.QueryEscape(`{"post_max_size":10}`), strings.NewReader("a="+strings.Repeat("x", 20)))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			reason := "POST Content-Length of 22 bytes exceeds the limit of 10 bytes"
			if chunked {
				req.ContentLength = -1
				reason = "Actual POST length does not match Content-Length, and exceeds 10 bytes"
			}

			body, resp := testRequest(req, handler, t)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "RequestParseBodyException: "+reason, "chunked: %t", chunked)
			assert.NotContains(t, body, "'a' =>", "chunked: %t", chunked)
		}
	}, &testOptions{
		nbParallelRequests: 1,
		phpIni:             formParserIni,
		initOpts:           []frankenphp.Option{frankenphp.WithGoFormParser(true)},
	})
}

// PHP ignores RFC 2231 parameters and would register the part as a field
func TestGoFormParserRFC2231Filename(t *testing.T) {
	body := multipartBody("Content-Disposition: form-data; name=\"doc\"; filename*=UTF-8''%C3%A9t%C3%A9.txt\r\nContent-Type: text/plain\r\n\r\ncontent")
	results := dumpForms(t, []formParserCase{{name: "rfc2231", contentType: "multipart/form-data; boundary=" + formBoundary, body: body}}, true, "", map[string]string{"display_errors": "0"})

	assert.Contains(t, results["rfc2231"], "'name' => 'été.txt'")
	assert.Contains(t, results["rfc2231"], "0 => 'content'")
}

// Like Caddy's request_body max_size, post_max_size answers 413 and the script doesn't run
func TestGoFormParserPostMaxSize(t *testing.T) {
	tmpDir := t.TempDir()

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for _, contentType := range []string{"multipart/form-data", "application/x-www-form-urlencoded"} {
			for _, size := range []int{512, 4096} {
				for _, chunked := range []bool{false, true} {
					t.Run(fmt.Sprintf("%q %d chunked=%t", contentType, size, chunked), func(t *testing.T) {
						marker := filepath.Join(t.TempDir(), "marker")
						w := httptest.NewRecorder()
						handler(w, newBodyRequest(t, http.MethodPost, "http://example.com/read-input-then-touch.php?marker="+marker, contentType, size, chunked))

						if size <= 1024 {
							assert.Equal(t, http.StatusOK, w.Code)
							assert.FileExists(t, marker)

							return
						}

						assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
						waitForThread(t, handler)
						assert.NoFileExists(t, marker)
						assert.NoFileExists(t, marker+"-caught")
					})
				}
			}
		}
	}, &testOptions{
		nbParallelRequests: 1,
		phpIni:             map[string]string{"post_max_size": "1K", "upload_tmp_dir": iniString(tmpDir)},
		initOpts:           []frankenphp.Option{frankenphp.WithNumThreads(1), frankenphp.WithGoFormParser(true)},
	})

	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGoFormParserWorkerPostMaxSize(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, _ int) {
		for _, contentType := range []string{"multipart/form-data", "application/x-www-form-urlencoded"} {
			w := httptest.NewRecorder()
			handler(w, newBodyRequest(t, http.MethodPost, "http://example.com/worker-read-input.php", contentType, 4096, true))
			assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, contentType)
			assert.NotContains(t, w.Body.String(), "count=", contentType)
		}

		// the worker wasn't restarted
		w := httptest.NewRecorder()
		handler(w, newBodyRequest(t, http.MethodPost, "http://example.com/worker-read-input.php", "", 512, false))
		assert.Equal(t, "count=1 finally=0 read=512", w.Body.String())
	}, &testOptions{
		workerScript:       "worker-read-input.php",
		nbWorkers:          1,
		nbParallelRequests: 1,
		phpIni:             map[string]string{"post_max_size": "1K"},
		initOpts:           []frankenphp.Option{frankenphp.WithGoFormParser(true)},
	})
}
