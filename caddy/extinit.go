package caddy

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/dunglas/frankenphp/internal/extgen"
	"github.com/spf13/cobra"
)

func init() {
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "extension-init",
		Usage: "go_extension.go [--overwrite-readme]",
		Short: "Initializes a PHP extension from a Go file (EXPERIMENTAL)",
		Long: `
Initializes a PHP extension from a Go file. This command generates the necessary C files for the extension, including the header and source files, as well as the arginfo file.`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.Flags().BoolP("overwrite-readme", "r", false, "Overwrite README.md if it exists")

			cmd.RunE = cmdInitExtension
		},
	})
}

func cmdInitExtension(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("the path to the Go source is required")
	}

	overwriteReadme, err := cmd.Flags().GetBool("overwrite-readme")
	if err != nil {
		return err
	}

	sourceFile := args[0]
	baseName := extgen.SanitizePackageName(strings.TrimSuffix(filepath.Base(sourceFile), ".go"))

	generator := extgen.Generator{BaseName: baseName, SourceFile: sourceFile, BuildDir: filepath.Dir(sourceFile), OverwriteReadme: overwriteReadme}

	if err := generator.Generate(); err != nil {
		return err
	}

	log.Printf("PHP extension %q initialized successfully in directory %q", baseName, generator.BuildDir)

	return nil
}
