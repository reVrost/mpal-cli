package mpal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBacktestPrices struct {
	byTicker map[string][]Bar
}

func (f fakeBacktestPrices) Bars(_ context.Context, ticker string, start time.Time, end time.Time) (BarsResult, error) {
	bars := f.byTicker[ticker]
	filtered := make([]Bar, 0, len(bars))
	for _, bar := range bars {
		if bar.Date.Before(start) || bar.Date.After(end) {
			continue
		}
		filtered = append(filtered, bar)
	}
	return BarsResult{
		Ticker: ticker,
		Start:  start,
		End:    end,
		Bars:   filtered,
		Freshness: &Freshness{
			Source:   "marketpal_historical_prices",
			Provider: "marketpal",
			Storage:  "dynamodb",
			Stale:    false,
		},
	}, nil
}

type customFreshnessBacktestPrices struct {
	byTicker  map[string][]Bar
	freshness *Freshness
}

func (f customFreshnessBacktestPrices) Bars(_ context.Context, ticker string, start time.Time, end time.Time) (BarsResult, error) {
	bars := f.byTicker[ticker]
	filtered := make([]Bar, 0, len(bars))
	for _, bar := range bars {
		if bar.Date.Before(start) || bar.Date.After(end) {
			continue
		}
		filtered = append(filtered, bar)
	}
	return BarsResult{
		Ticker:    ticker,
		Start:     start,
		End:       end,
		Bars:      filtered,
		Freshness: f.freshness,
	}, nil
}

type fakeFactorSnapshots struct {
	byTicker map[string][]FactorSnapshot
}

func (f fakeFactorSnapshots) SnapshotsAsOf(_ context.Context, tickers []string, asOf time.Time, _ string) (map[string]FactorSnapshot, error) {
	result := map[string]FactorSnapshot{}
	for _, ticker := range NormalizeTickers(tickers) {
		for _, snapshot := range f.byTicker[ticker] {
			if snapshot.SnapshotDate.After(asOf) {
				continue
			}
			current, ok := result[ticker]
			if !ok || snapshot.SnapshotDate.After(current.SnapshotDate) {
				result[ticker] = snapshot
			}
		}
	}
	return result, nil
}

func (f fakeFactorSnapshots) Coverage(_ context.Context, tickers []string, _ string) ([]FactorSnapshotCoverage, error) {
	var result []FactorSnapshotCoverage
	for _, ticker := range NormalizeTickers(tickers) {
		snapshots := f.byTicker[ticker]
		if len(snapshots) == 0 {
			continue
		}
		result = append(result, FactorSnapshotCoverage{
			YahooTicker:        ticker,
			FirstSnapshotDate:  snapshots[0].SnapshotDate,
			LatestSnapshotDate: snapshots[len(snapshots)-1].SnapshotDate,
			SnapshotCount:      int64(len(snapshots)),
		})
	}
	return result, nil
}

func TestBacktestRunMomentumOnlyExecutesAtNextOpen(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 1
	cfg.Scoring.ProfileWeight = 0
	cfg.Portfolio.MaxPositions = 1
	cfg.Portfolio.MaxPositionPct = 0.98

	engine := Engine{Prices: fakeBacktestPrices{byTicker: map[string][]Bar{
		"AAPL": risingBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, 5), 100, 0.8),
	}}}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{},
	)

	require.NoError(t, err)
	require.True(t, result.Trusted)
	require.NotEmpty(t, result.Trades)
	assert.True(t, result.Trades[0].Date.After(result.Trades[0].SignalDate))
	assert.Greater(t, result.Metrics.FinalEquity, result.Metrics.InitialEquity)
}

func TestBacktestRunDoesNotApplyFillOnSignalDate(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 1
	cfg.Scoring.ProfileWeight = 0
	cfg.Portfolio.MaxPositions = 1
	cfg.Portfolio.MaxPositionPct = 0.98
	cfg.Backtest.FeeBps = 0
	cfg.Backtest.SlippageBps = 0

	engine := Engine{Prices: fakeBacktestPrices{byTicker: map[string][]Bar{
		"AAPL": risingBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, 5), 100, 1),
	}}}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{},
	)

	require.NoError(t, err)
	require.NotEmpty(t, result.Trades)
	assert.Equal(t, start.AddDate(0, 0, 1), result.Trades[0].Date)
	require.NotEmpty(t, result.EquityCurve)
	assert.Equal(t, start, result.EquityCurve[0].Date)
	assert.Equal(t, 100000.0, result.EquityCurve[0].Equity)
	assert.Equal(t, 100000.0, result.EquityCurve[0].Cash)
	assert.Equal(t, 0, result.EquityCurve[0].Positions)
}

func TestBacktestRunRejectsPriceSourceWithoutDurableStorage(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 1
	cfg.Scoring.ProfileWeight = 0

	engine := Engine{Prices: customFreshnessBacktestPrices{
		byTicker: map[string][]Bar{
			"AAPL": risingBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, 5), 100, 0.8),
		},
		freshness: &Freshness{Source: "custom_loader", Provider: "custom"},
	}}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{},
	)

	require.Error(t, err)
	var untrusted UntrustedBacktestError
	require.True(t, errors.As(err, &untrusted))
	assert.False(t, result.Trusted)
	assert.Contains(t, result.DataQuality.Blockers, "AAPL price data storage is not trusted for backtests: custom_loader/custom")
}

func TestBacktestRunRejectsMissingPriceCoverage(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 1
	cfg.Scoring.ProfileWeight = 0

	engine := Engine{Prices: fakeBacktestPrices{byTicker: map[string][]Bar{
		"AAPL": risingBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, -10), 100, 0.8),
	}}}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{},
	)

	require.Error(t, err)
	var untrusted UntrustedBacktestError
	require.True(t, errors.As(err, &untrusted))
	assert.Contains(t, result.DataQuality.Blockers, "AAPL price coverage ends before requested end 2026-01-30")
}

func TestBacktestRunAllowUntrustedReturnsUntrustedResult(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 1
	cfg.Scoring.ProfileWeight = 0

	engine := Engine{Prices: customFreshnessBacktestPrices{
		byTicker: map[string][]Bar{
			"AAPL": risingBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, 5), 100, 0.8),
		},
		freshness: &Freshness{Source: "custom_loader", Provider: "custom"},
	}}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{AllowUntrusted: true},
	)

	require.NoError(t, err)
	assert.False(t, result.Trusted)
	assert.Equal(t, "untrusted", result.TrustStatus)
}

func TestBacktestRunProfileStrategyRequiresSnapshots(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 0.7
	cfg.Scoring.ProfileWeight = 0.3

	engine := Engine{Prices: fakeBacktestPrices{byTicker: map[string][]Bar{
		"AAPL": risingBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, 5), 100, 0.8),
	}}}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{},
	)

	require.Error(t, err)
	var untrusted UntrustedBacktestError
	require.True(t, errors.As(err, &untrusted))
	assert.False(t, result.Trusted)
	assert.Contains(t, result.TrustStatus, "untrusted")
}

func TestBacktestRunUsesLatestSnapshotAsOfWithoutLookahead(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	lowScore := 20.0
	highScore := 100.0
	cfg := testConfig()
	cfg.Scoring.MomentumWeight = 0
	cfg.Scoring.ProfileWeight = 1
	cfg.Portfolio.MaxPositions = 1

	engine := Engine{
		Prices: fakeBacktestPrices{byTicker: map[string][]Bar{
			"AAPL": flatBacktestBars(start.AddDate(0, 0, -90), end.AddDate(0, 0, 5), 100),
		}},
		Factors: fakeFactorSnapshots{byTicker: map[string][]FactorSnapshot{
			"AAPL": {
				{Ticker: "AAPL", YahooTicker: "AAPL", SnapshotDate: start.AddDate(0, 0, -5), QVMScore: &lowScore, ProfileVersion: defaultBacktestProfileVersion},
				{Ticker: "AAPL", YahooTicker: "AAPL", SnapshotDate: end.AddDate(0, 0, 10), QVMScore: &highScore, ProfileVersion: defaultBacktestProfileVersion},
			},
		}},
	}
	result, err := engine.BacktestRun(
		context.Background(),
		start,
		end,
		Universe{Tickers: []string{"AAPL"}},
		cfg,
		StrategyRef{ID: cfg.ID, Version: cfg.Version, Approved: true},
		BacktestOptions{},
	)

	require.NoError(t, err)
	require.True(t, result.Trusted)
	assert.Empty(t, result.Trades)
}

func TestPlanPortfolioReservesCashBufferForStarters(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Portfolio.MaxPositions = 5
	cfg.Portfolio.MaxPositionPct = 0.5
	cfg.Risk.CashBufferPct = 0.02
	cfg.Risk.StarterPositionPct = 0.5
	cfg.Risk.MaxSingleTradePct = 0.5
	cfg.Risk.MaxNewPositionsPerRun = 5
	signals := []SignalResult{
		{Ticker: "A", FinalScore: 1},
		{Ticker: "B", FinalScore: 0.99},
		{Ticker: "C", FinalScore: 0.98},
		{Ticker: "D", FinalScore: 0.97},
		{Ticker: "E", FinalScore: 0.96},
	}
	plan := PlanPortfolio(
		time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		Universe{Tickers: []string{"A", "B", "C", "D", "E"}},
		Portfolio{Cash: 100000, Equity: 100000},
		signals,
		cfg,
	)

	require.Len(t, plan.Targets, 2)
	total := 0.0
	for _, target := range plan.Targets {
		total += target.TargetWeight
	}
	assert.InDelta(t, 0.98, total, 0.000001)
}

func TestPlanPortfolioComparisonFixedVersusFractionalKellySizing(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	universe := Universe{Tickers: []string{"AAPL"}}
	portfolio := Portfolio{Cash: 100000, Equity: 100000}
	signals := []SignalResult{{Ticker: "AAPL", FinalScore: 0.9, Markov: markovEdge(0.55, 0.45, 0.3, 100)}}

	fixedCfg := testConfig()
	fixedPlan := PlanPortfolio(asOf, universe, portfolio, signals, fixedCfg)
	require.Len(t, fixedPlan.ProposedTrades, 1)
	assert.Equal(t, 0.02, fixedPlan.ProposedTrades[0].TargetWeight)

	kellyCfg := kellyTestConfig()
	kellyPlan := PlanPortfolio(asOf, universe, portfolio, signals, kellyCfg)
	require.Len(t, kellyPlan.ProposedTrades, 1)
	assert.Equal(t, 0.0075, kellyPlan.ProposedTrades[0].TargetWeight)
	assert.NotContains(t, kellyPlan.ProposedTrades[0].Reason, "improves returns")
}

func TestExecuteBacktestTradesSellsBeforeBuys(t *testing.T) {
	t.Parallel()

	fillDate := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.Backtest.FeeBps = 0
	cfg.Backtest.SlippageBps = 0
	cfg.Portfolio.MinTradeValue = 1

	trades, cash, positions, _, warnings := executeBacktestTrades(
		map[string][]backtestBar{
			"AAA": {{Date: fillDate, Open: 100, High: 100, Low: 100, Close: 100}},
			"BBB": {{Date: fillDate, Open: 100, High: 100, Low: 100, Close: 100}},
		},
		map[string]backtestPosition{"AAA": {Shares: 100}},
		0,
		10000,
		PortfolioPlanResult{ProposedTrades: []ProposedTrade{
			{Ticker: "BBB", Side: SideBuy, DeltaWeight: 1, Reason: "rotate in"},
			{Ticker: "AAA", Side: SideSell, DeltaWeight: -1, Reason: "rotate out"},
		}},
		fillDate.AddDate(0, 0, -1),
		fillDate,
		cfg,
	)

	require.Empty(t, warnings)
	require.Len(t, trades, 2)
	assert.Equal(t, SideSell, trades[0].Side)
	assert.Equal(t, SideBuy, trades[1].Side)
	assert.InDelta(t, 0, cash, 0.000001)
	assert.NotContains(t, positions, "AAA")
	assert.InDelta(t, 100, positions["BBB"].Shares, 0.000001)
}

func risingBacktestBars(start time.Time, end time.Time, price float64, step float64) []Bar {
	var bars []Bar
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		adjustedClose := price
		bars = append(bars, Bar{
			Date:          date,
			Open:          price - step/2,
			High:          price + step,
			Low:           price - step,
			Close:         price,
			AdjustedClose: &adjustedClose,
			Volume:        1_000_000,
		})
		price += step
	}
	return bars
}

func flatBacktestBars(start time.Time, end time.Time, price float64) []Bar {
	var bars []Bar
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		adjustedClose := price
		bars = append(bars, Bar{
			Date:          date,
			Open:          price,
			High:          price,
			Low:           price,
			Close:         price,
			AdjustedClose: &adjustedClose,
			Volume:        1_000_000,
		})
	}
	return bars
}
