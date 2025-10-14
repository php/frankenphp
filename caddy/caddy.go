// Package caddy provides a PHP module for the Caddy web server.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"fmt"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
)

const (
	defaultDocumentRoot = "public"
	defaultWatchPattern = "./**/*.{php,yaml,yml,twig,env}"
)

func init() {
	caddy.RegisterModule(FrankenPHPApp{})
	caddy.RegisterModule(FrankenPHPModule{})
	caddy.RegisterModule(FrankenPHPAdmin{})

	httpcaddyfile.RegisterGlobalOption("frankenphp", parseGlobalOption)

	httpcaddyfile.RegisterHandlerDirective("php", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("php", "before", "file_server")

	httpcaddyfile.RegisterDirective("php_server", parsePhpServer)
	httpcaddyfile.RegisterDirectiveOrder("php_server", "before", "file_server")
}

// return a nice error message
func wrongSubDirectiveError(module string, allowedDriectives string, wrongValue string) error {
	return fmt.Errorf("unknown '%s' subdirective: '%s' (allowed directives are: %s)", module, wrongValue, allowedDriectives)
}
