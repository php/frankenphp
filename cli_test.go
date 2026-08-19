package frankenphp_test

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func TestExecuteScriptCLI(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "testdata/command.php", "foo", "bar")
	stdoutStderr, err := cmd.CombinedOutput()
	assert.Error(t, err)

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
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

// `-i` (and any other invocation without a script) is only supported since PHP
// 8.6, where the real CLI SAPI is reused. older versions must fail cleanly.
func TestExecuteCLIPHPInfo(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "-i")
	stdoutStderr, err := cmd.CombinedOutput()
	stdoutStderrStr := string(stdoutStderr)

	if frankenphp.Version().VersionID < 80600 {
		assert.Error(t, err)

		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
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

// Regression test for https://github.com/php/frankenphp/issues/2558. When a
// C-created thread segfaults while FrankenPHP is PID 1, the process must
// exit instead of looping forever in Go's runtime.raisebadsignal.
func TestCThreadSegfaultAsPID1(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PID namespaces are only available on Linux")
	}
	useSudo := os.Getenv("FRANKENPHP_TEST_PID_NAMESPACE_WITH_SUDO") == "1"
	testCLIPath, err := filepath.Abs("internal/testcli/testcli")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(testCLIPath); err != nil {
		if useSudo {
			t.Fatalf("internal/testcli/testcli must be compiled for the PID namespace test: %v", err)
		}
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}
	unsharePath, err := exec.LookPath("unshare")
	if err != nil {
		if useSudo {
			t.Fatal("unshare is required for the PID namespace test")
		}
		t.Skip("unshare is not available")
	}
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Fatal("true is required for the PID namespace test probe")
	}

	command := unsharePath
	args := []string{
		"--user",
		"--map-root-user",
		"--pid",
		"--fork",
		"--kill-child=KILL",
		testCLIPath,
		"--segfault",
	}
	rootlessOutput, rootlessErr := runPIDNamespaceProbe(
		unsharePath,
		"--user",
		"--map-root-user",
		"--pid",
		"--fork",
		"--kill-child=KILL",
		truePath,
	)
	if rootlessErr != nil {
		if !useSudo {
			t.Skipf("unprivileged PID namespaces are not available: %v: %s", rootlessErr, rootlessOutput)
		}

		sudoPath, sudoErr := exec.LookPath("sudo")
		if sudoErr != nil {
			t.Fatal("sudo was requested for the PID namespace test but was not found")
		}
		sudoOutput, sudoErr := runPIDNamespaceProbe(
			sudoPath,
			"-n",
			unsharePath,
			"--pid",
			"--fork",
			"--kill-child=KILL",
			truePath,
		)
		if sudoErr != nil {
			t.Fatalf("failed to create a PID namespace with sudo: %v: %s", sudoErr, sudoOutput)
		}
		command = sudoPath
		args = []string{
			"-n",
			unsharePath,
			"--pid",
			"--fork",
			"--kill-child=KILL",
			testCLIPath,
			"--segfault",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("FrankenPHP did not exit after the PHP thread segfaulted: %s", output)
	}

	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("expected FrankenPHP to exit with an error, got %v: %s", err, output)
	}
	assert.Equal(t, 2, exitError.ExitCode(), "output: %s", output)
}

func runPIDNamespaceProbe(command string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if ctx.Err() != nil {
		return output, ctx.Err()
	}
	return output, err
}

func ExampleExecuteScriptCLI() {
	if len(os.Args) <= 1 {
		log.Println("Usage: my-program script.php")
		os.Exit(1)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[0], os.Args))
}
