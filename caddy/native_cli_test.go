//go:build linux

package caddy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func nativeBinary(t *testing.T) string {
	t.Helper()
	path := os.Getenv("FRANKENPHP_NATIVE_TEST_BINARY")
	if path == "" {
		t.Skip("build internal/nativetest with build-native.sh and set FRANKENPHP_NATIVE_TEST_BINARY")
	}
	return path
}

func nativeCommand(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	return exec.CommandContext(ctx, nativeBinary(t), args...)
}

func TestNativeCLI(t *testing.T) {
	t.Run("GoExtensionAndArguments", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "script with spaces.php")
		require.NoError(t, os.WriteFile(path, []byte(`<?php
echo frankenphp_native_test(), "\n";
echo json_encode(['version' => PHP_VERSION_ID, 'argv' => $argv]), "\n";
exit(7);
`), 0o600))
		cmd := nativeCommand(t, "php-cli", path, "two words", "--literal")
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit, "%s", out)
		require.Equal(t, 7, exit.ExitCode(), "%s", out)
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		require.Len(t, lines, 2)
		require.Equal(t, "42", lines[0])
		var result struct {
			Version int      `json:"version"`
			Args    []string `json:"argv"`
		}
		require.NoError(t, json.Unmarshal([]byte(lines[1]), &result))
		want := []string{path, "two words", "--literal"}
		if result.Version < 80600 {
			// The older CLI emulation includes the executable in $argv.
			want = append([]string{cmd.Args[0]}, want...)
		}
		require.Equal(t, want, result.Args)
	})

	t.Run("Eval", func(t *testing.T) {
		out, err := nativeCommand(t, "php-cli", "-r", `echo frankenphp_native_test();`).CombinedOutput()
		require.NoError(t, err, "%s", out)
		require.Equal(t, "42", string(out))
	})

	t.Run("EmbeddedScript", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "embedded.php"), []byte(`<?php echo frankenphp_native_test();`), 0o600))
		cmd := nativeCommand(t, "php-cli", "embedded.php")
		cmd.Env = append(os.Environ(), "FRANKENPHP_TEST_EMBEDDED_PATH="+dir)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		require.Equal(t, "42", string(out))
	})

	t.Run("CaddyCommands", func(t *testing.T) {
		out, err := nativeCommand(t, "list-modules").CombinedOutput()
		require.NoError(t, err, "%s", out)
		require.Contains(t, string(out), "http.handlers.php")
	})
}

func TestNativeCLISignals(t *testing.T) {
	for _, async := range []bool{true, false} {
		for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGALRM} {
			t.Run(fmt.Sprintf("async=%t/%s", async, sig), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "signals.php")
				code := fmt.Sprintf(`<?php
if (!extension_loaded('pcntl')) { echo "skip\n"; exit(0); }
pcntl_async_signals(%t);
$received = 0;
pcntl_signal(%d, function ($signal) use (&$received) {
    echo "handled:$signal\n";
    if (++$received === 3) { exit(0); }
});
echo frankenphp_native_test() === 42 ? "ready\n" : "failed\n";
while (true) {
    frankenphp_native_test();
    usleep(1000);
    %s
}
`, async, sig, map[bool]string{false: "pcntl_signal_dispatch();", true: ""}[async])
				require.NoError(t, os.WriteFile(path, []byte(code), 0o600))
				cmd := nativeCommand(t, "php-cli", path)
				lines, wait := nativeStart(t, cmd)
				ready := <-lines
				if ready == "skip" {
					require.NoError(t, wait())
					t.Skip("pcntl is not loaded")
				}
				require.Equal(t, "ready", ready)
				assertNativeSignalMasks(t, cmd.Process.Pid)
				for range 3 {
					require.NoError(t, cmd.Process.Signal(sig))
					require.Equal(t, fmt.Sprintf("handled:%d", sig), <-lines)
				}
				require.NoError(t, wait())
			})
		}
	}
}

func nativeStart(t *testing.T, cmd *exec.Cmd) (<-chan string, func() error) {
	t.Helper()
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	lines := make(chan string, 8)
	done := make(chan struct{})
	var waitErr error
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
		waitErr = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
		if stderr.Len() != 0 {
			t.Log(stderr.String())
		}
	})
	return lines, func() error {
		<-done
		return waitErr
	}
}

func assertNativeSignalMasks(t *testing.T, pid int) {
	t.Helper()
	statuses, err := filepath.Glob(fmt.Sprintf("/proc/%d/task/*/status", pid))
	require.NoError(t, err)
	require.Greater(t, len(statuses), 1, "Go must be running alongside PHP")
	for _, path := range statuses {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue // A Go thread can exit while /proc is being read.
		}
		require.NoError(t, err)
		for line := range strings.SplitSeq(string(data), "\n") {
			if !strings.HasPrefix(line, "SigBlk:") {
				continue
			}
			mask, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "SigBlk:")), 16, 64)
			require.NoError(t, err)
			isPHPThread := filepath.Base(filepath.Dir(path)) == strconv.Itoa(pid)
			for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGALRM} {
				blocked := mask&(uint64(1)<<(sig-1)) != 0
				if !isPHPThread {
					require.True(t, blocked, "%s must block %s", path, sig)
				}
			}
		}
	}
}

func TestNativeCLIDefaultSignal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.php")
	require.NoError(t, os.WriteFile(path, []byte(`<?php echo "ready\n"; while (true) usleep(10000);`), 0o600))
	cmd := nativeCommand(t, "php-cli", path)
	lines, wait := nativeStart(t, cmd)
	require.Equal(t, "ready", <-lines)
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	var exit *exec.ExitError
	require.ErrorAs(t, wait(), &exit)
	status := exit.Sys().(syscall.WaitStatus)
	require.True(t, status.Signaled())
	require.Equal(t, syscall.SIGTERM, status.Signal())
}

func TestNativeServerShutdown(t *testing.T) {
	binary := nativeBinary(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.php"), []byte(`<?php echo frankenphp_native_test();`), 0o600))
	config := fmt.Sprintf(`{
    admin off
    auto_https off
    frankenphp {
        num_threads 2
    }
}
http://%s {
    root * %s
    php_server
}
`, address, dir)
	configPath := filepath.Join(dir, "Caddyfile")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--config", configPath, "--adapter", "caddyfile")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(dir, "config"), "XDG_DATA_HOME="+filepath.Join(dir, "data"))
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
		if t.Failed() {
			t.Log(output.String())
		}
	})
	client := &http.Client{Timeout: time.Second}
	require.Eventually(t, func() bool {
		response, err := client.Get("http://" + address)
		if err != nil {
			return false
		}
		defer response.Body.Close()
		var body bytes.Buffer
		_, err = body.ReadFrom(response.Body)
		return err == nil && response.StatusCode == http.StatusOK && body.String() == "42"
	}, 10*time.Second, 50*time.Millisecond)
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	<-done
	require.NoError(t, waitErr, "%s", output.String())
}
