//go:build include_php_cli

package caddy

import (
	_ "embed"
	"errors"
	"fmt"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/spf13/cobra"
	"os"
)

//go:embed frankenphp/php-cli
var phpcli []byte

func init() {
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "install-php",
		Usage: "location",
		Short: "Installs the embedded PHP binary to the desired location",
		Long: `
FrankenPHP was embedded with a php-cli binary. You can extract this to the desired location.`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.DisableFlagParsing = true
			cmd.RunE = caddycmd.WrapCommandFuncForCobra(extractPHP)
		},
	})
}

func extractPHP(fs caddycmd.Flags) (int, error) {
	args := os.Args[2:]
	if len(args) < 1 {
		return 1, errors.New("the location is required")
	}

	destPath := args[0]
	
	err := os.WriteFile(destPath, phpcli, 0777)
	if err != nil {
		err = fmt.Errorf("Failed to write php-cli to %s: %v\n", destPath, err)
		if err != nil {
			panic(err)
		}
		os.Exit(1)
		return 1, nil
	}

	os.Exit(0)

	return 0, nil
}
