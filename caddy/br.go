//go:build !nobrotli

package caddy

import (
	"runtime/debug"

	"github.com/dunglas/frankenphp"
)

var brotli = true

func init() {
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range buildInfo.Deps {
			if dep.Path == "github.com/dunglas/caddy-cbrotli" {
				frankenphp.AddPHPInfoEntry("dunglas/caddy-cbrotli", dep.Version)
				break
			}
		}
	}
}
