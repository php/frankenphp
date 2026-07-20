// Package caddy provides a PHP module for the Caddy web server.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"fmt"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
)

const (
	defaultDocumentRoot = "public"
	defaultWatchPattern = "./**/*.{env,php,twig,yaml,yml}"

	// defaultRequestBodyTimeout is the idle timeout applied to request body
	// reads when the directive is omitted; mirrors nginx's client_body_timeout.
	// Set request_body_timeout to 0 to disable.
	defaultRequestBodyTimeout = caddy.Duration(60 * time.Second)
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

// wrongSubDirectiveError returns a nice error message.
func wrongSubDirectiveError(module string, allowedDirectives string, wrongValue string) error {
	return fmt.Errorf("unknown %q subdirective: %s (allowed directives are: %s)", module, wrongValue, allowedDirectives)
}
