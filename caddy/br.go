//go:build !nobrotli

package caddy

// #include <brotli/encode.h>
import "C"

import (
	"fmt"
	"runtime/debug"

	"github.com/dunglas/frankenphp"
)

var brotli = true

func init() {
	brotliVer := C.BrotliEncoderVersion()
	if brotliVer != 0 {
		major := int(brotliVer >> 24)
		minor := int((brotliVer >> 12) & 0xfff)
		patch := int(brotliVer & 0xfff)
		frankenphp.AddPhpinfoEntry("libbrotli", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range buildInfo.Deps {
			if dep.Path == "github.com/dunglas/caddy-cbrotli" {
				frankenphp.AddPhpinfoEntry("dunglas/caddy-cbrotli", dep.Version)
				break
			}
		}
	}
}
