package caddy

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dunglas/frankenphp/internal/extgen"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/spf13/cobra"
)

func init() {
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "extension-init",
		Usage: "go_extension.go [--verbose]",
		Short: "Initializes a PHP extension from a Go file (EXPERIMENTAL)",
		Long: `
Initializes a PHP extension from a Go file. This command generates the necessary C files for the extension, including the header and source files, as well as the arginfo file.`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.Flags().BoolP("debug", "v", false, "Enable verbose debug logs")

			cmd.RunE = caddycmd.WrapCommandFuncForCobra(cmdInitExtension)
		},
	})
}

func cmdInitExtension(_ caddycmd.Flags) (int, error) {
	if len(os.Args) < 3 {
		return 1, errors.New("the path to the Go source is required")
	}

	sourceFile := os.Args[2]
	baseName := extgen.SanitizePackageName(strings.TrimSuffix(filepath.Base(sourceFile), ".go"))

	generator := extgen.Generator{BaseName: baseName, SourceFile: sourceFile, BuildDir: filepath.Dir(sourceFile)}

	if err := generator.Generate(); err != nil {
		return 1, err
	}

	log.Printf("PHP extension %q initialized successfully in directory %q", baseName, generator.BuildDir)

	return 0, nil
}
