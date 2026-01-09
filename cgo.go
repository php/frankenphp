package frankenphp

// #cgo darwin pkg-config: libxml-2.0
// #cgo unix CFLAGS: -Wall -Werror
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo unix LDFLAGS: -lphp -lm -lutil
// #cgo linux LDFLAGS: -ldl -lresolv
// #cgo darwin LDFLAGS: -Wl,-rpath,/usr/local/lib -liconv -ldl
// #cgo windows CFLAGS: -D_WINDOWS -DWINDOWS=1 -DZEND_WIN32=1 -DPHP_WIN32=1 -DWIN32 -D_MBCS -D_USE_MATH_DEFINES -DNDebug -DNDEBUG -DZEND_DEBUG=0 -DZTS=1 -DFD_SETSIZE=256
// #cgo windows LDFLAGS: -lpthreadVC3
import "C"
