package mpal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type pendingBacktestFill struct {
	RebalanceIndex int
	SignalDate     time.Time
	FillDate       time.Time
	Plan           PortfolioPlanResult
	Equity         float64
}

func (e Engine) runBacktestSimulation(
	ctx context.Context,
	result BacktestResult,
	seriesByTicker map[string][]backtestBar,
	calendar []time.Time,
	rebalances []time.Time,
	universe Universe,
	cfg StrategyConfig,
	opts BacktestOptions,
) BacktestResult {
	cash := cfg.Backtest.InitialCash
	positions := map[string]backtestPosition{}
	rebalanceSet := dateSet(rebalances)
	calendarSet := dateSet(calendar)
	pendingFills := map[string][]pendingBacktestFill{}
	for _, date := range calendar {
		if fills := pendingFills[dateString(date)]; len(fills) > 0 {
			for _, fill := range fills {
				result, cash, positions = applyPendingBacktestFill(result, seriesByTicker, positions, cash, fill, cfg)
			}
			delete(pendingFills, dateString(date))
		}
		if _, ok := rebalanceSet[dateString(date)]; ok {
			var fill *pendingBacktestFill
			result, fill = e.applyBacktestRebalance(ctx, result, date, seriesByTicker, universe, cfg, opts, cash, positions)
			if fill != nil {
				if _, ok := calendarSet[dateString(fill.FillDate)]; ok {
					pendingFills[dateString(fill.FillDate)] = append(pendingFills[dateString(fill.FillDate)], *fill)
				} else {
					warning := "fill date " + dateString(fill.FillDate) + " is outside requested backtest window; trades not executed"
					result.Warnings = append(result.Warnings, warning)
					result.Rebalances[fill.RebalanceIndex].Warnings = append(result.Rebalances[fill.RebalanceIndex].Warnings, warning)
				}
			}
		}
		equity, exposure := markBacktestEquity(seriesByTicker, positions, cash, date)
		point := EquityPoint{
			Date:      date,
			Equity:    round(equity, 2),
			Cash:      round(cash, 2),
			Exposure:  round(exposure, 6),
			Positions: len(nonZeroPositions(positions)),
		}
		result.EquityCurve = append(result.EquityCurve, point)
	}
	applyDrawdowns(result.EquityCurve)
	if len(result.EquityCurve) > 0 {
		last := result.EquityCurve[len(result.EquityCurve)-1]
		result.FinalPortfolio = finalBacktestPortfolio(seriesByTicker, positions, cash, last.Date)
		result.Metrics = computeBacktestMetrics(cfg.Backtest.InitialCash, result.EquityCurve, result.Trades, result.Rebalances)
	}
	return result
}

func (e Engine) applyBacktestRebalance(
	ctx context.Context,
	result BacktestResult,
	date time.Time,
	seriesByTicker map[string][]backtestBar,
	universe Universe,
	cfg StrategyConfig,
	opts BacktestOptions,
	cash float64,
	positions map[string]backtestPosition,
) (BacktestResult, *pendingBacktestFill) {
	fillDate, hasFill := nextAvailableFillDate(seriesByTicker, date)
	if !hasFill {
		result.DataQuality.Blockers = append(result.DataQuality.Blockers, "missing next execution date after "+dateString(date))
		return result, nil
	}

	portfolio := portfolioFromBacktestPositions(seriesByTicker, positions, cash, date)
	signals, warnings := e.backtestSignals(ctx, seriesByTicker, universe.Tickers, date, cfg, opts)
	result.Warnings = append(result.Warnings, warnings...)
	if usesProfileFactors(cfg) {
		for _, warning := range warnings {
			if strings.Contains(warning, "factor snapshot") {
				result.DataQuality.Blockers = append(result.DataQuality.Blockers, warning)
			}
		}
	}
	if cfg.Events.Enabled {
		for _, warning := range warnings {
			if strings.Contains(warning, "event score reader") || strings.Contains(warning, "event score lookup") {
				result.DataQuality.Blockers = append(result.DataQuality.Blockers, warning)
			}
		}
	}
	plan := PlanPortfolio(date, universe, portfolio, signals, cfg)
	validation := ValidatePlan(plan, universe, portfolio, cfg)
	if !validation.Valid {
		result.DataQuality.Blockers = append(result.DataQuality.Blockers, validation.Errors...)
	}
	rebalanceIndex := len(result.Rebalances)
	result.Rebalances = append(result.Rebalances, BacktestRebalance{
		Date:       date,
		FillDate:   fillDate,
		Result:     plan.Result,
		Targets:    plan.Targets,
		Rejected:   plan.Rejected,
		Warnings:   append(warnings, validation.Warnings...),
		DataSource: result.DataQuality.BarSource,
	})
	if !validation.Valid || plan.Result != ResultTrade || len(plan.ProposedTrades) == 0 {
		return result, nil
	}
	return result, &pendingBacktestFill{
		RebalanceIndex: rebalanceIndex,
		SignalDate:     date,
		FillDate:       fillDate,
		Plan:           plan,
		Equity:         portfolio.Equity,
	}
}

func (e Engine) backtestSignals(
	ctx context.Context,
	seriesByTicker map[string][]backtestBar,
	tickers []string,
	asOf time.Time,
	cfg StrategyConfig,
	opts BacktestOptions,
) ([]SignalResult, []string) {
	var warnings []string
	var snapshots map[string]FactorSnapshot
	var eventScores map[string]EventScore
	if usesProfileFactors(cfg) {
		if e.Factors == nil {
			return nil, []string{"factor snapshot reader is not configured"}
		}
		var err error
		snapshots, err = e.Factors.SnapshotsAsOf(ctx, NormalizeTickers(tickers), asOf, opts.ProfileVersion)
		if err != nil {
			return nil, []string{"factor snapshot lookup failed: " + err.Error()}
		}
	}
	if cfg.Events.Enabled {
		if e.Events == nil {
			return nil, []string{"event score reader is not configured"}
		}
		var err error
		eventScores, err = e.Events.ScoresAsOf(ctx, NormalizeTickers(tickers), asOf, normalizedEventGuardrails(cfg).LookbackDays)
		if err != nil {
			return nil, []string{"event score lookup failed: " + err.Error()}
		}
	}
	signals := make([]SignalResult, 0, len(tickers))
	for _, ticker := range NormalizeTickers(tickers) {
		signal, ok, signalWarnings := backtestSignalForTicker(seriesByTicker[ticker], ticker, asOf, cfg, opts, snapshots, eventScores)
		warnings = append(warnings, signalWarnings...)
		if ok {
			signals = append(signals, signal)
		}
	}
	SortSignals(signals)
	return signals, warnings
}

func backtestSignalForTicker(
	series []backtestBar,
	ticker string,
	asOf time.Time,
	cfg StrategyConfig,
	opts BacktestOptions,
	snapshots map[string]FactorSnapshot,
	eventScores map[string]EventScore,
) (SignalResult, bool, []string) {
	bars := barsThrough(series, asOf)
	if len(bars) < 2 {
		return SignalResult{}, false, []string{ticker + " skipped: insufficient bars as of " + dateString(asOf)}
	}
	momentum := simpleMomentumScore(backtestBarsToCoreBars(bars))
	profileScore := 0.0
	reasons := []string{fmt.Sprintf("combined %.2f momentum and %.2f profile weights", cfg.Scoring.MomentumWeight, cfg.Scoring.ProfileWeight)}
	if usesProfileFactors(cfg) {
		snapshot, ok := snapshots[ticker]
		if !ok {
			return SignalResult{}, false, []string{ticker + " skipped: no factor snapshot as of " + dateString(asOf)}
		}
		age := int(asOf.Sub(snapshot.SnapshotDate).Hours() / 24)
		if age > opts.SnapshotFreshnessDays {
			return SignalResult{}, false, []string{ticker + " skipped: factor snapshot is stale"}
		}
		if snapshot.QVMScore != nil {
			profileScore = clamp(*snapshot.QVMScore/100, 0, 1)
		}
		if snapshot.QVMMomentumScore != nil {
			momentum = clamp(*snapshot.QVMMomentumScore/100, -1, 1)
			reasons = append(reasons, "using point-in-time QVM momentum snapshot")
		}
		reasons = append(reasons, "using point-in-time factor snapshot "+dateString(snapshot.SnapshotDate))
	}
	finalScore := cfg.Scoring.MomentumWeight*momentum + cfg.Scoring.ProfileWeight*profileScore
	var eventScore *EventScore
	if cfg.Events.Enabled {
		if score, ok := eventScores[ticker]; ok {
			eventScore = &score
			finalScore, reasons = applyEventGuardrail(finalScore, eventScore, cfg, reasons)
		}
	}
	return signalResult(ticker, asOf, momentum, ProfileScore{ProfileScore: profileScore}, finalScore, cfg, reasons, nil, nil, eventScore), true, nil
}

func applyPendingBacktestFill(
	result BacktestResult,
	seriesByTicker map[string][]backtestBar,
	positions map[string]backtestPosition,
	cash float64,
	fill pendingBacktestFill,
	cfg StrategyConfig,
) (BacktestResult, float64, map[string]backtestPosition) {
	trades, nextCash, nextPositions, turnover, tradeWarnings := executeBacktestTrades(seriesByTicker, positions, cash, fill.Equity, fill.Plan, fill.SignalDate, fill.FillDate, cfg)
	result.Warnings = append(result.Warnings, tradeWarnings...)
	result.DataQuality.Blockers = append(result.DataQuality.Blockers, tradeWarnings...)
	result.Trades = append(result.Trades, trades...)
	if fill.RebalanceIndex >= 0 && fill.RebalanceIndex < len(result.Rebalances) {
		result.Rebalances[fill.RebalanceIndex].Trades = trades
		result.Rebalances[fill.RebalanceIndex].Turnover = round(turnover, 6)
		result.Rebalances[fill.RebalanceIndex].Warnings = append(result.Rebalances[fill.RebalanceIndex].Warnings, tradeWarnings...)
	}
	return result, nextCash, nextPositions
}

func executeBacktestTrades(
	seriesByTicker map[string][]backtestBar,
	positions map[string]backtestPosition,
	cash float64,
	equity float64,
	plan PortfolioPlanResult,
	signalDate time.Time,
	fillDate time.Time,
	cfg StrategyConfig,
) ([]BacktestTrade, float64, map[string]backtestPosition, float64, []string) {
	nextPositions := cloneBacktestPositions(positions)
	var trades []BacktestTrade
	var warnings []string
	turnover := 0.0
	feeRate := cfg.Backtest.FeeBps / 10000
	slippageRate := cfg.Backtest.SlippageBps / 10000
	for _, proposed := range orderedBacktestProposedTrades(plan.ProposedTrades) {
		bar, ok := barOnDate(seriesByTicker[proposed.Ticker], fillDate)
		if !ok {
			warnings = append(warnings, proposed.Ticker+" skipped: missing execution price")
			continue
		}
		baseValue := math.Abs(proposed.DeltaWeight) * equity
		if baseValue <= 0 {
			continue
		}
		price := bar.Open
		side := proposed.Side
		if side == SideBuy {
			price *= 1 + slippageRate
			value := math.Min(baseValue, cash/(1+feeRate))
			if value < cfg.Portfolio.MinTradeValue {
				continue
			}
			shares := value / price
			fee := value * feeRate
			cash -= value + fee
			pos := nextPositions[proposed.Ticker]
			pos.Shares += shares
			nextPositions[proposed.Ticker] = pos
			trades = append(trades, newBacktestTrade(fillDate, signalDate, proposed.Ticker, side, shares, price, value, fee, cash, cfg.Backtest.SlippageBps, proposed.Reason))
			turnover += math.Abs(proposed.DeltaWeight)
		} else if side == SideSell {
			price *= 1 - slippageRate
			pos := nextPositions[proposed.Ticker]
			shares := math.Min(pos.Shares, baseValue/price)
			if shares <= 0 {
				continue
			}
			value := shares * price
			if value < cfg.Portfolio.MinTradeValue {
				continue
			}
			fee := value * feeRate
			pos.Shares -= shares
			if pos.Shares <= 0.000001 {
				delete(nextPositions, proposed.Ticker)
			} else {
				nextPositions[proposed.Ticker] = pos
			}
			cash += value - fee
			trades = append(trades, newBacktestTrade(fillDate, signalDate, proposed.Ticker, side, shares, price, value, fee, cash, cfg.Backtest.SlippageBps, proposed.Reason))
			turnover += math.Abs(proposed.DeltaWeight)
		}
	}
	return trades, cash, nextPositions, turnover, warnings
}

func orderedBacktestProposedTrades(trades []ProposedTrade) []ProposedTrade {
	ordered := append([]ProposedTrade(nil), trades...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return tradeExecutionPriority(ordered[i].Side) < tradeExecutionPriority(ordered[j].Side)
	})
	return ordered
}

func tradeExecutionPriority(side string) int {
	if side == SideSell {
		return 0
	}
	if side == SideBuy {
		return 1
	}
	return 2
}

func newBacktestTrade(date time.Time, signalDate time.Time, ticker string, side string, shares float64, price float64, value float64, fee float64, cash float64, slippageBps float64, reason string) BacktestTrade {
	return BacktestTrade{
		Date:           date,
		SignalDate:     signalDate,
		Ticker:         ticker,
		Side:           side,
		Shares:         round(shares, 8),
		Price:          round(price, 4),
		GrossValue:     round(value, 2),
		Fee:            round(fee, 2),
		SlippageBps:    slippageBps,
		CashAfterTrade: round(cash, 2),
		Reason:         reason,
	}
}

func computeBacktestMetrics(initial float64, curve []EquityPoint, trades []BacktestTrade, rebalances []BacktestRebalance) BacktestMetrics {
	if initial <= 0 || len(curve) == 0 {
		return BacktestMetrics{}
	}
	final := curve[len(curve)-1].Equity
	totalReturn := final/initial - 1
	years := math.Max(1/tradingDaysPerYear, float64(len(curve))/tradingDaysPerYear)
	cagr := math.Pow(final/initial, 1/years) - 1
	returns := dailyReturns(curve)
	vol := stddev(returns) * math.Sqrt(tradingDaysPerYear)
	sharpe := 0.0
	if vol != 0 {
		sharpe = (average(returns) * tradingDaysPerYear) / vol
	}
	sortino := sortinoRatio(returns)
	maxDrawdown := minDrawdown(curve)
	calmar := 0.0
	if maxDrawdown != 0 {
		calmar = cagr / math.Abs(maxDrawdown)
	}
	avgTurnover := 0.0
	for _, rebalance := range rebalances {
		avgTurnover += rebalance.Turnover
	}
	if len(rebalances) > 0 {
		avgTurnover /= float64(len(rebalances))
	}
	cashDrag := averageCashWeight(curve)
	return BacktestMetrics{
		InitialEquity:        round(initial, 2),
		FinalEquity:          round(final, 2),
		TotalReturn:          round(totalReturn, 6),
		CAGR:                 round(cagr, 6),
		AnnualizedVolatility: round(vol, 6),
		Sharpe:               round(sharpe, 6),
		Sortino:              round(sortino, 6),
		MaxDrawdown:          round(maxDrawdown, 6),
		Calmar:               round(calmar, 6),
		CashDrag:             round(cashDrag, 6),
		TradeCount:           len(trades),
		RebalanceCount:       len(rebalances),
		AverageTurnover:      round(avgTurnover, 6),
	}
}

func (e Engine) factorCoverage(ctx context.Context, tickers []string, profileVersion string) ([]FactorSnapshotCoverage, error) {
	if e.Factors == nil {
		return nil, errors.New("factor snapshot reader is not configured")
	}
	return e.Factors.Coverage(ctx, NormalizeTickers(tickers), profileVersion)
}

func usesProfileFactors(cfg StrategyConfig) bool {
	return cfg.Scoring.ProfileWeight > 0
}
