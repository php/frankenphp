package testext

// #cgo darwin pkg-config: libxml-2.0
// #cgo CFLAGS: -Wall -Werror
// #cgo CFLAGS: -I/usr/local/include -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo darwin CFLAGS: -I/opt/homebrew/include
// #cgo LDFLAGS: -L/usr/local/lib -L/usr/lib -lphp-zts-85 -lm -lutil
// #cgo linux LDFLAGS: -ldl -lresolv
// #cgo darwin LDFLAGS: -Wl,-rpath,/usr/local/lib -L/opt/homebrew/lib -L/opt/homebrew/opt/libiconv/lib -liconv -ldl
// #include "extension.h"
import "C"
import (
	"io"
	"net/http/httptest"
	"testing"
	"unsafe"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRegisterExtension(t *testing.T) {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.module1_entry))
	frankenphp.RegisterExtension(unsafe.Pointer(&C.module2_entry))

	err := frankenphp.Init()
	require.Nil(t, err)
	defer frankenphp.Shutdown()

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()

	req, err = frankenphp.NewRequestWithContext(req, frankenphp.WithRequestDocumentRoot("./testdata", false))
	assert.NoError(t, err)

	err = frankenphp.ServeHTTP(w, req)
	assert.NoError(t, err)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "ext1")
	assert.Contains(t, string(body), "ext2")
}
