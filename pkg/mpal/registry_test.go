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

func TestMarketPalTraderSkillCopiesMatch(t *testing.T) {
	t.Parallel()

	rootRaw, err := os.ReadFile(filepath.Join("..", "..", "skills", "marketpal-trader", "SKILL.md"))
	require.NoError(t, err)

	agentRaw, err := os.ReadFile(filepath.Join("..", "..", ".agents", "skills", "marketpal-trader", "SKILL.md"))
	require.NoError(t, err)

	require.Equal(t, string(rootRaw), string(agentRaw))
}

func TestBestSwingStrategiesAreApprovedAPICompatibleDefaults(t *testing.T) {
	t.Parallel()

	registry := StrategyRegistry{}
	infos, err := registry.List()
	require.NoError(t, err)

	byID := make(map[string]StrategyInfo, len(infos))
	for _, info := range infos {
		byID[info.ID] = info
	}

	tests := []struct {
		id      string
		cadence string
	}{
		{id: "best_weekly_swing_v1", cadence: "weekly"},
		{id: "best_monthly_swing_v1", cadence: "monthly"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.id, func(t *testing.T) {
			t.Parallel()

			info, ok := byID[tt.id]
			require.True(t, ok)
			require.True(t, info.Approved)
			require.True(t, info.APICompatible)
			require.True(t, info.Validation.Valid)
			require.Equal(t, tt.cadence, info.Cadence)
			require.Equal(t, ScoringContractV1, info.ScoringContract)
			require.Equal(t, HostedStrategyAPIContract, info.APIContract)
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

func TestStrategySchemaAcceptsMinimalFractionalKellySizing(t *testing.T) {
	t.Parallel()

	schema := loadStrategySchema(t)
	doc := loadYAMLDocument(t, filepath.Join("..", "..", "strategies", "engine_weekly_swing_v1.yaml"))
	risk := doc["risk"].(map[string]any)
	risk["sizing_method"] = "fractional_kelly"

	require.NoError(t, schema.Validate(doc))
}

func TestStrategySchemaAcceptsHalfKellyFraction(t *testing.T) {
	t.Parallel()

	schema := loadStrategySchema(t)
	doc := loadYAMLDocument(t, filepath.Join("..", "..", "strategies", "engine_weekly_swing_v1.yaml"))
	risk := doc["risk"].(map[string]any)
	risk["sizing_method"] = "fractional_kelly"
	risk["kelly_fraction"] = 0.5

	require.NoError(t, schema.Validate(doc))
}

func TestStrategySchemaAcceptsRiskProfileWithOverrides(t *testing.T) {
	t.Parallel()

	schema := loadStrategySchema(t)
	doc := loadYAMLDocument(t, filepath.Join("..", "..", "strategies", "engine_weekly_swing_v1.yaml"))
	risk := doc["risk"].(map[string]any)
	risk["profile"] = "weekly_swing"
	delete(risk, "turnover_budget_pct")
	delete(risk, "max_single_trade_pct")
	delete(risk, "starter_position_pct")
	risk["turnover_budget_pct"] = 0.10

	require.NoError(t, schema.Validate(doc))
}

func TestStrategySchemaAcceptsAdvancedKellyOverrides(t *testing.T) {
	t.Parallel()

	schema := loadStrategySchema(t)
	doc := loadYAMLDocument(t, filepath.Join("..", "..", "strategies", "engine_weekly_swing_v1.yaml"))
	risk := doc["risk"].(map[string]any)
	risk["sizing_method"] = "fractional_kelly"
	risk["kelly_fraction"] = 0.25
	risk["kelly_min_edge"] = 0.0
	risk["kelly_max_fraction"] = 0.05
	risk["kelly_default_payoff_ratio"] = 1.0
	risk["kelly_min_confidence"] = 0.25
	risk["kelly_min_sample_count"] = 30
	risk["kelly_missing_edge_policy"] = "skip"

	require.NoError(t, schema.Validate(doc))
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

func TestBuiltInRiskProfilesPreserveExpandedSizing(t *testing.T) {
	t.Parallel()

	expected := map[string]RiskConfig{
		"best_monthly_swing_v1.yaml": {
			TurnoverBudgetPct:  0.05,
			MaxSingleTradePct:  0.025,
			StarterPositionPct: 0.012,
		},
		"best_weekly_swing_v1.yaml": {
			TurnoverBudgetPct:  0.12,
			MaxSingleTradePct:  0.035,
			StarterPositionPct: 0.015,
		},
		"engine_quality_swing_rebuild_v1.yaml": {
			TurnoverBudgetPct:  0.30,
			MaxSingleTradePct:  0.04,
			StarterPositionPct: 0.015,
		},
		"engine_quality_value_reversion_v1.yaml": {
			TurnoverBudgetPct:  0.10,
			MaxSingleTradePct:  0.03,
			StarterPositionPct: 0.015,
		},
		"engine_weekly_swing_v1.yaml": {
			TurnoverBudgetPct:  0.12,
			MaxSingleTradePct:  0.035,
			StarterPositionPct: 0.015,
		},
		"momentum_only_v1.yaml": {
			TurnoverBudgetPct:  0.20,
			MaxSingleTradePct:  0.05,
			StarterPositionPct: 0.02,
		},
		"momentum_profile_v1.yaml": {
			TurnoverBudgetPct:  0.20,
			MaxSingleTradePct:  0.05,
			StarterPositionPct: 0.02,
		},
		"portfolio_low_churn_swing_v1.yaml": {
			TurnoverBudgetPct:  0.06,
			MaxSingleTradePct:  0.025,
			StarterPositionPct: 0.012,
		},
		"portfolio_quality_value_reversion_v1.yaml": {
			TurnoverBudgetPct:  0.08,
			MaxSingleTradePct:  0.025,
			StarterPositionPct: 0.012,
		},
		"simple_score_v1.yaml": {
			TurnoverBudgetPct:  0.20,
			MaxSingleTradePct:  0.05,
			StarterPositionPct: 0.02,
		},
	}
	matches, err := filepath.Glob(filepath.Join("..", "..", "strategies", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			cfg, _, err := LoadStrategyFile(path)
			require.NoError(t, err)
			want := expected[filepath.Base(path)]
			require.Equal(t, want.TurnoverBudgetPct, cfg.Risk.TurnoverBudgetPct)
			require.Equal(t, want.MaxSingleTradePct, cfg.Risk.MaxSingleTradePct)
			require.Equal(t, want.StarterPositionPct, cfg.Risk.StarterPositionPct)
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

func TestLoadStrategyBytesExpandsRiskProfileAndKeepsOverrides(t *testing.T) {
	t.Parallel()

	cfg, _, err := LoadStrategyBytes([]byte(`
id: risk_profile_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
  rebalance: weekly
risk:
  profile: weekly_swing
  turnover_budget_pct: 0.10
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
`))
	require.NoError(t, err)
	require.True(t, ValidateStrategyConfig(cfg).Valid)
	require.Equal(t, RiskProfileWeeklySwing, cfg.Risk.Profile)
	require.Equal(t, 0.10, cfg.Risk.TurnoverBudgetPct)
	require.Equal(t, 0.035, cfg.Risk.MaxSingleTradePct)
	require.Equal(t, 0.015, cfg.Risk.StarterPositionPct)
}

func TestLoadStrategyBytesUsesCanonicalExpandedHash(t *testing.T) {
	t.Parallel()

	_, slimHash, err := LoadStrategyBytes([]byte(`
id: canonical_strategy
version: 1.0.0
defaults: swing_v1
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
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
`))
	require.NoError(t, err)

	_, expandedHash, err := LoadStrategyBytes([]byte(`
id: canonical_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
event_guardrail:
  enabled: true
  lookback_days: 14
  event_veto_score: -0.55
  event_boost_score: 0.70
  event_boost_amount: 0.03
portfolio:
  long_only: true
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
  protect_unscored_holdings: true
backtest:
  initial_cash: 100000
  fee_bps: 5
  slippage_bps: 10
`))
	require.NoError(t, err)

	require.Equal(t, slimHash, expandedHash)
}

func TestLoadStrategyBytesUsesCanonicalExpandedHashForRiskProfile(t *testing.T) {
	t.Parallel()

	_, profiledHash, err := LoadStrategyBytes([]byte(`
id: canonical_risk_profile_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
  rebalance: weekly
risk:
  profile: weekly_swing
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
`))
	require.NoError(t, err)

	_, expandedHash, err := LoadStrategyBytes([]byte(`
id: canonical_risk_profile_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
  rebalance: weekly
risk:
  turnover_budget_pct: 0.12
  max_single_trade_pct: 0.035
  starter_position_pct: 0.015
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
`))
	require.NoError(t, err)

	require.Equal(t, profiledHash, expandedHash)
}

func TestLoadStrategyBytesRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, _, err := LoadStrategyBytes([]byte(`
id: unknown_field_strategy
version: 1.0.0
approved: true
not_a_real_field: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
risk:
  turnover_budget_pct: 0.3
  max_single_trade_pct: 0.2
  starter_position_pct: 0.02
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
`))
	require.Error(t, err)
}

func TestHostedStrategyAPICompatibilityRejectsAdvancedScoring(t *testing.T) {
	t.Parallel()

	cfg, _, err := LoadStrategyBytes([]byte(`
id: local_v2_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.15
  profile_weight: 0.00
  quality_weight: 0.35
  value_weight: 0.35
  reversion_weight: 0.15
  min_buy_score: 0.62
  min_hold_score: 0.30
portfolio:
  long_only: true
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
risk:
  turnover_budget_pct: 0.3
  max_single_trade_pct: 0.2
  starter_position_pct: 0.02
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
  protect_unscored_holdings: true
backtest:
  initial_cash: 100000
  fee_bps: 5
  slippage_bps: 10
`))
	require.NoError(t, err)

	localValidation := ValidateStrategyConfig(cfg)
	require.True(t, localValidation.Valid)
	apiCompatibility := ValidateHostedStrategyAPICompatibility(cfg)
	require.False(t, apiCompatibility.Valid)
	require.Contains(t, apiCompatibility.Errors[0], HostedStrategyAPIContract)
	require.Equal(t, ScoringContractV2, StrategyScoringContract(cfg))
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
