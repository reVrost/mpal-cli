package mpal

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"sync/atomic"
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

func floatPtr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func kellyTestConfig() StrategyConfig {
	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Risk.SizingMethod = SizingMethodFractionalKelly
	return cfg
}

func markovEdge(pWin float64, pLoss float64, confidence float64, sampleCount int) *MarkovRead {
	return &MarkovRead{
		Horizon:                "weekly",
		HorizonBars:            5,
		FavorableProbability:   pWin,
		UnfavorableProbability: pLoss,
		Confidence:             confidence,
		SampleCount:            sampleCount,
	}
}

func rawKellyEdge(pWin float64, pLoss float64, confidence float64, sampleCount int) *RawKellyRead {
	raw := 0.0
	if pWin+pLoss > 0 {
		raw = (pWin - pLoss) / (pWin + pLoss)
	}
	return &RawKellyRead{
		Horizon:                "weekly",
		HorizonBars:            5,
		RawKelly:               round(raw, 6),
		FavorableProbability:   pWin,
		UnfavorableProbability: pLoss,
		PayoffRatio:            1,
		Confidence:             confidence,
		SampleCount:            sampleCount,
		CalibrationStatus:      "heuristic_markov",
		Source:                 "markov",
	}
}

func testBars(asOf time.Time, past float64, latest float64) []Bar {
	return []Bar{
		{Date: asOf.AddDate(0, 0, -80), Close: past},
		{Date: asOf, Close: latest},
	}
}

type blockingPrices struct {
	bars      BarsResult
	active    atomic.Int32
	maxActive atomic.Int32
}

func (f *blockingPrices) Bars(context.Context, string, time.Time, time.Time) (BarsResult, error) {
	active := f.active.Add(1)
	for {
		maxActive := f.maxActive.Load()
		if active <= maxActive || f.maxActive.CompareAndSwap(maxActive, active) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	f.active.Add(-1)
	return f.bars, nil
}

type recordingEventScores struct {
	mu      sync.Mutex
	calls   int
	tickers []string
	scores  map[string]EventScore
}

func (f *recordingEventScores) ScoresAsOf(_ context.Context, tickers []string, _ time.Time, _ int) (map[string]EventScore, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.tickers = append([]string{}, tickers...)
	result := map[string]EventScore{}
	for _, ticker := range NormalizeTickers(tickers) {
		if score, ok := f.scores[ticker]; ok {
			result[ticker] = score
		}
	}
	return result, nil
}

func TestRankSignalsUsesBoundedConcurrency(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	prices := &blockingPrices{bars: BarsResult{Bars: testBars(asOf, 90, 110)}}
	engine := Engine{
		Prices:            prices,
		Profiles:          fakeProfiles{score: ProfileScore{ProfileScore: 0.5}},
		SignalConcurrency: 4,
	}

	signals, warnings := engine.RankSignals(context.Background(), []string{"AAPL", "MSFT", "NVDA", "GOOG"}, asOf, testConfig())

	require.Empty(t, warnings)
	require.Len(t, signals, 4)
	require.Greater(t, prices.maxActive.Load(), int32(1))
}

func TestRankSignalsBatchesEventScoresOnce(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Events.Enabled = true
	events := &recordingEventScores{scores: map[string]EventScore{
		"AAPL": {Ticker: "AAPL", Score: 0.9, PublishedAt: asOf, ScoredAt: asOf},
	}}
	engine := Engine{
		Prices:            fakePrices{bars: BarsResult{Bars: testBars(asOf, 90, 110)}},
		Profiles:          fakeProfiles{score: ProfileScore{ProfileScore: 0.5}},
		Events:            events,
		SignalConcurrency: 1,
	}

	signals, warnings := engine.RankSignals(context.Background(), []string{"MSFT", "AAPL", "AAPL"}, asOf, cfg)

	require.Empty(t, warnings)
	require.Len(t, signals, 2)
	require.Equal(t, 1, events.calls)
	require.Equal(t, []string{"AAPL", "MSFT"}, events.tickers)
	for _, signal := range signals {
		if signal.Ticker == "AAPL" {
			require.NotNil(t, signal.EventScore)
			require.Equal(t, 0.9, *signal.EventScore)
		}
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

func TestStrategyRunTradeResultDoesNotAutoJournal(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
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
		Portfolio{Equity: 100000, Cash: 100000},
		testConfig(),
		StrategyRef{ID: "test_strategy", Version: "1.0.0", ConfigHash: "abc", Approved: true},
	)
	require.NoError(t, err)
	require.Equal(t, ResultTrade, result.Result)
	require.Equal(t, ResultTrade, result.ModelResult)
	require.Equal(t, ResultTrade, result.ExecutionResult)
	require.True(t, result.Validation.Valid)
	require.Empty(t, result.JournalEntryID)
	require.Len(t, result.BaselinePlan.ProposedTrades, 1)
	assert.Equal(t, SideBuy, result.BaselinePlan.ProposedTrades[0].Side)
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

func TestSignalScoreSupportsQualityValueReversionWeights(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	quality := 0.9
	value := 0.8
	cfg := testConfig()
	cfg.Scoring = ScoringConfig{
		MomentumWeight:  0,
		ProfileWeight:   0,
		QualityWeight:   0.4,
		ValueWeight:     0.4,
		ReversionWeight: 0.2,
		MinBuyScore:     0.6,
		MinHoldScore:    0.2,
	}
	engine := Engine{
		Prices: fakePrices{bars: BarsResult{Bars: []Bar{
			{Date: asOf.AddDate(0, 0, -120), Close: 100},
			{Date: asOf.AddDate(0, 0, -60), Close: 75},
			{Date: asOf, Close: 75},
		}}},
		Profiles: fakeProfiles{score: ProfileScore{
			Ticker:       "AAPL",
			AsOf:         asOf,
			ProfileScore: 0.1,
			QualityScore: &quality,
			ValueScore:   &value,
			ScoreSource:  "qvm_components",
		}},
	}

	signal, err := engine.SignalScore(context.Background(), "AAPL", asOf, cfg)
	require.NoError(t, err)
	require.NotNil(t, signal.QualityScore)
	require.NotNil(t, signal.ValueScore)
	require.NotNil(t, signal.ReversionScore)
	assert.Equal(t, 0.9, *signal.QualityScore)
	assert.Equal(t, 0.8, *signal.ValueScore)
	assert.Equal(t, 1.0, *signal.ReversionScore)
	assert.Equal(t, 0.88, signal.FinalScore)
	assert.Equal(t, SideBuy, signal.ActionHint)
	assert.Contains(t, signal.Reasons[0], "quality")
}

func TestMarkovHorizonFollowsRebalanceCadence(t *testing.T) {
	t.Parallel()

	horizon, bars := markovHorizon("daily")
	assert.Equal(t, "daily", horizon)
	assert.Equal(t, 1, bars)

	horizon, bars = markovHorizon("weekly")
	assert.Equal(t, "weekly", horizon)
	assert.Equal(t, 5, bars)

	horizon, bars = markovHorizon("monthly")
	assert.Equal(t, "monthly", horizon)
	assert.Equal(t, 21, bars)

	horizon, bars = markovHorizon("monthly_or_quarterly_manual")
	assert.Equal(t, "weekly", horizon)
	assert.Equal(t, 5, bars)
}

func TestMarkovReadProbabilitiesAndWarnings(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	read := markovRead(markovTestBars(asOf, 45), testConfig())
	require.NotNil(t, read)

	total := 0.0
	for _, probability := range read.TransitionProbabilities {
		total += probability
	}
	assert.InDelta(t, 1.0, total, 0.00001)
	assert.Equal(t, markovModelTrendBucketV1, read.Model)
	assert.Equal(t, "weekly", read.Horizon)
	assert.Equal(t, 5, read.HorizonBars)
	assert.NotEmpty(t, read.CurrentState)
	assert.LessOrEqual(t, read.FavorableProbability, 1.0)
	assert.LessOrEqual(t, read.UnfavorableProbability, 1.0)
	assert.NotEmpty(t, read.Warnings)
}

func TestSignalScoreIncludesMarkovMetadata(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Portfolio.Rebalance = "monthly"
	serverMarkov := markovEdge(0.7, 0.3, 0.6, 80)
	serverMarkov.Horizon = "monthly"
	serverMarkov.HorizonBars = 21
	serverMarkov.Model = markovModelTrendBucketV1
	serverKelly := rawKellyEdge(0.7, 0.3, 0.6, 80)
	serverKelly.Horizon = "monthly"
	serverKelly.HorizonBars = 21
	engine := Engine{
		Prices: fakePrices{bars: BarsResult{Bars: markovTestBars(asOf, 140)}},
		Profiles: fakeProfiles{score: ProfileScore{
			Ticker:       "AAPL",
			AsOf:         asOf,
			ProfileScore: 0.8,
			ScoreSource:  "qvm_score",
			Markov:       map[string]MarkovRead{"monthly": *serverMarkov},
			RawKelly:     map[string]RawKellyRead{"monthly": *serverKelly},
		}},
	}

	signal, err := engine.SignalScore(context.Background(), "AAPL", asOf, cfg)
	require.NoError(t, err)
	require.NotNil(t, signal.Markov)
	assert.Equal(t, "monthly", signal.Markov.Horizon)
	assert.Equal(t, 21, signal.Markov.HorizonBars)
	assert.Equal(t, markovModelTrendBucketV1, signal.Markov.Model)
	require.NotNil(t, signal.RawKelly)
	assert.Equal(t, "monthly", signal.RawKelly.Horizon)
}

func TestPlanPortfolioIgnoresMarkovWhenOrderingStarters(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Risk.MaxNewPositionsPerRun = 1
	signals := []SignalResult{
		{
			Ticker:     "LOW",
			FinalScore: 0.8,
			Markov: &MarkovRead{
				FavorableProbability: 0.95,
			},
		},
		{
			Ticker:     "HIGH",
			FinalScore: 0.9,
			Markov: &MarkovRead{
				FavorableProbability: 0.05,
			},
		},
	}

	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"HIGH", "LOW"}},
		Portfolio{Equity: 100000, Cash: 100000},
		signals,
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	assert.Equal(t, "HIGH", plan.ProposedTrades[0].Ticker)
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "LOW", Reason: "max_new_positions_per_run reached"})
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

func TestPlanPortfolioProtectsMissingComponentHolding(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Scoring = ScoringConfig{
		MomentumWeight: 0,
		ProfileWeight:  0,
		QualityWeight:  0.5,
		ValueWeight:    0.5,
		MinBuyScore:    0.6,
		MinHoldScore:   0.2,
	}
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"IVV.AX"}},
		Portfolio{Equity: 100000, Positions: []Position{{Ticker: "IVV.AX", MarketValue: 30000, Weight: 0.3}}},
		[]SignalResult{{
			Ticker:     "IVV.AX",
			FinalScore: 0,
			Warnings:   []string{"quality component unavailable: zero score used"},
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

func TestPlanPortfolioListingRegionTiltPrefersCloseRegionCandidate(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Portfolio.ListingRegionTilt = "US"
	cfg.Risk.MaxNewPositionsPerRun = 1
	signals := []SignalResult{
		{Ticker: "AAA.AX", FinalScore: 0.9},
		{Ticker: "MSFT", FinalScore: 0.86},
	}

	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAA.AX", "MSFT"}},
		Portfolio{Equity: 100000, Cash: 100000},
		signals,
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	assert.Equal(t, "MSFT", plan.ProposedTrades[0].Ticker)
	assert.Contains(t, plan.ProposedTrades[0].Reason, "preferred by listing-region tilt toward US")
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "AAA.AX", Reason: "max_new_positions_per_run reached"})
}

func TestPlanPortfolioListingRegionTiltDoesNotOverrideClearScoreLead(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Portfolio.ListingRegionTilt = "US"
	cfg.Risk.MaxNewPositionsPerRun = 1
	signals := []SignalResult{
		{Ticker: "AAA.AX", FinalScore: 0.96},
		{Ticker: "MSFT", FinalScore: 0.85},
	}

	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAA.AX", "MSFT"}},
		Portfolio{Equity: 100000, Cash: 100000},
		signals,
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	assert.Equal(t, "AAA.AX", plan.ProposedTrades[0].Ticker)
	assert.NotContains(t, plan.ProposedTrades[0].Reason, "listing-region tilt")
}

func TestPlanPortfolioListingRegionTiltSurfacesUSStartersWhenUnderweight(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 20
	cfg.Portfolio.ListingRegionTilt = "US"
	cfg.Risk.MaxNewPositionsPerRun = 3
	signals := []SignalResult{
		{Ticker: "MIN.AX", FinalScore: 1.0},
		{Ticker: "SXE.AX", FinalScore: 0.955275},
		{Ticker: "GNP.AX", FinalScore: 0.951175},
		{Ticker: "DOCN", FinalScore: 0.88695},
		{Ticker: "MU", FinalScore: 0.88565},
	}
	portfolio := Portfolio{
		Equity: 199826.89,
		Cash:   20000,
		Positions: []Position{
			{Ticker: "GOOGL", MarketValue: 30971.58, Weight: 0.154992},
			{Ticker: "AMD", MarketValue: 26380.90, Weight: 0.132019},
			{Ticker: "MSFT", MarketValue: 6301.07, Weight: 0.031533},
			{Ticker: "XYZ.AX", MarketValue: 36380.18, Weight: 0.182058},
			{Ticker: "LTR.AX", MarketValue: 24073.70, Weight: 0.120473},
			{Ticker: "XRO.AX", MarketValue: 18633.88, Weight: 0.093250},
		},
	}

	plan := PlanPortfolio(
		time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"MIN.AX", "SXE.AX", "GNP.AX", "DOCN", "MU"}},
		portfolio,
		signals,
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 3)
	tradesByTicker := map[string]ProposedTrade{}
	for _, trade := range plan.ProposedTrades {
		tradesByTicker[trade.Ticker] = trade
	}
	assert.Contains(t, tradesByTicker, "MIN.AX")
	assert.Contains(t, tradesByTicker, "DOCN")
	assert.Contains(t, tradesByTicker, "MU")
	assert.Contains(t, tradesByTicker["DOCN"].Reason, "preferred by listing-region tilt toward US")
	assert.Contains(t, tradesByTicker["MU"].Reason, "preferred by listing-region tilt toward US")
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "GNP.AX", Reason: "max_new_positions_per_run reached"})
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "SXE.AX", Reason: "max_new_positions_per_run reached"})
}

func TestPlanPortfolioListingRegionTiltInactiveAbovePreferredExposure(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Portfolio.MaxPositionPct = 0.8
	cfg.Portfolio.ListingRegionTilt = "US"
	cfg.Risk.MaxNewPositionsPerRun = 1
	signals := []SignalResult{
		{Ticker: "AAA.AX", FinalScore: 0.9},
		{Ticker: "MSFT", FinalScore: 0.86},
	}
	portfolio := Portfolio{
		Equity: 100000,
		Cash:   40000,
		Positions: []Position{
			{Ticker: "AAPL", MarketValue: 60000, Weight: 0.6},
		},
	}

	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAA.AX", "MSFT"}},
		portfolio,
		signals,
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	assert.Equal(t, "AAA.AX", plan.ProposedTrades[0].Ticker)
	assert.NotContains(t, plan.ProposedTrades[0].Reason, "listing-region tilt")
}

func TestPlanPortfolioDefaultFixedSizingRemainsUnchanged(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Risk.StarterPositionPct = 0.02
	cfg.Risk.MaxSingleTradePct = 0.2
	cfg.Portfolio.MaxPositionPct = 0.2
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.95, 0.05, 1, 100)}},
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	assert.Equal(t, 0.02, plan.ProposedTrades[0].TargetWeight)
	assert.Nil(t, plan.ProposedTrades[0].Sizing)
	assert.Equal(t, "starter position from top-ranked score above buy threshold", plan.ProposedTrades[0].Reason)
}

func TestPlanPortfolioKellySizingReducesWeakNoisyStarter(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.55, 0.45, 0.3, 100), RawKelly: rawKellyEdge(0.55, 0.45, 0.3, 100)}},
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	trade := plan.ProposedTrades[0]
	assert.Equal(t, 0.0075, trade.TargetWeight)
	require.NotNil(t, trade.Sizing)
	assert.Equal(t, SizingMethodFractionalKelly, trade.Sizing.Method)
	assert.Equal(t, "weekly", trade.Sizing.Horizon)
	assert.Equal(t, 5, trade.Sizing.HorizonBars)
	assert.Equal(t, 0.1, trade.Sizing.RawKelly)
	assert.Equal(t, 0.0075, trade.Sizing.TargetWeight)
	assert.Equal(t, 0.0075, trade.Sizing.KellyTargetWeight)
	assert.Equal(t, 0.0075, trade.Sizing.FinalTargetWeight)
	assert.Equal(t, SizingBindingKellyTarget, trade.Sizing.BindingConstraint)
	assert.Equal(t, 0.55, trade.Sizing.FavorableProbability)
	assert.Equal(t, 0.45, trade.Sizing.UnfavorableProbability)
	assert.Equal(t, "heuristic_markov", trade.Sizing.CalibrationStatus)
	assert.Contains(t, trade.Reason, "starter position sized by fractional Kelly")
}

func TestPlanPortfolioKellySizingCapsHighEdgeByKellyAndTradeCaps(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	cfg.Risk.MaxSingleTradePct = 0.03
	cfg.Risk.KellyMaxFraction = floatPtr(0.08)
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.9, 0.1, 1, 100), RawKelly: rawKellyEdge(0.9, 0.1, 1, 100)}},
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	trade := plan.ProposedTrades[0]
	assert.Equal(t, 0.03, trade.TargetWeight)
	require.NotNil(t, trade.Sizing)
	assert.Equal(t, 0.08, trade.Sizing.TargetWeight)
	assert.Equal(t, 0.08, trade.Sizing.KellyTargetWeight)
	assert.Equal(t, 0.03, trade.Sizing.FinalTargetWeight)
	assert.Equal(t, SizingBindingMaxSingleTradePct, trade.Sizing.BindingConstraint)
	assert.Contains(t, trade.Sizing.Warnings, "Kelly target 0.080 clamped to final target 0.030 by risk controls")
}

func TestPlanPortfolioKellySizingReportsKellyMaxFractionAsBindingConstraint(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	cfg.Risk.KellyMaxFraction = floatPtr(0.04)
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.9, 0.1, 1, 100), RawKelly: rawKellyEdge(0.9, 0.1, 1, 100)}},
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	trade := plan.ProposedTrades[0]
	assert.Equal(t, 0.04, trade.TargetWeight)
	require.NotNil(t, trade.Sizing)
	assert.Equal(t, 0.04, trade.Sizing.KellyTargetWeight)
	assert.Equal(t, 0.2, trade.Sizing.FractionalKelly)
	assert.Equal(t, 0.04, trade.Sizing.FinalTargetWeight)
	assert.Equal(t, SizingBindingKellyMaxFraction, trade.Sizing.BindingConstraint)
}

func TestPlanPortfolioKellyMissingMarkovFallsBackToFixed(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	cfg.Risk.KellyMissingEdgePolicy = KellyMissingEdgePolicyFixed
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9}},
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	trade := plan.ProposedTrades[0]
	assert.Equal(t, 0.02, trade.TargetWeight)
	require.NotNil(t, trade.Sizing)
	assert.Equal(t, SizingMethodFixed, trade.Sizing.Method)
	assert.Equal(t, "fixed_fallback", trade.Sizing.Source)
	assert.Equal(t, 0.02, trade.Sizing.FinalTargetWeight)
	assert.Equal(t, SizingBindingFixedFallback, trade.Sizing.BindingConstraint)
	assert.Contains(t, trade.Reason, "fixed sizing fallback")
}

func TestPlanPortfolioKellyMissingMarkovSkipsCandidate(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	cfg.Risk.KellyMissingEdgePolicy = KellyMissingEdgePolicySkip
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9}},
		cfg,
	)

	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "AAPL", Reason: "fractional Kelly edge unavailable: missing Markov edge data; missing_edge_policy=skip"})
}

func TestPlanPortfolioKellyLowConfidenceOrSampleCountDoesNotSize(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"LOWCONF", "LOWSAMPLE"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{
			{Ticker: "LOWCONF", FinalScore: 0.9, Markov: markovEdge(0.9, 0.1, 0.1, 100), RawKelly: rawKellyEdge(0.9, 0.1, 0.1, 100)},
			{Ticker: "LOWSAMPLE", FinalScore: 0.8, Markov: markovEdge(0.9, 0.1, 1, 10), RawKelly: rawKellyEdge(0.9, 0.1, 1, 10)},
		},
		cfg,
	)

	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "LOWCONF", Reason: "fractional Kelly edge unavailable: Markov confidence 0.100 below Kelly minimum 0.250"})
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "LOWSAMPLE", Reason: "fractional Kelly edge unavailable: Markov sample_count 10 below Kelly minimum 30"})
}

func TestPlanPortfolioKellyTopUpDoesNotExceedKellyTarget(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	cfg.Risk.TurnoverBudgetPct = 1
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 90000, Positions: []Position{{Ticker: "AAPL", MarketValue: 1000, Weight: 0.01}}},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.6, 0.4, 0.3, 100), RawKelly: rawKellyEdge(0.6, 0.4, 0.3, 100)}},
		cfg,
	)

	require.Len(t, plan.ProposedTrades, 1)
	trade := plan.ProposedTrades[0]
	assert.Equal(t, TradeIntentTopUp, trade.Intent)
	assert.Equal(t, 0.015, trade.TargetWeight)
	assert.Equal(t, 0.005, trade.DeltaWeight)
	require.NotNil(t, trade.Sizing)
	assert.Equal(t, 0.015, trade.Sizing.KellyTargetWeight)
	assert.Equal(t, 0.015, trade.Sizing.FinalTargetWeight)
	assert.Equal(t, SizingBindingKellyTarget, trade.Sizing.BindingConstraint)
}

func TestPlanPortfolioKellyMinTradeValuePreventsTinyStarter(t *testing.T) {
	t.Parallel()

	cfg := kellyTestConfig()
	cfg.Portfolio.MinTradeValue = 1000
	plan := PlanPortfolio(
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"AAPL"}},
		Portfolio{Equity: 100000, Cash: 100000},
		[]SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.55, 0.45, 0.3, 100), RawKelly: rawKellyEdge(0.55, 0.45, 0.3, 100)}},
		cfg,
	)

	assert.Empty(t, plan.ProposedTrades)
	assert.Contains(t, plan.Rejected, RejectedTicker{Ticker: "AAPL", Reason: "fractional Kelly target below min trade value or insufficient funding"})
}

func TestPlanPortfolioKellyTurnoverBudgetAndCashBufferStillApply(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	signal := SignalResult{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.9, 0.1, 1, 100), RawKelly: rawKellyEdge(0.9, 0.1, 1, 100)}

	turnoverCfg := kellyTestConfig()
	turnoverCfg.Risk.TurnoverBudgetPct = 0.01
	turnoverPlan := PlanPortfolio(asOf, Universe{Tickers: []string{"AAPL"}}, Portfolio{Equity: 100000, Cash: 100000}, []SignalResult{signal}, turnoverCfg)
	require.Len(t, turnoverPlan.ProposedTrades, 1)
	assert.Equal(t, 0.01, turnoverPlan.ProposedTrades[0].TargetWeight)
	require.NotNil(t, turnoverPlan.ProposedTrades[0].Sizing)
	assert.Equal(t, SizingBindingTurnoverBudgetPct, turnoverPlan.ProposedTrades[0].Sizing.BindingConstraint)

	cashCfg := kellyTestConfig()
	cashCfg.Risk.CashBufferPct = 0.02
	cashPlan := PlanPortfolio(asOf, Universe{Tickers: []string{"AAPL"}}, Portfolio{Equity: 100000, Cash: 3000}, []SignalResult{signal}, cashCfg)
	require.Len(t, cashPlan.ProposedTrades, 1)
	assert.Equal(t, 0.01, cashPlan.ProposedTrades[0].TargetWeight)
	require.NotNil(t, cashPlan.ProposedTrades[0].Sizing)
	assert.Equal(t, SizingBindingCashBufferPct, cashPlan.ProposedTrades[0].Sizing.BindingConstraint)
}

func TestValidateStrategyConfigAcceptsAndRejectsKellySizing(t *testing.T) {
	t.Parallel()

	valid := kellyTestConfig()
	valid.Risk.KellyMissingEdgePolicy = KellyMissingEdgePolicySkip
	assert.True(t, ValidateStrategyConfig(valid).Valid)

	invalid := kellyTestConfig()
	invalid.Risk.SizingMethod = "kelly"
	invalid.Risk.KellyFraction = floatPtr(0)
	invalid.Risk.KellyMaxFraction = floatPtr(2)
	invalid.Risk.KellyDefaultPayoffRatio = floatPtr(0)
	invalid.Risk.KellyMinConfidence = floatPtr(2)
	invalid.Risk.KellyMinSampleCount = intPtr(-1)
	invalid.Risk.KellyMissingEdgePolicy = "reject"
	result := ValidateStrategyConfig(invalid)
	require.False(t, result.Valid)
	assert.Contains(t, result.Errors, "risk.sizing_method must be empty, fixed, or fractional_kelly")
	assert.Contains(t, result.Errors, "risk.kelly_fraction must be in (0,1]")
	assert.Contains(t, result.Errors, "risk.kelly_max_fraction must be in (0,1]")
	assert.Contains(t, result.Errors, "risk.kelly_default_payoff_ratio must be > 0")
	assert.Contains(t, result.Errors, "risk.kelly_min_confidence must be in [0,1]")
	assert.Contains(t, result.Errors, "risk.kelly_min_sample_count must be >= 0")
	assert.Contains(t, result.Errors, "risk.kelly_missing_edge_policy must be fixed or skip")
}

func TestValidateStrategyConfigRejectsUnknownListingRegionTilt(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.ListingRegionTilt = "EU"

	result := ValidateStrategyConfig(cfg)

	require.False(t, result.Valid)
	assert.Contains(t, result.Errors, "portfolio.listing_region_tilt must be empty, US, or ASX")
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
		Type:      JournalTypeWeeklyTradeReview,
		CreatedAt: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Input: map[string]any{
			"raw_model_plan":   map[string]any{},
			"human_final_plan": map[string]any{},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "jrnl_test", entry.ID)

	entries, err := journal.List(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	got, err := journal.Get(context.Background(), "jrnl_test")
	require.NoError(t, err)
	assert.Equal(t, JournalTypeWeeklyTradeReview, got.Type)
}

func TestJournalListHandlesLargeEntries(t *testing.T) {
	t.Parallel()

	journal := FileJournal{Path: t.TempDir() + "/journal.jsonl"}
	largeValue := strings.Repeat("x", 128*1024)
	_, err := journal.Append(context.Background(), JournalEntry{
		ID:        "jrnl_large",
		Type:      JournalTypeBaselinePlan,
		CreatedAt: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Output:    map[string]any{"large": largeValue},
	})
	require.NoError(t, err)

	entries, err := journal.List(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	got, err := journal.Get(context.Background(), "jrnl_large")
	require.NoError(t, err)
	output, ok := got.Output.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, largeValue, output["large"])
}

func markovTestBars(asOf time.Time, count int) []Bar {
	start := asOf.AddDate(0, 0, -count+1)
	bars := make([]Bar, 0, count)
	price := 100.0
	for i := 0; i < count; i++ {
		if i%11 == 0 {
			price *= 0.985
		} else if i%7 == 0 {
			price *= 1.012
		} else {
			price *= 1.003
		}
		bars = append(bars, Bar{
			Date:  start.AddDate(0, 0, i),
			Close: price,
		})
	}
	return bars
}
