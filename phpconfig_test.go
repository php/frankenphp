package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPHPConfig(t *testing.T) {
	supported := PHPConfig{Version: PHPVersion{MajorVersion: 8, MinorVersion: 2}, ZTS: true}

	t.Run("a supported build is accepted", func(t *testing.T) {
		require.NoError(t, checkPHPConfig(supported))
	})

	t.Run("PHP older than 8.2 is rejected", func(t *testing.T) {
		old := supported
		old.Version.MinorVersion = 1
		assert.ErrorIs(t, checkPHPConfig(old), ErrInvalidPHPVersion)
	})

	t.Run("Zend signals are rejected in ZTS", func(t *testing.T) {
		signals := supported
		signals.ZendSignals = true
		assert.ErrorIs(t, checkPHPConfig(signals), ErrZendSignals)
	})

	t.Run("Zend signals are allowed without ZTS", func(t *testing.T) {
		// without ZTS the ini entries address the globals directly
		signals := supported
		signals.ZTS = false
		signals.ZendSignals = true
		require.NoError(t, checkPHPConfig(signals))
	})
}
