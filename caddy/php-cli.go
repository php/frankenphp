package caddy

import (
	"errors"
	"os"
	"path/filepath"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/dunglas/frankenphp"

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
	args := os.Args[2:]
	if len(args) < 1 {
		return 1, errors.New("the path to the PHP script is required")
	}

	if frankenphp.EmbeddedAppPath != "" {
		if _, err := os.Stat(args[0]); err != nil {
			args[0] = filepath.Join(frankenphp.EmbeddedAppPath, args[0])
		}
	}

	var status int
	if len(args) >= 2 && args[0] == "-r" {
		status = frankenphp.ExecutePHPCode(args[1])
	} else {
		status = frankenphp.ExecuteScriptCLI(args[0], args)
	}

	os.Exit(status)

	return status, nil
}
