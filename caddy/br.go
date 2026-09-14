//go:build !nobrotli

package caddy

import "github.com/dunglas/frankenphp"

var brotli = true

func init() {
	frankenphp.AddPHPInfoModule("dunglas/caddy-cbrotli", "github.com/dunglas/caddy-cbrotli")
}
