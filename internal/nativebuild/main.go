// nativebuild builds a Caddy main package as a Go archive and links the native
// FrankenPHP entry point. Arguments are the usual go build flags and package.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed entrypoint/main.c
var entrypoint []byte

//go:embed entrypoint/init.ld
var initScript []byte

func main() {
	if err := build(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "nativebuild:", err)
		os.Exit(1)
	}
}

func build(args []string) error {
	output := "frankenphp"
	var buildArgs, externalFlags []string
	for i := 0; i < len(args); i++ {
		flag, value, hasValue := strings.Cut(args[i], "=")
		if flag == "-o" || flag == "-buildmode" || flag == "-ldflags" {
			if !hasValue {
				i++
				if i == len(args) {
					return fmt.Errorf("%s requires a value", flag)
				}
				value = args[i]
			}
		}
		switch {
		case flag == "-o":
			output = value
		case flag == "-buildmode":
			if value != "pie" && value != "exe" && value != "default" {
				return fmt.Errorf("unsupported native build mode: %s", value)
			}
		case flag == "-ldflags":
			flags, err := linkerFlags(value)
			if err != nil {
				return err
			}
			externalFlags = flags
			buildArgs = append(buildArgs, "-ldflags", value)
		default:
			buildArgs = append(buildArgs, args[i])
		}
	}

	goCommand := os.Getenv("FRANKENPHP_GO")
	if goCommand == "" {
		goCommand = "go"
	}
	envJSON, err := exec.Command(goCommand, "env", "-json", "CC", "GOGCCFLAGS", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_LDFLAGS", "GOOS", "PKG_CONFIG").Output()
	if err != nil {
		return err
	}
	var env map[string]string
	if err := json.Unmarshal(envJSON, &env); err != nil {
		return err
	}
	if env["GOOS"] != "linux" {
		return errors.New("the native launcher currently requires Linux")
	}
	cc, err := splitFlags(env["CC"])
	if err != nil || len(cc) == 0 {
		return fmt.Errorf("invalid CC: %q", env["CC"])
	}
	ccFlags, err := compilerFlags(cc, env["GOGCCFLAGS"])
	if err != nil {
		return err
	}
	for _, name := range []string{"CGO_CPPFLAGS", "CGO_CFLAGS"} {
		flags, err := splitFlags(env[name])
		if err != nil {
			return err
		}
		ccFlags = append(ccFlags, flags...)
	}
	envLDFlags, err := splitFlags(env["CGO_LDFLAGS"])
	if err != nil {
		return err
	}

	list := exec.Command(goCommand, append([]string{"list", "-deps", "-json"}, buildArgs...)...)
	list.Stderr = os.Stderr
	packages, err := list.Output()
	if err != nil {
		return err
	}
	var ldflags, pkgConfigs []string
	decoder := json.NewDecoder(bytes.NewReader(packages))
	for {
		var pkg struct {
			CgoLDFLAGS   []string
			CgoPkgConfig []string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		ldflags = append(ldflags, pkg.CgoLDFLAGS...)
		pkgConfigs = append(pkgConfigs, pkg.CgoPkgConfig...)
	}
	if len(pkgConfigs) > 0 {
		flags, err := exec.Command(env["PKG_CONFIG"], append([]string{"--libs"}, pkgConfigs...)...).Output()
		if err != nil {
			return err
		}
		parsed, err := splitFlags(string(flags))
		if err != nil {
			return err
		}
		ldflags = append(ldflags, parsed...)
	}
	ldflags = append(ldflags, envLDFlags...)

	dir, err := os.MkdirTemp("", "frankenphp-native-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	archive := filepath.Join(dir, "frankenphp.a")
	if err := run(goCommand, append([]string{"build", "-buildmode=c-archive", "-o", archive}, buildArgs...)...); err != nil {
		return err
	}
	source := filepath.Join(dir, "main.c")
	if err := os.WriteFile(source, entrypoint, 0o600); err != nil {
		return err
	}
	script := filepath.Join(dir, "init.ld")
	if err := os.WriteFile(script, initScript, 0o600); err != nil {
		return err
	}
	linkArgs := append(cc[1:], ccFlags...)
	// Go's archive is position independent. Keep the final executable PIE, as
	// in the existing static build; -static-pie in the linker flags overrides it.
	linkArgs = append(linkArgs, "-pie", "-Wl,-T,"+script, "-o", output, source, archive)
	linkArgs = append(linkArgs, ldflags...)
	linkArgs = append(linkArgs, externalFlags...)
	return run(cc[0], linkArgs...)
}

func compilerFlags(cc []string, value string) ([]string, error) {
	flags, err := splitFlags(value)
	if err != nil {
		return nil, err
	}
	// go env removes three arguments from its "CC -I ." probe. With a
	// multiword CC, its remaining arguments can leak into GOGCCFLAGS.
	probe := append(slices.Clone(cc), "-I", ".")[3:]
	if len(probe) > 0 && len(flags) >= len(probe) && slices.Equal(flags[:len(probe)], probe) {
		flags = flags[len(probe):]
	}
	return flags, nil
}

// Go does not invoke the external linker in c-archive mode. Forward its flags
// ourselves, preserving static PHP symbol exports, stack size and link options.
func linkerFlags(value string) ([]string, error) {
	flags, err := splitFlags(strings.TrimPrefix(value, "all="))
	if err != nil {
		return nil, err
	}
	var external, strip []string
	for i := 0; i < len(flags); i++ {
		flag, value, hasValue := strings.Cut(flags[i], "=")
		if !hasValue || value == "true" {
			if flag == "-s" {
				strip = append(strip, "-s")
			} else if flag == "-w" {
				strip = append(strip, "-Wl,--strip-debug")
			}
		}
		if flag != "-extldflags" {
			continue
		}
		if !hasValue {
			i++
			if i == len(flags) {
				return nil, errors.New("-extldflags requires a value")
			}
			value = flags[i]
		}
		external, err = splitFlags(value)
		if err != nil {
			return nil, err
		}
	}
	return append(external, strip...), nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd.String(), err)
	}
	return nil
}

// splitFlags accepts the quoting used by CC, CGO_LDFLAGS and pkg-config without
// evaluating shell expansions or running a shell.
func splitFlags(s string) ([]string, error) {
	var args []string
	var arg strings.Builder
	var quote rune
	escaped, started := false, false
	for _, r := range s {
		switch {
		case escaped:
			arg.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, started = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				arg.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, started = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if started {
				args = append(args, arg.String())
				arg.Reset()
				started = false
			}
		default:
			arg.WriteRune(r)
			started = true
		}
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated quoting in flags: %q", s)
	}
	if started {
		args = append(args, arg.String())
	}
	return args, nil
}
