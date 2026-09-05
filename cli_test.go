package frankenphp_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteScriptCLI(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "testdata/command.php", "foo", "bar")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.Error(t, err)

	if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
		assert.Equal(t, 3, exitError.ExitCode())
	}

	stdoutStderrStr := string(stdoutStderr)

	assert.Contains(t, stdoutStderrStr, `"foo"`)
	assert.Contains(t, stdoutStderrStr, `"bar"`)
	assert.Contains(t, stdoutStderrStr, "From the CLI")
}

func TestExecuteCLICode(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "-r", "echo 'Hello World';")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.NoError(t, err)

	stdoutStderrStr := string(stdoutStderr)
	assert.Equal(t, stdoutStderrStr, `Hello World`)
}

// The CLI must print phpinfo() as plain text, like the CLI SAPI does.
func TestExecuteCLICodePHPInfoAsText(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "-r", "phpinfo();")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.NoError(t, err)

	stdoutStderrStr := string(stdoutStderr)

	assert.Contains(t, stdoutStderrStr, "PHP Version => ")
	assert.Contains(t, stdoutStderrStr, "frankenphp => ")
	assert.Contains(t, stdoutStderrStr, "go => go")
	assert.Contains(t, stdoutStderrStr, "Go modules")
	assert.Contains(t, stdoutStderrStr, "Module => Version")
	assert.NotContains(t, stdoutStderrStr, "<!DOCTYPE")
	assert.NotContains(t, stdoutStderrStr, "<table>")
	assert.NotContains(t, stdoutStderrStr, "<details>")
}

// `-i` (and any other invocation without a script) is only supported since PHP
// 8.6, where the real CLI SAPI is reused. Older versions must fail cleanly.
func TestExecuteCLIPHPInfo(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "-i")
	stdoutStderr, err := cmd.CombinedOutput()
	stdoutStderrStr := string(stdoutStderr)

	if frankenphp.Version().VersionID < 80600 {
		assert.Error(t, err)

		if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
			assert.Equal(t, 1, exitError.ExitCode())
		}

		assert.Contains(t, stdoutStderrStr, "this functionality is not available in frankenphp php-cli")

		return
	}

	assert.NoError(t, err, "output: %s", stdoutStderrStr)
	assert.Contains(t, stdoutStderrStr, "PHP Version => "+frankenphp.Version().Version)
}

// Regression test for https://github.com/php/frankenphp/issues/1902. A
// long-running CLI script that installs pcntl_signal handlers must
// receive its own signals reliably
func TestExecuteScriptCLISignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pcntl is not available on Windows")
	}
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "testdata/command-pcntl.php")
	stdoutStderr, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
		t.Skipf("pcntl/posix not available: %s", stdoutStderr)
	}
	assert.NoError(t, err, "output: %s", stdoutStderr)
	assert.Contains(t, string(stdoutStderr), "ok")
}

func TestExecuteCLIEnvironment(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	t.Setenv("FRANKENPHP_CLI_ENVIRONMENT_TEST", "inherited")
	for _, tt := range []struct {
		name string
		code string
		want string
	}{
		{
			name: "getenv named",
			code: `echo json_encode([getenv($name), getenv($name, true)]);`,
			want: `["inherited","inherited"]`,
		},
		{
			name: "getenv all",
			code: `
$env = getenv();
$localEnv = getenv(null, true);
echo json_encode([is_array($env), $env[$name], is_array($localEnv), $localEnv[$name]]);`,
			want: `[true,"inherited",true,"inherited"]`,
		},
		{
			name: "putenv",
			code: `
$results = [putenv($name . "=changed=value"), getenv($name), getenv($name, true), getenv()[$name]];
$results[] = putenv($name . "=");
$results[] = getenv($name);
$results[] = array_key_exists($name, getenv());
$results[] = putenv($name);
$results[] = getenv($name);
$results[] = getenv($name, true);
$results[] = array_key_exists($name, getenv());
echo json_encode($results);`,
			want: `[true,"changed=value","changed=value","changed=value",true,"",true,true,false,false,false]`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "internal/testcli/testcli", "-n", "-r", `$name = "FRANKENPHP_CLI_ENVIRONMENT_TEST"; `+tt.code)
			cmd.WaitDelay = time.Second
			output, err := cmd.CombinedOutput()
			require.NoError(t, ctx.Err(), "CLI timed out; output: %s", output)
			require.NoError(t, err, "output: %s", output)
			require.Equal(t, tt.want, string(output))
		})
	}
}

func TestExecuteCLIHTTPFunctionsUnavailable(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	for _, tt := range []struct {
		name string
		args string
	}{
		{"getallheaders", ""},
		{"apache_request_headers", ""},
		{"fastcgi_finish_request", ""},
		{"frankenphp_request_headers", ""},
		{"frankenphp_response_headers", ""},
		{"apache_response_headers", ""},
		{"frankenphp_finish_request", ""},
		{"frankenphp_handle_request", "static function () {}"},
		{"headers_send", "103"},
		{"mercure_publish", "'https://example.com/topic', 'test'"},
		{"frankenphp_log", "'CLI feature detection'"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// Frameworks call these after feature detection. Exposing HTTP
			// callbacks in CLI can access missing Go threads or a foreign SAPI context.
			code := fmt.Sprintf(`
$function = %q;
if (function_exists($function)) {
    $function(%s);
}
var_export(function_exists($function));
`, tt.name, tt.args)
			cmd := exec.CommandContext(ctx, "internal/testcli/testcli", "-r", code)
			cmd.WaitDelay = time.Second
			output, err := cmd.CombinedOutput()
			require.NoError(t, ctx.Err(), "output: %s", output)
			require.NoError(t, err, "output: %s", output)
			require.Equal(t, "false", string(output))
		})
	}
}

func TestExecuteCLINativeHTTPFunctions(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "internal/testcli/testcli", "-r", `
if (PHP_SAPI !== 'cli') {
    throw new RuntimeException('Expected ordinary CLI');
}
foreach (['header', 'header_remove', 'headers_list', 'headers_sent',
          'http_response_code', 'flush', 'connection_status',
          'connection_aborted', 'ignore_user_abort'] as $function) {
    if (!function_exists($function)) {
        throw new RuntimeException('Missing native function: ' . $function);
    }
}
header('X-CLI-Test: test');
header_remove('X-CLI-Test');
headers_list();
headers_sent();
http_response_code(204);
flush();
connection_status();
connection_aborted();
ignore_user_abort(false);
// Older PHP versions use the embed SAPI rather than the native CLI SAPI.
if (PHP_VERSION_ID >= 80600) {
    foreach (['dl', 'cli_set_process_title', 'cli_get_process_title'] as $function) {
        if (!function_exists($function)) {
            throw new RuntimeException('Missing native CLI function: ' . $function);
        }
    }
}
echo 'ok';
`)
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "output: %s", output)
	require.NoError(t, err, "output: %s", output)
	require.Equal(t, "ok", string(output))
}

func TestExecuteScriptCLILifecycle(t *testing.T) {
	const childEnv = "FRANKENPHP_CLI_LIFECYCLE_CHILD"
	if scenario := os.Getenv(childEnv); scenario != "" {
		calls := 1
		switch scenario {
		case "repeated":
			calls = 2
		case "rejected-then-script":
			// Missing -r code is rejected before PHP startup by the pre-8.6
			// emulation, but still installs the module registration hook.
			args := []string{"cli-lifecycle", "-n", "-r"}
			if status := frankenphp.ExecuteScriptCLI(args[0], args); status == 0 {
				t.Fatal("CLI accepted -r without code")
			}
		default:
			t.Fatalf("unknown CLI lifecycle scenario %q", scenario)
		}

		for i := 1; i <= calls; i++ {
			code := fmt.Sprintf("file_put_contents('cli-lifecycle-script-%d', 'executed'); exit(%d);", i, 20+i)
			args := []string{"cli-lifecycle", "-n", "-r", code}
			if status := frankenphp.ExecuteScriptCLI(args[0], args); status != 20+i {
				t.Fatalf("CLI call %d returned %d, want %d", i, status, 20+i)
			}
		}
		// Older CLI emulation closes standard streams at shutdown. Use the
		// process exit status rather than the test runner's final output.
		os.Exit(0)
	}

	for _, scenario := range []string{"repeated", "rejected-then-script"} {
		t.Run(scenario, func(t *testing.T) {
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			// Keep PHP's process-global CLI lifecycle out of server tests, and
			// bound both a recursive-hook crash and a hung child.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, self, "-test.run=^TestExecuteScriptCLILifecycle$")
			cmd.Env = append(os.Environ(), childEnv+"="+scenario)
			// File markers survive pre-8.6 CLI shutdown closing standard streams.
			cmd.Dir = t.TempDir()
			cmd.WaitDelay = time.Second
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("CLI lifecycle child failed: %v (context: %v)\n%s", err, ctx.Err(), output)
			}
			calls := 1
			if scenario == "repeated" {
				calls = 2
			}
			for i := 1; i <= calls; i++ {
				marker := filepath.Join(cmd.Dir, fmt.Sprintf("cli-lifecycle-script-%d", i))
				if content, err := os.ReadFile(marker); err != nil || string(content) != "executed" {
					t.Fatalf("CLI lifecycle child did not execute script %d: marker %q, error %v\n%s", i, content, err, output)
				}
			}
		})
	}
}

func TestExecuteCLIOpcacheReset(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	for _, tt := range []struct {
		name      string
		enableCLI string
		code      string
		want      string
	}{
		{
			name:      "disabled",
			enableCLI: "0",
			code:      `echo json_encode([ini_get('opcache.enable_cli'), opcache_reset()]);`,
			want:      `["0",false]`,
		},
		{
			name:      "enabled",
			enableCLI: "1",
			// Native reset schedules a restart at request shutdown, not an
			// immediate cache flush. Inspecting the pending flag needs no fixture.
			code: `
$before = opcache_get_status(false);
$reset = opcache_reset();
$after = opcache_get_status(false);
echo json_encode([ini_get('opcache.enable_cli'), $before['opcache_enabled'],
    $before['restart_pending'], $reset, $after['restart_pending']]);`,
			want: `["1",true,false,true,true]`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// The emulated CLI (PHP < 8.6) does not parse -d or -n. Use an
			// isolated INI instead, including an empty scan directory.
			iniPath := filepath.Join(t.TempDir(), "php.ini")
			t.Setenv("PHPRC", iniPath)
			t.Setenv("PHP_INI_SCAN_DIR", t.TempDir())
			ini := "opcache.enable=1\nopcache.enable_cli=" + tt.enableCLI + "\n" +
				"opcache.file_cache_only=0\nopcache.restrict_api=\nopcache.jit=disable\n"
			code := `
if (!extension_loaded('Zend OPcache')) {
    fwrite(STDERR, "OPcache is not loaded\n");
    exit(77);
}
` + tt.code

			// PHP 8.5+ includes OPcache; older builds may link it statically
			// or provide a shared extension. Do not load a static extension twice.
			for _, shared := range []bool{false, true} {
				config := ini
				if shared {
					config += "zend_extension=opcache\n"
				}
				require.NoError(t, os.WriteFile(iniPath, []byte(config), 0o600))

				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				cmd := exec.CommandContext(ctx, "internal/testcli/testcli", "-r", code)
				cmd.WaitDelay = time.Second
				output, err := cmd.CombinedOutput()
				cancel()
				if exitError, ok := errors.AsType[*exec.ExitError](err); ok && exitError.ExitCode() == 77 {
					if shared {
						t.Skipf("OPcache is unavailable, including as a shared extension: %s", output)
					}
					continue
				}
				require.NoError(t, err, "output: %s", output)
				require.Equal(t, tt.want, string(output))
				return
			}
		})
	}
}

func ExampleExecuteScriptCLI() {
	if len(os.Args) <= 1 {
		log.Println("Usage: my-program script.php")
		os.Exit(1)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[0], os.Args))
}
