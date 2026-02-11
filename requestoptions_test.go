package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRequestSplitPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		splitPath     []string
		wantErr       error
		wantSplitPath []string
	}{
		{
			name:          "valid lowercase split path",
			splitPath:     []string{".php"},
			wantErr:       nil,
			wantSplitPath: []string{".php"},
		},
		{
			name:          "valid uppercase split path normalized",
			splitPath:     []string{".PHP"},
			wantErr:       nil,
			wantSplitPath: []string{".php"},
		},
		{
			name:          "valid mixed case split path normalized",
			splitPath:     []string{".PhP", ".PHTML"},
			wantErr:       nil,
			wantSplitPath: []string{".php", ".phtml"},
		},
		{
			name:          "empty split path",
			splitPath:     []string{},
			wantErr:       nil,
			wantSplitPath: []string{},
		},
		{
			name:      "non-ASCII character in split path rejected",
			splitPath: []string{".php", ".Ⱥphp"},
			wantErr:   ErrInvalidSplitPath,
		},
		{
			name:      "unicode character in split path rejected",
			splitPath: []string{".phpⱥ"},
			wantErr:   ErrInvalidSplitPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &frankenPHPContext{}
			opt, err := WithRequestSplitPath(tt.splitPath)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.NoError(t, opt(ctx))
			assert.Equal(t, tt.wantSplitPath, ctx.splitPath)
		})
	}
}
