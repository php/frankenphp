package caddy

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/caddyserver/caddy/v2"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/dunglas/frankenphp"
	"go.uber.org/zap/exp/zapslog"

	"github.com/spf13/cobra"
)

func init() {
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "php-cli",
		Usage: "script.php [args ...]",
		Short: "Runs a PHP command",
		Long: `
Executes a PHP script similarly to the CLI SAPI.`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.DisableFlagParsing = true
			cmd.RunE = caddycmd.WrapCommandFuncForCobra(cmdPHPCLI)
		},
	})
}

func cmdPHPCLI(fs caddycmd.Flags) (int, error) {
	// php's cli sapi expects the 0th arg to be the program itself, only filter out 'php-cli' arg
	args := append([]string{os.Args[0]}, os.Args[2:]...)

	if len(args) > 1 {
		args[1] = resolveEmbeddedScript(args[1])
	}

	logger := slog.New(zapslog.NewHandler(caddy.Log().Core(), zapslog.WithName("frankenphp")))
	status := frankenphp.ExecuteScriptCLI(args[0], args, frankenphp.WithLogger(logger))

	os.Exit(status)

	return status, nil
}

// resolveEmbeddedScript resolves relative scripts missing from the working directory,
// including extension-less ones such as bin/console, against the embedded app.
func resolveEmbeddedScript(script string) string {
	if frankenphp.EmbeddedAppPath == "" || strings.HasPrefix(script, "-") || !filepath.IsLocal(script) {
		return script
	}
	if _, err := os.Stat(script); err == nil {
		return script
	}

	embedded := filepath.Join(frankenphp.EmbeddedAppPath, script)
	if _, err := os.Stat(embedded); err != nil {
		return script
	}

	return embedded
}
