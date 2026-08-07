package frankenphp_test

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
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
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("unshare is not available")
	}

	probe := exec.Command("unshare", "--user", "--map-root-user", "--pid", "--fork", "true")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Skipf("unprivileged PID namespaces are not available: %s", output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		"unshare",
		"--user",
		"--map-root-user",
		"--pid",
		"--fork",
		"--kill-child=KILL",
		"internal/testcli/testcli",
		"--segfault",
	)
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

func ExampleExecuteScriptCLI() {
	if len(os.Args) <= 1 {
		log.Println("Usage: my-program script.php")
		os.Exit(1)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[1], os.Args))
}
