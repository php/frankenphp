//go:build nowatcher

package frankenphp_test

import (
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// Validate() must refuse what Init() refuses: a build without the watcher
// takes no worker asking for one, and finding that out after Start() shut
// the running configuration down is the outage Validate() prevents
func TestValidateRefusesWatchWithoutWatcher(t *testing.T) {
	err := frankenphp.Validate(
		frankenphp.WithWorkers("watched", "testdata/index.php", 1,
			frankenphp.WithWorkerWatchMode([]string{"./testdata"}),
		),
	)

	require.ErrorContains(t, err, "watcher support is not enabled")
}
