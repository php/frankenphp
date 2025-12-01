package phpheaders

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllCommonHeadersAreCorrect(t *testing.T) {
	keys := make([]string, 0, len(CommonRequestHeaders))
	for k := range CommonRequestHeaders {
		keys = append(keys, k)
	}
	uncommonHeaders := GetUnCommonHeaders(t.Context(), keys)
	fakeRequest := httptest.NewRequest("GET", "http://localhost", nil)

	for header, phpHeader := range CommonRequestHeaders {
		// verify that common and uncommon headers return the same result
		assert.Equal(t, phpHeader+"\x00", uncommonHeaders[header], "header is not well formed: "+phpHeader)

		// net/http will capitalize lowercase headers, verify that headers are capitalized
		fakeRequest.Header.Add(header, "foo")
		assert.Contains(t, fakeRequest.Header, header, "header is not correctly capitalized: "+header)
	}
}
