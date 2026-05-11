package mpal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDecisionGateEvidenceAuditsTradesRejectedAndAlternates(t *testing.T) {
	t.Parallel()

	run := decisionGateTestRun()
	result := BuildDecisionGateEvidence(run, DecisionGateOptions{Alternates: 1})

	require.Equal(t, DecisionGateMode, result.Mode)
	require.Equal(t, run.RunID, result.SourceRunID)
	require.NotEmpty(t, result.EvidenceHash)
	assert.Equal(t, "weekly", result.StrategyHorizon)
	assert.Equal(t, 5, result.StrategyHorizonBars)
	require.Len(t, result.Items, 3)

	trade := result.Items[0]
	assert.Equal(t, "AAPL", trade.Ticker)
	assert.Equal(t, DecisionGateRoleProposedTrade, trade.Role)
	assert.Equal(t, DecisionGateStatusBaselineExecutable, trade.BaselineStatus)
	assert.True(t, trade.IsExecutable)
	require.NotNil(t, trade.Sizing)
	assert.Equal(t, "weekly", trade.Sizing.Horizon)
	assert.Equal(t, SizingBindingMaxSingleTradePct, trade.Sizing.BindingConstraint)
	require.NotNil(t, trade.StrategyMarkov)
	assert.Equal(t, "weekly", trade.StrategyMarkov.Horizon)

	rejected := result.Items[1]
	assert.Equal(t, "MSFT", rejected.Ticker)
	assert.Equal(t, DecisionGateRoleRejected, rejected.Role)
	assert.Equal(t, DecisionGateStatusRejectedByStrategy, rejected.BaselineStatus)
	assert.Equal(t, "event veto", rejected.Reason)

	alternate := result.Items[2]
	assert.Equal(t, "GOOGL", alternate.Ticker)
	assert.Equal(t, DecisionGateRoleAlternateSignal, alternate.Role)
	assert.Equal(t, DecisionGateStatusAlternateContext, alternate.BaselineStatus)
	assert.False(t, alternate.IsExecutable)
	assert.Equal(t, 0.82, alternate.Score)
}

func TestBuildDecisionGateEvidenceMarksInvalidBaseline(t *testing.T) {
	t.Parallel()

	run := decisionGateTestRun()
	run.Validation = ValidationResult{Valid: false, Errors: []string{"plan exceeds turnover budget"}}
	result := BuildDecisionGateEvidence(run, DecisionGateOptions{})

	require.NotEmpty(t, result.Items)
	assert.Equal(t, DecisionGateStatusBaselineInvalid, result.Items[0].BaselineStatus)
	assert.False(t, result.Items[0].IsExecutable)
}

func TestBuildDecisionGateEvidenceSuppressesAlternatesWhenZero(t *testing.T) {
	t.Parallel()

	result := BuildDecisionGateEvidence(decisionGateTestRun(), DecisionGateOptions{Alternates: 0})

	require.Len(t, result.Items, 2)
	for _, item := range result.Items {
		assert.NotEqual(t, DecisionGateRoleAlternateSignal, item.Role)
	}
}

func TestBuildDecisionGateEvidenceAddsContextOnlyMarkov(t *testing.T) {
	t.Parallel()

	run := decisionGateTestRun()
	cfg := testConfig()
	cfg.Risk.SizingMethod = SizingMethodFractionalKelly
	contextRead := markovEdge(0.7, 0.3, 0.5, 100)
	contextRead.Horizon = "daily"
	contextRead.HorizonBars = 1
	contextKelly := rawKellyEdge(0.7, 0.3, 0.5, 100)
	contextKelly.Horizon = "daily"
	contextKelly.HorizonBars = 1
	result := BuildDecisionGateEvidence(run, DecisionGateOptions{
		Alternates: 0,
		Strategy:   &cfg,
		MarkovContexts: []TickerMarkovResult{{
			Rebalance:   "daily",
			Horizon:     "daily",
			HorizonBars: 1,
			Results: []TickerMarkovItem{{
				Ticker:   "AAPL",
				Markov:   contextRead,
				RawKelly: contextKelly,
			}},
		}},
	})

	require.NotEmpty(t, result.Items)
	require.Len(t, result.Items[0].MarkovContext, 1)
	context := result.Items[0].MarkovContext[0]
	assert.True(t, context.ContextOnly)
	assert.Equal(t, "daily", context.Horizon)
	assert.Equal(t, 1, context.HorizonBars)
	assert.Equal(t, 0.4, context.RawKelly)
	assert.Equal(t, 0.05, context.FractionalKelly)
	assert.Equal(t, 0.05, context.KellyTargetWeight)
	assert.Equal(t, "heuristic_markov", context.CalibrationStatus)
	assert.Equal(t, DecisionGateStatusBaselineExecutable, result.Items[0].BaselineStatus)
}

func TestBuildDecisionGateEvidenceHashIsStable(t *testing.T) {
	t.Parallel()

	run := decisionGateTestRun()
	left := BuildDecisionGateEvidence(run, DecisionGateOptions{Alternates: 1})
	right := BuildDecisionGateEvidence(run, DecisionGateOptions{Alternates: 1})

	assert.Equal(t, left.EvidenceHash, right.EvidenceHash)
}

func decisionGateTestRun() StrategyRunResult {
	asOf := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	aaplMarkov := markovEdge(0.65, 0.35, 0.5, 100)
	msftMarkov := markovEdge(0.4, 0.6, 0.5, 100)
	googlMarkov := markovEdge(0.6, 0.4, 0.5, 100)
	aaplKelly := rawKellyEdge(0.65, 0.35, 0.5, 100)
	msftKelly := rawKellyEdge(0.4, 0.6, 0.5, 100)
	googlKelly := rawKellyEdge(0.6, 0.4, 0.5, 100)
	return StrategyRunResult{
		RunID:           "strategy_run_test",
		Mode:            "strategy_run",
		AsOf:            asOf,
		Strategy:        StrategyRef{ID: "test_strategy", Version: "1.0.0", ConfigHash: "sha256:test", Approved: true},
		Result:          ResultTrade,
		ModelResult:     ResultTrade,
		ExecutionResult: ResultTrade,
		Signals: []SignalResult{
			{Ticker: "AAPL", FinalScore: 0.91, ActionHint: "BUY_CANDIDATE", Markov: aaplMarkov, RawKelly: aaplKelly},
			{Ticker: "MSFT", FinalScore: 0.88, ActionHint: "BUY_CANDIDATE", Markov: msftMarkov, RawKelly: msftKelly},
			{Ticker: "GOOGL", FinalScore: 0.82, ActionHint: "BUY_CANDIDATE", Markov: googlMarkov, RawKelly: googlKelly},
		},
		BaselinePlan: PortfolioPlanResult{
			Result: ResultTrade,
			ProposedTrades: []ProposedTrade{{
				Ticker:         "AAPL",
				Side:           SideBuy,
				Intent:         TradeIntentStarter,
				TargetWeight:   0.03,
				DeltaWeight:    0.03,
				EstimatedValue: 3000,
				Reason:         "starter position sized by fractional Kelly",
				Sizing: &SizingDecision{
					Method:            SizingMethodFractionalKelly,
					Source:            "markov",
					Horizon:           "weekly",
					HorizonBars:       5,
					RawKelly:          0.3,
					FractionalKelly:   0.0375,
					TargetWeight:      0.0375,
					KellyTargetWeight: 0.0375,
					FinalTargetWeight: 0.03,
					BindingConstraint: SizingBindingMaxSingleTradePct,
					CalibrationStatus: "heuristic_markov",
					Confidence:        0.5,
					SampleCount:       100,
				},
			}},
			Rejected: []RejectedTicker{{Ticker: "MSFT", Reason: "event veto"}},
		},
		Validation: ValidationResult{Valid: true},
	}
}
