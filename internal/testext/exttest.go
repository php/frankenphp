package testext

// #cgo darwin pkg-config: libxml-2.0
// #cgo unix CFLAGS: -Wall -Werror
// #cgo unix CFLAGS: -I/usr/local/include -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo darwin CFLAGS: -I/opt/homebrew/include
// #cgo unix LDFLAGS: -L/usr/local/lib -L/usr/lib -lphp -lm -lutil
// #cgo linux LDFLAGS: -ldl -lresolv
// #cgo darwin LDFLAGS: -Wl,-rpath,/usr/local/lib -L/opt/homebrew/lib -L/opt/homebrew/opt/libiconv/lib -liconv -ldl
// #cgo windows CFLAGS: -D_WINDOWS -DWINDOWS=1 -DZEND_WIN32=1 -DPHP_WIN32=1 -DWIN32 -D_MBCS -D_USE_MATH_DEFINES -DNDebug -DNDEBUG -DZEND_DEBUG=0 -DZTS=1 -DFD_SETSIZE=256 -DENABLE_INTSAFE_SIGNED_FUNCTIONS
// #include "extension.h"
import "C"
import (
	"io"
	"net/http/httptest"
	"runtime"
	"testing"
	"unsafe"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRegisterExtension(t *testing.T) {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.module1_entry))
	frankenphp.RegisterExtension(unsafe.Pointer(&C.module2_entry))

	// race the GC against C reading the raw array
	stop := make(chan struct{})
	go func() {
		var sink [][]unsafe.Pointer
		for {
			select {
			case <-stop:
				return
			default:
			}
			runtime.GC()
			for i := 0; i < 20000; i++ {
				sink = append(sink, make([]unsafe.Pointer, 2))
				if len(sink) > 100000 {
					sink = nil
				}
			}
		}
	}()
	defer close(stop)

	t.Log("initializing PHP with the registered extensions")
	err := frankenphp.Init()
	require.Nil(t, err)
	defer func() {
		t.Log("shutting down PHP")
		frankenphp.Shutdown()
		t.Log("PHP shutdown completed")
	}()

	t.Log("checking the signal handler")
	require.Equal(t, int(C.SUCCESS), int(C.test_signal_handler()))
	t.Log("checking recovery from a Go nil-pointer panic")
	assert.Panics(t, func() {
		var p *int
		_ = *p
	})

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()

	req, err = frankenphp.NewRequestWithContext(req, frankenphp.WithRequestDocumentRoot("./testdata", false))
	assert.NoError(t, err)

	t.Log("serving the extension-list request")
	err = frankenphp.ServeHTTP(w, req)
	assert.NoError(t, err)

	t.Log("checking the registered extension names")
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "ext1")
	assert.Contains(t, string(body), "ext2")
}
