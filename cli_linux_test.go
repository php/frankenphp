//go:build linux

package frankenphp_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestExecuteScriptCLIPhpInfoForkChild(t *testing.T) {
	if _, err := os.Stat("internal/testcli/testcli"); err != nil {
		t.Skip("internal/testcli/testcli has not been compiled, run `cd internal/testcli/ && go build`")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "internal/testcli/testcli", "testdata/command-phpinfo-fork.php")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid
	err := cmd.Wait()
	if err != nil {
		// kill the whole group: the parent may be stuck on a wedged child
		_ = unix.Kill(-pid, unix.SIGKILL)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("fork child did not finish: %s", output.String())
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
		t.Skipf("pcntl unavailable: %s", output.String())
	}
	require.NoError(t, err, "%s", output.String())
	require.Contains(t, output.String(), "parent-ok")
	require.Contains(t, output.String(), "child-safe",
		"a fork child must not call into Go to render FrankenPHP phpinfo data")
}

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

	// PDEATHSIG and subreapers are Linux-only: isolate them in a child process
	require.NoError(t, unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0))
	input, release, err := os.Pipe()
	require.NoError(t, err)
	defer func() { _ = input.Close() }()
	pid := 0
	t.Cleanup(func() {
		// EOF releases a child whose PID never got reported
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
	cmd.Env = append(os.Environ(), "FRANKENPHP_TEST_DETACHED_READY="+ready)
	cmd.Stdin = input
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
		// exit 2: nothing was forked, nothing to reap
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

	// the CLI joined its PHP thread before exiting: the child must survive the
	// forking thread's exit, not just the process's
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
			pid = 0 // don't signal a possibly reused PID
			require.True(t, status.Exited(), "detached child terminated by signal %d (%s)", status.Signal(), status.Signal())
			require.Equal(t, 0, status.ExitStatus(), "detached child failed")
			require.NoError(t, writeErr)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached child did not finish after CLI parent exited")
}
