// Package caddy provides a PHP module for the Caddy web server.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/dunglas/frankenphp"
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
	// Keep PHP available until every Caddy app has finished stopping, including
	// the HTTP server's shutdown delay and in-flight request draining.
	caddy.OnExit(func(context.Context) {
		frankenphp.Shutdown()
	})

	caddy.RegisterModule(&FrankenPHPApp{})
	caddy.RegisterModule(&FrankenPHPModule{})
	caddy.RegisterModule(&FrankenPHPAdmin{})

	// Report Caddy version in phpinfo()
	simpleVersion, _ := caddy.Version()
	frankenphp.AddPHPInfoEntry("Caddy", simpleVersion)

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

// parseNonNegativeInt rejects negative values, which the runtime treats as sentinels (auto, default).
func parseNonNegativeInt(d *caddyfile.Dispenser) (int, error) {
	v, err := strconv.Atoi(d.Val())
	if err != nil {
		return 0, d.WrapErr(err)
	}
	if v < 0 {
		return 0, d.Errf("%q must not be negative", d.Val())
	}

	return v, nil
}
