//go:build linux

package frankenphp_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestExecuteScriptCLIDetachedChild(t *testing.T) {
	const helperEnv = "FRANKENPHP_TEST_DETACHED_CHILD"
	dir := os.Getenv(helperEnv)
	if dir == "" {
		if _, err := os.Stat("internal/testcli/testcli"); err != nil {
			t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
		}
		self, err := os.Executable()
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, self, "-test.run=^TestExecuteScriptCLIDetachedChild$", "-test.v")
		cmd.Env = append(os.Environ(), helperEnv+"="+t.TempDir())
		cmd.WaitDelay = time.Second
		output, err := cmd.CombinedOutput()
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 77 {
			t.Skipf("pcntl/posix unavailable: %s", output)
		}
		require.NoError(t, err, "%s", output)
		return
	}

	// PDEATHSIG and subreapers are Linux-specific. Isolate adoption from other tests.
	require.NoError(t, unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0))
	input, release, err := os.Pipe()
	require.NoError(t, err)
	defer func() { _ = input.Close() }()
	pid := 0
	t.Cleanup(func() {
		// EOF also releases a child whose PID was not reported before a parent failure.
		_ = release.Close()
		if pid > 0 {
			_ = unix.Kill(pid, unix.SIGKILL)
		}
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			var status unix.WaitStatus
			_, err := unix.Wait4(-1, &status, unix.WNOHANG, nil)
			if errors.Is(err, unix.ECHILD) {
				return
			}
			if err != nil && !errors.Is(err, unix.EINTR) {
				t.Errorf("reaping detached child: %v", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Error("detached child cleanup timed out")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ready := filepath.Join(dir, "ready")
	_, err = os.Lstat(ready)
	require.ErrorIs(t, err, os.ErrNotExist, "readiness path must not already exist")
	cmd := exec.CommandContext(ctx, "internal/testcli/testcli", "testdata/command-detached.php")
	// PHP's emulated and native CLIs expose different script argv layouts.
	cmd.Env = append(os.Environ(), "FRANKENPHP_TEST_DETACHED_READY="+ready)
	cmd.Stdin = input
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
		// The fixture checks extensions before forking, so nothing needs reaping.
		t.Logf("%s", output)
		os.Exit(77)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "CHILD=") {
			pid, _ = strconv.Atoi(strings.TrimPrefix(line, "CHILD="))
		}
	}
	require.NoError(t, err, "CLI parent: %s", output)
	require.Greater(t, pid, 0, "no child PID: %s", output)

	// CombinedOutput has waited for the actual CLI parent exit, not just readiness.
	// The CLI joins its PHP thread before exiting, so this also covers Linux's
	// PDEATHSIG on the forking thread's exit rather than the whole process's exit.
	_, writeErr := release.WriteString("survived\n")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var status unix.WaitStatus
		got, err := unix.Wait4(pid, &status, unix.WNOHANG, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		require.NoError(t, err)
		if got == pid {
			pid = 0 // Reaped: cleanup must not signal a potentially reused PID.
			require.True(t, status.Exited(), "detached child terminated by signal %d (%s)", status.Signal(), status.Signal())
			require.Equal(t, 0, status.ExitStatus(), "detached child failed")
			require.NoError(t, writeErr)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached child did not finish after CLI parent exited")
}
