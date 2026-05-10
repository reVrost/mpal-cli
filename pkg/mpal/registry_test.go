package mpal

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestBuiltInStrategiesValidateAgainstSchema(t *testing.T) {
	t.Parallel()

	schema := loadStrategySchema(t)
	matches, err := filepath.Glob(filepath.Join("..", "..", "strategies", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			doc := loadYAMLDocument(t, path)
			require.NoError(t, schema.Validate(doc))
		})
	}
}

func TestStrategySchemaRejectsInvalidStrategy(t *testing.T) {
	t.Parallel()

	schema := loadStrategySchema(t)
	doc := loadYAMLDocument(t, filepath.Join("..", "..", "strategies", "engine_weekly_swing_v1.yaml"))
	scoring := doc["scoring"].(map[string]any)
	scoring["min_buy_score"] = 1.5

	require.Error(t, schema.Validate(doc))
}

func TestBuiltInStrategiesUseDefaultsInsteadOfHiddenKnobs(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob(filepath.Join("..", "..", "strategies", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			doc := loadYAMLDocument(t, path)
			require.Contains(t, doc, "defaults")
			require.NotContains(t, doc, "event_guardrail")
			require.NotContains(t, doc, "backtest")
			require.NotContains(t, doc["portfolio"].(map[string]any), "long_only")
			require.NotContains(t, doc["risk"].(map[string]any), "protect_unscored_holdings")
		})
	}
}

func TestStrategyDefaultsExpandSlimBuiltIns(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob(filepath.Join("..", "..", "strategies", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			cfg, _, err := LoadStrategyFile(path)
			require.NoError(t, err)
			require.True(t, ValidateStrategyConfig(cfg).Valid)
			require.True(t, cfg.Portfolio.LongOnly)
			require.True(t, cfg.Risk.ProtectUnscoredHoldings)
			require.Equal(t, BacktestConfig{InitialCash: 100000, FeeBps: 5, SlippageBps: 10}, cfg.Backtest)

			switch cfg.Defaults {
			case StrategyDefaultsSwingV1:
				require.Equal(t, EventGuardrailConfig{
					Enabled:      true,
					LookbackDays: 14,
					VetoScore:    -0.55,
					BoostScore:   0.70,
					BoostAmount:  0.03,
				}, cfg.Events)
			case StrategyDefaultsBasicV1:
				require.False(t, cfg.Events.Enabled)
			default:
				t.Fatalf("unexpected defaults profile %q", cfg.Defaults)
			}
		})
	}
}

func TestLoadStrategyBytesPreservesExplicitAdvancedConfigWithoutDefaults(t *testing.T) {
	t.Parallel()

	cfg, _, err := LoadStrategyBytes([]byte(`
id: custom_full_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
event_guardrail:
  enabled: true
  lookback_days: 9
  event_veto_score: -0.4
  event_boost_score: 0.8
  event_boost_amount: 0.02
portfolio:
  long_only: false
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
  rebalance: weekly
risk:
  turnover_budget_pct: 0.3
  max_single_trade_pct: 0.2
  starter_position_pct: 0.02
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
  protect_unscored_holdings: false
backtest:
  initial_cash: 12345
  fee_bps: 1
  slippage_bps: 2
`))
	require.NoError(t, err)

	require.Empty(t, cfg.Defaults)
	require.False(t, cfg.Portfolio.LongOnly)
	require.False(t, cfg.Risk.ProtectUnscoredHoldings)
	require.Equal(t, EventGuardrailConfig{
		Enabled:      true,
		LookbackDays: 9,
		VetoScore:    -0.4,
		BoostScore:   0.8,
		BoostAmount:  0.02,
	}, cfg.Events)
	require.Equal(t, BacktestConfig{InitialCash: 12345, FeeBps: 1, SlippageBps: 2}, cfg.Backtest)
}

func loadStrategySchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "strategy.schema.json"))
	require.NoError(t, err)

	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(raw, &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	return resolved
}

func loadYAMLDocument(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	return doc
}
