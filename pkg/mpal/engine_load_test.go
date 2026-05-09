package mpal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadStrategyRunResultReadsPathAndJSON(t *testing.T) {
	t.Parallel()

	run := StrategyRunResult{
		RunID:           "run_20260508_1",
		AsOf:            time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		Result:          ResultTrade,
		ModelResult:     ResultTrade,
		ExecutionResult: ResultTrade,
	}
	raw, err := json.Marshal(run)
	require.NoError(t, err)

	fromJSON, err := LoadStrategyRunResult(string(raw))
	require.NoError(t, err)
	require.Equal(t, run.RunID, fromJSON.RunID)
	require.Equal(t, run.ExecutionResult, fromJSON.ExecutionResult)

	path := filepath.Join(t.TempDir(), "run.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	fromPath, err := LoadStrategyRunResult(path)
	require.NoError(t, err)
	require.Equal(t, run.RunID, fromPath.RunID)
	require.Equal(t, run.ExecutionResult, fromPath.ExecutionResult)
}
