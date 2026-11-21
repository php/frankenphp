package frankenphp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

func BenchmarkFormParser(b *testing.B) {
	multipartType := "multipart/form-data; boundary=" + formBoundary

	fields := func(n int) []string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = field(fmt.Sprintf("field%d", i), strings.Repeat("v", 32))
		}

		return parts
	}
	files := func(n, size int) []string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = file(fmt.Sprintf("file%d", i), "a.bin", "application/octet-stream", strings.Repeat("x", size))
		}

		return parts
	}
	urlencoded := func(n int) string {
		pairs := make([]string, n)
		for i := range pairs {
			pairs[i] = fmt.Sprintf("field%d=%s", i, strings.Repeat("v%20", 8))
		}

		return strings.Join(pairs, "&")
	}

	payloads := []struct{ name, contentType, body string }{
		{"urlencoded/10 fields", "application/x-www-form-urlencoded", urlencoded(10)},
		{"urlencoded/1000 fields", "application/x-www-form-urlencoded", urlencoded(1000)},
		{"multipart/10 fields", multipartType, multipartBody(fields(10)...)},
		{"multipart/1000 fields", multipartType, multipartBody(fields(1000)...)},
		{"multipart/1 file 1KiB", multipartType, multipartBody(files(1, 1<<10)...)},
		{"multipart/1 file 1MiB", multipartType, multipartBody(files(1, 1<<20)...)},
		{"multipart/10 files 100KiB", multipartType, multipartBody(files(10, 100<<10)...)},
	}

	cwd, _ := os.Getwd()
	testDataDir := cwd + "/testdata/"

	for _, parser := range []string{"php", "go"} {
		b.Run("parser="+parser, func(b *testing.B) {
			require.NoError(b, frankenphp.Init(
				frankenphp.WithGoFormParser(parser == "go"),
				frankenphp.WithWorkers("form-count", testDataDir+"form-count.php", 1),
				frankenphp.WithNumThreads(2),
				frankenphp.WithPhpIni(map[string]string{"post_max_size": "64M", "upload_max_filesize": "64M", "max_file_uploads": "100", "max_input_vars": "10000"}),
			))
			b.Cleanup(frankenphp.Shutdown)

			handler := func(w http.ResponseWriter, r *http.Request) {
				req, err := frankenphp.NewRequestWithContext(r, frankenphp.WithRequestDocumentRoot(testDataDir, false))
				require.NoError(b, err)
				require.NoError(b, frankenphp.ServeHTTP(w, req))
			}

			for _, p := range payloads {
				b.Run(p.name, func(b *testing.B) {
					b.SetBytes(int64(len(p.body)))
					b.ReportAllocs()

					for b.Loop() {
						req := httptest.NewRequest(http.MethodPost, "http://example.com/form-count.php", strings.NewReader(p.body))
						req.Header.Set("Content-Type", p.contentType)
						w := httptest.NewRecorder()
						handler(w, req)

						if w.Code != http.StatusOK || strings.HasPrefix(w.Body.String(), "0 0") {
							b.Fatalf("unexpected response %d: %s", w.Code, w.Body.String())
						}
					}
				})
			}
		})
	}
}
