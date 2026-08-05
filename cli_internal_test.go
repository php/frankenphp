package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit coverage for the guard added for
// https://github.com/php/frankenphp/issues/2342: the embedded PHP CLI SAPI must
// not be started while FrankenPHP is already running, otherwise the process
// crashes with a segmentation fault.
func TestRefuseCLIWhileRunning(t *testing.T) {
	// When FrankenPHP is not running, the CLI helpers proceed normally.
	assert.False(t, refuseCLIWhileRunning())

	require.NoError(t, Init())
	defer Shutdown()

	// While running, the guard trips and the public entrypoints refuse cleanly
	// with the usage exit code instead of crashing the process.
	assert.True(t, refuseCLIWhileRunning())
	assert.Equal(t, cliUsageErrorExitCode, ExecuteScriptCLI("testdata/command.php", []string{"testdata/command.php"}))
	assert.Equal(t, cliUsageErrorExitCode, ExecutePHPCode("echo 'noop';"))
}
