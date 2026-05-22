package phpheaders

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllCommonHeadersAreCorrect(t *testing.T) {
	fakeRequest := httptest.NewRequest("GET", "http://localhost", nil)

	for header, phpHeader := range CommonRequestHeaders {
		// verify that common and uncommon headers return the same result
		expectedPHPHeader := GetUnCommonHeader(t.Context(), header)
		assert.Equal(t, phpHeader+"\x00", expectedPHPHeader, "header is not well formed: "+phpHeader)

		// net/http will capitalize lowercase headers, verify that headers are capitalized
		fakeRequest.Header.Add(header, "foo")
		assert.Contains(t, fakeRequest.Header, header, "header is not correctly capitalized: "+header)
	}
}

// Go's net/http server rejects header names containing spaces with a 400 response
// before the request reaches the handler, so headerNameReplacer does not need to
// translate spaces to underscores.
func TestHeaderWithSpaceIsRejectedByNetHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler should not be reached, got headers: %v", r.Header)
	}))
	t.Cleanup(ts.Close)

	conn, err := net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\nBad Header: x\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)

	resp, err := io.ReadAll(conn)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "400 Bad Request")
	assert.Contains(t, string(resp), "invalid header name")
}
