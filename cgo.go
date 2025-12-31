package frankenphp

// #cgo darwin pkg-config: libxml-2.0
// #cgo CFLAGS: -Wall -Werror
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo LDFLAGS: -lphp -lm -lutil
// #cgo linux LDFLAGS: -ldl -lresolv
// #cgo darwin LDFLAGS: -Wl,-rpath,/usr/local/lib -liconv -ldl
import "C"
