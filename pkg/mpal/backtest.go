package mpal

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	defaultBacktestProfileVersion         = "qvm_v1"
	defaultSnapshotFreshnessDays          = 45
	defaultPriceCoverageToleranceDays     = 7
	backtestWarmupCalendarDays            = 430
	tradingDaysPerYear                    = 252.0
	liveYahooHistoricalFetchSource        = "yahoo_historical_fetch"
	trustedHistoricalPriceStorageDynamo   = "dynamodb"
	trustedHistoricalPriceStoragePostgres = "postgres"
)

type UntrustedBacktestError struct {
	Reasons []string
}

func (e UntrustedBacktestError) Error() string {
	if len(e.Reasons) == 0 {
		return "backtest is untrusted"
	}
	return "backtest is untrusted: " + strings.Join(e.Reasons, "; ")
}

type backtestBar struct {
	Date  time.Time
	Open  float64
	High  float64
	Low   float64
	Close float64
}

type backtestPosition struct {
	Shares float64
}

func (e Engine) BacktestRun(
	ctx context.Context,
	start time.Time,
	end time.Time,
	universe Universe,
	cfg StrategyConfig,
	ref StrategyRef,
	opts BacktestOptions,
) (BacktestResult, error) {
	opts = normalizeBacktestOptions(opts)
	result := newBacktestResult(start, end, cfg, ref)
	if err := validateBacktestInputs(start, end, universe, cfg); err != nil {
		result.TrustReasons = append(result.TrustReasons, err.Error())
		result.DataQuality.Blockers = append(result.DataQuality.Blockers, err.Error())
		return result, err
	}
	if err := EnsureApproved(cfg); err != nil {
		result.TrustReasons = append(result.TrustReasons, err.Error())
		result.DataQuality.Blockers = append(result.DataQuality.Blockers, err.Error())
		return result, err
	}

	seriesByTicker, quality := e.loadBacktestBars(ctx, universe.Tickers, start, end)
	result.DataQuality = quality
	if usesProfileFactors(cfg) {
		coverage, err := e.factorCoverage(ctx, universe.Tickers, opts.ProfileVersion)
		if err != nil {
			result.DataQuality.Blockers = append(result.DataQuality.Blockers, "factor snapshot coverage failed: "+err.Error())
		}
		result.DataQuality.Coverage = coverage
	}

	calendar := backtestCalendar(seriesByTicker, start, end)
	if usesProfileFactors(cfg) {
		result.DataQuality.ProfileSource = "ticker_factor_snapshot/postgres"
	}
	if cfg.Events.Enabled {
		result.DataQuality.EventSource = "article_insights/postgres"
	}
	if len(calendar) == 0 {
		result.DataQuality.Blockers = append(result.DataQuality.Blockers, "no trading dates in requested range")
	}
	rebalances := rebalanceDates(calendar, cfg.Portfolio.Rebalance)
	if len(rebalances) == 0 {
		result.DataQuality.Blockers = append(result.DataQuality.Blockers, "no rebalance dates in requested range")
	}
	if len(result.DataQuality.Blockers) == 0 {
		result = e.runBacktestSimulation(ctx, result, seriesByTicker, calendar, rebalances, universe, cfg, opts)
	}
	if opts.Benchmark != "" {
		result = e.attachBenchmark(ctx, result, opts.Benchmark, start, end)
	}

	result.Trusted = len(result.DataQuality.Blockers) == 0
	result.DataQuality.Trusted = result.Trusted
	result.TrustStatus = "trusted"
	if !result.Trusted {
		result.TrustStatus = "untrusted"
		result.TrustReasons = append(result.TrustReasons, result.DataQuality.Blockers...)
	}
	result = e.appendBacktestJournal(ctx, result, universe)
	if opts.TrustedOnly && !opts.AllowUntrusted && !result.Trusted {
		return result, UntrustedBacktestError{Reasons: result.TrustReasons}
	}
	return result, nil
}

func (e Engine) attachBenchmark(ctx context.Context, result BacktestResult, ticker string, start time.Time, end time.Time) BacktestResult {
	if e.Prices == nil {
		result.Warnings = append(result.Warnings, "benchmark skipped: price data source is not configured")
		return result
	}
	bars, err := e.Prices.Bars(ctx, strings.ToUpper(ticker), start, end)
	if err != nil {
		result.Warnings = append(result.Warnings, "benchmark skipped: "+err.Error())
		return result
	}
	series, blockers, warnings := adjustedBacktestBars(bars.Bars)
	result.Warnings = append(result.Warnings, warnings...)
	if len(blockers) > 0 {
		result.Warnings = append(result.Warnings, "benchmark skipped: "+strings.Join(blockers, "; "))
		return result
	}
	if len(series) < 2 {
		result.Warnings = append(result.Warnings, "benchmark skipped: insufficient bars")
		return result
	}
	first := series[0].Close
	last := series[len(series)-1].Close
	if first <= 0 {
		result.Warnings = append(result.Warnings, "benchmark skipped: invalid first price")
		return result
	}
	totalReturn := last/first - 1
	result.Benchmark = &BenchmarkResult{
		Ticker:       strings.ToUpper(ticker),
		TotalReturn:  round(totalReturn, 6),
		ExcessReturn: round(result.Metrics.TotalReturn-totalReturn, 6),
	}
	return result
}

func (e Engine) appendBacktestJournal(ctx context.Context, result BacktestResult, universe Universe) BacktestResult {
	if e.Journal == nil {
		return result
	}
	entry, err := e.Journal.Append(ctx, JournalEntry{
		ID:        RunID("jrnl", result.End),
		RunID:     result.RunID,
		Type:      JournalTypeBacktest,
		CreatedAt: time.Now().UTC(),
		Strategy:  &result.Strategy,
		Input:     map[string]any{"start": result.Start, "end": result.End, "universe": universe},
		Output:    result,
		Warnings:  result.Warnings,
	})
	if err != nil {
		result.Warnings = append(result.Warnings, "journal append failed: "+err.Error())
		return result
	}
	result.JournalEntryID = entry.ID
	return result
}

func normalizeBacktestOptions(opts BacktestOptions) BacktestOptions {
	if opts.ProfileVersion == "" {
		opts.ProfileVersion = defaultBacktestProfileVersion
	}
	if opts.SnapshotFreshnessDays <= 0 {
		opts.SnapshotFreshnessDays = defaultSnapshotFreshnessDays
	}
	if opts.AllowUntrusted {
		opts.TrustedOnly = false
	} else if !opts.TrustedOnly {
		opts.TrustedOnly = true
	}
	return opts
}

func newBacktestResult(start time.Time, end time.Time, cfg StrategyConfig, ref StrategyRef) BacktestResult {
	initial := cfg.Backtest.InitialCash
	return BacktestResult{
		RunID:       RunID("backtest", end),
		Mode:        "backtest",
		Start:       start,
		End:         end,
		Strategy:    ref,
		TrustStatus: "pending",
		Metrics: BacktestMetrics{
			InitialEquity: round(initial, 2),
			FinalEquity:   round(initial, 2),
		},
		FinalPortfolio: Portfolio{Cash: initial, Equity: initial, Positions: []Position{}},
		DataQuality:    DataQualityReport{Trusted: false},
	}
}

func validateBacktestInputs(start time.Time, end time.Time, universe Universe, cfg StrategyConfig) error {
	if validation := ValidateStrategyConfig(cfg); !validation.Valid {
		return fmt.Errorf("invalid strategy config: %s", strings.Join(validation.Errors, "; "))
	}
	if !start.Before(end) {
		return fmt.Errorf("start must be before end")
	}
	if len(NormalizeTickers(universe.Tickers)) == 0 {
		return fmt.Errorf("universe is empty")
	}
	if cfg.Backtest.InitialCash <= 0 {
		return fmt.Errorf("backtest.initial_cash must be > 0")
	}
	return nil
}

func (e Engine) loadBacktestBars(
	ctx context.Context,
	tickers []string,
	start time.Time,
	end time.Time,
) (map[string][]backtestBar, DataQualityReport) {
	seriesByTicker := make(map[string][]backtestBar, len(tickers))
	quality := DataQualityReport{Trusted: false}
	if e.Prices == nil {
		quality.Blockers = append(quality.Blockers, "price data source is not configured")
		return seriesByTicker, quality
	}

	loadStart := start.AddDate(0, 0, -backtestWarmupCalendarDays)
	loadEnd := end.AddDate(0, 0, 10)
	for _, ticker := range NormalizeTickers(tickers) {
		bars, err := e.Prices.Bars(ctx, ticker, loadStart, loadEnd)
		item := TickerDataQuality{Ticker: ticker}
		if err != nil {
			item.Blockers = append(item.Blockers, "load bars failed: "+err.Error())
			quality.Blockers = append(quality.Blockers, ticker+" load bars failed: "+err.Error())
			quality.Tickers = append(quality.Tickers, item)
			continue
		}
		quality.BarSource = firstNonEmpty(quality.BarSource, freshnessLabel(bars.Freshness))
		if blocker := trustedBacktestPriceBlocker(bars.Freshness); blocker != "" {
			item.Blockers = append(item.Blockers, blocker)
			quality.Blockers = append(quality.Blockers, ticker+" "+blocker)
		}
		series, blockers, warnings := adjustedBacktestBars(bars.Bars)
		item.Blockers = append(item.Blockers, blockers...)
		item.Warnings = append(item.Warnings, warnings...)
		for _, blocker := range blockers {
			quality.Blockers = append(quality.Blockers, ticker+" "+blocker)
		}
		coverageBlockers := priceCoverageBlockers(series, start, end, defaultPriceCoverageToleranceDays)
		item.Blockers = append(item.Blockers, coverageBlockers...)
		for _, blocker := range coverageBlockers {
			quality.Blockers = append(quality.Blockers, ticker+" "+blocker)
		}
		quality.Warnings = append(quality.Warnings, warnings...)
		item.BarCount = len(series)
		if len(series) > 0 {
			first := series[0].Date
			last := series[len(series)-1].Date
			item.FirstBarDate = &first
			item.LastBarDate = &last
		}
		seriesByTicker[ticker] = series
		quality.Tickers = append(quality.Tickers, item)
	}
	return seriesByTicker, quality
}

func adjustedBacktestBars(bars []Bar) ([]backtestBar, []string, []string) {
	sorted := append([]Bar(nil), bars...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	seen := map[string]struct{}{}
	out := make([]backtestBar, 0, len(sorted))
	var blockers []string
	var warnings []string
	for _, bar := range sorted {
		dateKey := bar.Date.UTC().Format(time.DateOnly)
		if _, ok := seen[dateKey]; ok {
			blockers = append(blockers, "has duplicate bar for "+dateKey)
			continue
		}
		seen[dateKey] = struct{}{}
		if bar.Open <= 0 || bar.High <= 0 || bar.Low <= 0 || bar.Close <= 0 {
			blockers = append(blockers, "has nonpositive OHLC for "+dateKey)
			continue
		}
		if bar.AdjustedClose == nil || *bar.AdjustedClose <= 0 {
			blockers = append(blockers, "missing positive adjusted close for "+dateKey)
			continue
		}
		factor := *bar.AdjustedClose / bar.Close
		if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
			blockers = append(blockers, "invalid adjustment factor for "+dateKey)
			continue
		}
		out = append(out, backtestBar{
			Date:  dateOnly(bar.Date),
			Open:  bar.Open * factor,
			High:  bar.High * factor,
			Low:   bar.Low * factor,
			Close: bar.Close * factor,
		})
	}
	if len(out) < 2 {
		blockers = append(blockers, "has fewer than 2 valid adjusted bars")
	}
	return out, blockers, warnings
}
