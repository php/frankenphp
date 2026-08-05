package frankenphp_test

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"runtime"
	"testing"

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

// Regression test for https://github.com/php/frankenphp/issues/2342. Calling
// ExecuteScriptCLI (or ExecutePHPCode) while FrankenPHP is already running in the
// same process used to boot the embedded PHP CLI SAPI on top of the running
// FrankenPHP SAPI, corrupting the engine and crashing the whole process with a
// segmentation fault. It must now be refused cleanly with a non-zero exit status.
func TestExecuteScriptCLIWhileRunning(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	cmd := exec.Command("internal/testcli/testcli", "-init-first", "testdata/command.php")
	stdoutStderr, err := cmd.CombinedOutput()

	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError, "output: %s", stdoutStderr)

	// The process must exit cleanly, not be killed by a signal (before the fix
	// it crashed with SIGSEGV, reported by ProcessState.Exited() == false).
	assert.True(t, exitError.ProcessState.Exited(),
		"process was killed by a signal instead of exiting cleanly: %s\noutput: %s", exitError, stdoutStderr)
	assert.Equal(t, 1, exitError.ExitCode(), "output: %s", stdoutStderr)
}

func ExampleExecuteScriptCLI() {
	if len(os.Args) <= 1 {
		log.Println("Usage: my-program script.php")
		os.Exit(1)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[1], os.Args))
}
