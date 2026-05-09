package mpal

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePrices struct {
	bars BarsResult
}

func (f fakePrices) Bars(context.Context, string, time.Time, time.Time) (BarsResult, error) {
	return f.bars, nil
}

type fakeProfiles struct {
	score ProfileScore
	err   error
}

func (f fakeProfiles) Score(context.Context, string, time.Time) (ProfileScore, error) {
	if f.err != nil {
		return ProfileScore{}, f.err
	}
	return f.score, nil
}

type fakeEventScores struct {
	scores map[string]EventScore
	err    error
}

func (f fakeEventScores) ScoresAsOf(_ context.Context, tickers []string, _ time.Time, _ int) (map[string]EventScore, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := map[string]EventScore{}
	for _, ticker := range NormalizeTickers(tickers) {
		if score, ok := f.scores[ticker]; ok {
			result[ticker] = score
		}
	}
	return result, nil
}

func testConfig() StrategyConfig {
	return StrategyConfig{
		ID:       "test_strategy",
		Version:  "1.0.0",
		Approved: true,
		Scoring: ScoringConfig{
			MomentumWeight: 0.7,
			ProfileWeight:  0.3,
			MinBuyScore:    0.6,
			MinHoldScore:   0.2,
		},
		Portfolio: PortfolioConfig{
			LongOnly:       true,
			MaxPositions:   2,
			MaxPositionPct: 0.2,
			MinTradeValue:  100,
		},
		Risk: RiskConfig{
			TurnoverBudgetPct:       1,
			MaxSingleTradePct:       0.2,
			StarterPositionPct:      0.02,
			MaxNewPositionsPerRun:   2,
			CashBufferPct:           0.02,
			ProtectUnscoredHoldings: true,
		},
		Backtest: BacktestConfig{InitialCash: 100000},
	}
}

func testBars(asOf time.Time, past float64, latest float64) []Bar {
	return []Bar{
		{Date: asOf.AddDate(0, 0, -80), Close: past},
		{Date: asOf, Close: latest},
	}
}

func TestValidateStrategyConfig(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	result := ValidateStrategyConfig(cfg)
	require.True(t, result.Valid)

	cfg.Scoring.ProfileWeight = 0.4
	result = ValidateStrategyConfig(cfg)
	require.False(t, result.Valid)
	assert.Contains(t, result.Errors, "scoring weights must sum to 1")
}

func TestStrategyRunTradeResultAndJournal(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	journal := FileJournal{Path: t.TempDir() + "/journal.jsonl"}
	engine := Engine{
		Prices: fakePrices{bars: BarsResult{Bars: []Bar{
			{Date: asOf.AddDate(0, 0, -80), Close: 100},
			{Date: asOf, Close: 130},
		}}},
		Profiles: fakeProfiles{score: ProfileScore{
			Ticker:       "AAPL",
			AsOf:         asOf,
			ProfileScore: 0.8,
			ScoreSource:  "qvm_score",
		}},
		Journal: journal,
	}

	result, err := engine.StrategyRun(
		context.Background(),
		asOf,
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		testConfig(),
		StrategyRef{ID: "test_strategy", Version: "1.0.0", ConfigHash: "abc", Approved: true},
	)
	require.NoError(t, err)
	require.Equal(t, ResultTrade, result.Result)
	require.Equal(t, ResultTrade, result.ModelResult)
	require.Equal(t, ResultTrade, result.ExecutionResult)
	require.True(t, result.Validation.Valid)
	require.NotEmpty(t, result.JournalEntryID)
	require.Len(t, result.BaselinePlan.ProposedTrades, 1)
	assert.Equal(t, SideBuy, result.BaselinePlan.ProposedTrades[0].Side)

	entry, err := journal.Get(context.Background(), result.JournalEntryID)
	require.NoError(t, err)
	assert.Equal(t, JournalTypeBaselinePlan, entry.Type)
	output, ok := entry.Output.(map[string]any)
	require.True(t, ok)
	validation, ok := output["validation"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, validation["valid"])
}

func TestStrategyRunNoTradeResult(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	engine := Engine{
		Prices: fakePrices{bars: BarsResult{Bars: []Bar{
			{Date: asOf.AddDate(0, 0, -80), Close: 100},
			{Date: asOf, Close: 101},
		}}},
		Profiles: fakeProfiles{score: ProfileScore{Ticker: "AAPL", AsOf: asOf}},
	}

	result, err := engine.StrategyRun(
		context.Background(),
		asOf,
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		testConfig(),
		StrategyRef{ID: "test_strategy", Version: "1.0.0", ConfigHash: "abc", Approved: true},
	)
	require.NoError(t, err)
	require.Equal(t, ResultNoTrade, result.Result)
	require.Equal(t, ResultNoTrade, result.ModelResult)
	require.Equal(t, ResultNoTrade, result.ExecutionResult)
	require.True(t, result.Validation.Valid)
	assert.Empty(t, result.BaselinePlan.ProposedTrades)
}

func TestStrategyRunKeepsSignalTradeWhenNoExecutablePlan(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	engine := Engine{
		Prices: fakePrices{bars: BarsResult{Bars: []Bar{
			{Date: asOf.AddDate(0, 0, -80), Close: 100},
			{Date: asOf, Close: 130},
		}}},
		Profiles: fakeProfiles{score: ProfileScore{
			Ticker:       "AAPL",
			AsOf:         asOf,
			ProfileScore: 0.8,
			ScoreSource:  "qvm_score",
		}},
	}

	result, err := engine.StrategyRun(
		context.Background(),
		asOf,
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 0},
		cfg,
		StrategyRef{ID: "test_strategy", Version: "1.0.0", ConfigHash: "abc", Approved: true},
	)

	require.NoError(t, err)
	require.Equal(t, ResultNoTrade, result.Result)
	require.Equal(t, ResultTrade, result.ModelResult)
	require.Equal(t, ResultNoTrade, result.ExecutionResult)
	require.True(t, result.Validation.Valid)
	assert.Empty(t, result.BaselinePlan.ProposedTrades)
}

func TestMissingProfileFallsBackToNeutralWarning(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	engine := Engine{
		Prices: fakePrices{bars: BarsResult{Bars: []Bar{
			{Date: asOf.AddDate(0, 0, -80), Close: 100},
			{Date: asOf, Close: 130},
		}}},
		Profiles: fakeProfiles{err: errors.New("not found")},
	}

	signal, err := engine.SignalScore(context.Background(), "AAPL", asOf, testConfig())
	require.NoError(t, err)
	assert.Equal(t, 0.0, signal.ProfileScore)
	require.NotEmpty(t, signal.Warnings)
	assert.Contains(t, signal.Warnings[0], "profile unavailable")
}

func TestSignalScoreAppliesEventBoost(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	confidence := 0.8
	cfg := testConfig()
	cfg.Events.Enabled = true
	engine := Engine{
		Prices:   fakePrices{bars: BarsResult{Bars: testBars(asOf, 100, 114)}},
		Profiles: fakeProfiles{score: ProfileScore{Ticker: "AAPL", AsOf: asOf, ProfileScore: 0.3}},
		Events: fakeEventScores{scores: map[string]EventScore{
			"AAPL": {
				Ticker:      "AAPL",
				PublishedAt: asOf.AddDate(0, 0, -1),
				Score:       0.7,
				Confidence:  &confidence,
				ScoredAt:    asOf.Add(-time.Hour),
			},
		}},
	}

	signal, err := engine.SignalScore(context.Background(), "AAPL", asOf, cfg)
	require.NoError(t, err)
	require.NotNil(t, signal.EventScore, "warnings: %+v reasons: %+v", signal.Warnings, signal.Reasons)
	assert.Equal(t, 0.7, *signal.EventScore)
	assert.Equal(t, 0.63, signal.FinalScore)
	assert.Equal(t, SideBuy, signal.ActionHint)
	assert.False(t, signal.EventVeto)
	assert.Contains(t, signal.Reasons, "event boost 0.05 from latest scored article")
}

func TestPlanPortfolioRejectsEventVetoStarter(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, EventVeto: true}},
		cfg,
	)

	require.Equal(t, ResultNoTrade, plan.Result)
	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "AAPL", Reason: "event veto"})
}

func TestPlanPortfolioSkipsEventVetoTopUp(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Risk.TurnoverBudgetPct = 1
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 90000, Positions: []Position{{Ticker: "AAPL", MarketValue: 10000, Weight: 0.1}}},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, EventVeto: true}},
		cfg,
	)

	require.Equal(t, ResultNoTrade, plan.Result)
	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Warnings, "AAPL flagged for review: negative scored event")
}

func TestValidatePlanRejectsNonUniverseAndTurnover(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Risk.TurnoverBudgetPct = 0.1
	plan := PortfolioPlanResult{
		Targets: []TargetPosition{{Ticker: "MSFT", TargetWeight: 0.2}},
		ProposedTrades: []ProposedTrade{
			{Ticker: "MSFT", Side: SideBuy, DeltaWeight: 0.2},
		},
	}

	result := ValidatePlan(plan, Universe{Tickers: []string{"AAPL"}}, Portfolio{Equity: 100000}, cfg)
	require.False(t, result.Valid)
	assert.Contains(t, result.Errors, "MSFT is not in universe")
	assert.Contains(t, result.Errors, "MSFT trade is not in universe")
	assert.Contains(t, result.Errors, "plan exceeds turnover budget")
}

func TestPlanPortfolioPreservesExistingHoldingAboveHoldThreshold(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Positions: []Position{{Ticker: "AAPL", MarketValue: 10000, Weight: 0.1}}},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.5}},
		cfg,
	)

	require.Equal(t, ResultNoTrade, plan.Result)
	assert.Empty(t, plan.ProposedTrades)
}

func TestPlanPortfolioProtectsUnscoredHoldingsByDefault(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Positions: []Position{{Ticker: "IVV.AX", MarketValue: 20000, Weight: 0.2}}},
		nil,
		cfg,
	)

	require.Equal(t, ResultNoTrade, plan.Result)
	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Warnings, "IVV.AX protected: no usable score")
}

func TestPlanPortfolioProtectsMissingProfileHolding(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"IVV.AX"}},
		Portfolio{Equity: 100000, Positions: []Position{{Ticker: "IVV.AX", MarketValue: 30000, Weight: 0.3}}},
		[]SignalResult{{
			Ticker:        "IVV.AX",
			MomentumScore: 0,
			ProfileScore:  0,
			FinalScore:    0,
			Warnings:      []string{"profile unavailable: profile not found"},
		}},
		cfg,
	)

	require.Equal(t, ResultNoTrade, plan.Result)
	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Warnings, "IVV.AX protected: no usable score")
}

func TestPlanPortfolioCapsTurnoverAndSingleTrade(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Risk.TurnoverBudgetPct = 0.1
	cfg.Risk.MaxSingleTradePct = 0.05
	cfg.Risk.StarterPositionPct = 0.02
	portfolio := Portfolio{Equity: 100000, Positions: []Position{
		{Ticker: "AAA", MarketValue: 20000, Weight: 0.2},
		{Ticker: "BBB", MarketValue: 20000, Weight: 0.2},
		{Ticker: "CCC", MarketValue: 20000, Weight: 0.2},
	}}
	signals := []SignalResult{
		{Ticker: "AAA", FinalScore: 0.1},
		{Ticker: "BBB", FinalScore: 0.1},
		{Ticker: "CCC", FinalScore: 0.1},
	}

	plan := PlanPortfolio(time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), Universe{Tickers: []string{"AAA", "BBB", "CCC"}}, portfolio, signals, cfg)

	require.Len(t, plan.ProposedTrades, 2)
	turnover := 0.0
	for _, trade := range plan.ProposedTrades {
		assert.LessOrEqual(t, math.Abs(trade.DeltaWeight), 0.050001)
		turnover += math.Abs(trade.DeltaWeight)
	}
	assert.LessOrEqual(t, turnover, 0.100001)
}

func TestPlanPortfolioLimitsStarterPositions(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Risk.MaxNewPositionsPerRun = 2
	signals := []SignalResult{
		{Ticker: "AAA", FinalScore: 0.9},
		{Ticker: "BBB", FinalScore: 0.8},
		{Ticker: "CCC", FinalScore: 0.7},
	}

	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAA", "BBB", "CCC"}},
		Portfolio{Equity: 100000, Cash: 100000},
		signals,
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 2)
	for _, trade := range plan.ProposedTrades {
		assert.Equal(t, TradeIntentStarter, trade.Intent)
		assert.Equal(t, 0.02, trade.TargetWeight)
	}
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "CCC", Reason: "max_new_positions_per_run reached"})
}

func TestValidatePlanAllowsReducingExistingNonUniverseHolding(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	plan := PortfolioPlanResult{
		Targets: []TargetPosition{{Ticker: "AAPL", TargetWeight: 0.05}},
		ProposedTrades: []ProposedTrade{
			{Ticker: "AAPL", Side: SideSell, DeltaWeight: -0.05},
		},
	}

	result := ValidatePlan(
		plan,
		Universe{Tickers: []string{"MSFT"}},
		Portfolio{Equity: 100000, Positions: []Position{{Ticker: "AAPL", MarketValue: 10000, Weight: 0.1}}},
		cfg,
	)

	require.True(t, result.Valid)
}

func TestValidatePlanAllowsTrimmingOverweightHoldingTowardLimit(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositionPct = 0.12
	plan := PortfolioPlanResult{
		Targets: []TargetPosition{{Ticker: "PLS.AX", TargetWeight: 0.164}},
		ProposedTrades: []ProposedTrade{
			{Ticker: "PLS.AX", Side: SideSell, DeltaWeight: -0.05},
		},
	}

	result := ValidatePlan(
		plan,
		Universe{Tickers: []string{"PLS.AX"}},
		Portfolio{Equity: 100000, Positions: []Position{{Ticker: "PLS.AX", MarketValue: 21400, Weight: 0.214}}},
		cfg,
	)

	require.True(t, result.Valid)
}

func TestJournalAppendListGet(t *testing.T) {
	t.Parallel()

	journal := FileJournal{Path: t.TempDir() + "/journal.jsonl"}
	entry, err := journal.Append(context.Background(), JournalEntry{
		ID:        "jrnl_test",
		Type:      JournalTypeAgentOverride,
		CreatedAt: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "jrnl_test", entry.ID)

	entries, err := journal.List(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	got, err := journal.Get(context.Background(), "jrnl_test")
	require.NoError(t, err)
	assert.Equal(t, JournalTypeAgentOverride, got.Type)
}
