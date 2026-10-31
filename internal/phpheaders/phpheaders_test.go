package phpheaders

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http/httpguts"
)

func TestAllCommonHeadersAreCorrect(t *testing.T) {
	fakeRequest := httptest.NewRequest("GET", "http://localhost", nil)

	for header, phpHeader := range CommonRequestHeaders {
		// verify that common and uncommon headers return the same result
		expectedPHPHeader := GetUnCommonHeader(header)
		assert.Equal(t, phpHeader+"\x00", expectedPHPHeader, "header is not well formed: "+phpHeader)

		// net/http will capitalize lowercase headers, verify that headers are capitalized
		fakeRequest.Header.Add(header, "foo")
		assert.Contains(t, fakeRequest.Header, header, "header is not correctly capitalized: "+header)
	}
}

// Header names arriving over HTTP must be valid ASCII before conversion to PHP variables.
func TestInvalidHeaderNamesAreRejectedByNetHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler should not be reached, got headers: %v", r.Header)
	}))
	t.Cleanup(ts.Close)

	for _, name := range []string{"Bad Header", "X-Ä-Trace", "X-ı-ſ", "X-\xff-Trace", "X-\x00-Trace"} {
		t.Run(name, func(t *testing.T) {
			conn, err := net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, conn.Close()) })

			_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n" + name + ": x\r\nConnection: close\r\n\r\n"))
			require.NoError(t, err)

			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestHeaderName(t *testing.T) {
	for _, key := range []string{
		"X-Custom-Trace-Id", "X-CUSTOM-TRACE-ID", "x-custom-trace-id",
		"X-With_Underscore.And.Dot", "!#$%&'*+-.^_`|~0123456789",
	} {
		t.Run(key, func(t *testing.T) {
			want := "HTTP_" + strings.NewReplacer("-", "_").Replace(strings.ToUpper(key)) + "\x00"
			assert.Equal(t, want, headerName(key))
		})
	}
}

func FuzzHeaderName(f *testing.F) {
	for _, key := range []string{"X-Custom-Trace-Id", "x-custom-trace-id", "!#$%&'*+-.^_`|~0123456789"} {
		f.Add(key)
	}
	replacer := strings.NewReplacer("-", "_")
	f.Fuzz(func(t *testing.T, key string) {
		if !httpguts.ValidHeaderFieldName(key) {
			return
		}
		want := "HTTP_" + replacer.Replace(strings.ToUpper(key)) + "\x00"
		if got := headerName(key); got != want {
			t.Fatalf("headerName(%q) = %q, want %q", key, got, want)
		}
	})
}

var headerNameResult string

func BenchmarkHeaderName(b *testing.B) {
	for _, key := range []string{"X-Custom-Trace-Id", "X-CUSTOM-TRACE-ID", "x-custom-trace-id"} {
		b.Run(key, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				headerNameResult = headerName(key)
			}
		})
	}
}
