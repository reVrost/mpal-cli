package mpal

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootStrategiesMatchEmbeddedCopies(t *testing.T) {
	t.Parallel()

	matches, err := fs.Glob(builtinStrategyFS, "strategies/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	for _, embeddedPath := range matches {
		embeddedPath := embeddedPath
		t.Run(filepath.Base(embeddedPath), func(t *testing.T) {
			t.Parallel()

			embeddedRaw, err := builtinStrategyFS.ReadFile(embeddedPath)
			require.NoError(t, err)

			rootPath := filepath.Join("..", "..", "strategies", filepath.Base(embeddedPath))
			rootRaw, err := os.ReadFile(rootPath)
			require.NoError(t, err)
			require.Equal(t, string(rootRaw), string(embeddedRaw))
		})
	}
}
