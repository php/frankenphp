//go:build linux

package caddy

import "C"
import caddycmd "github.com/caddyserver/caddy/v2/cmd"

//export go_frankenphp_caddy_main
func go_frankenphp_caddy_main() {
	caddycmd.Main()
}
