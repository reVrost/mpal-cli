package mpal

import (
	"strings"
	"time"
)

// TradeReviewStartInputFromStrategyRun converts a completed strategy run into
// the deterministic first-pass review row that the CLI auto-journals.
func TradeReviewStartInputFromStrategyRun(run StrategyRunResult, cfg StrategyConfig, configText string, universe Universe, fallbackAsOf time.Time, portfolios ...Portfolio) TradeReviewStartInput {
	asOf := run.AsOf
	if asOf.IsZero() {
		asOf = fallbackAsOf
	}
	strategyID := strings.TrimSpace(run.Strategy.ID)
	if strategyID == "" {
		strategyID = cfg.ID
	}
	executionResult := firstNonEmpty(run.ExecutionResult, run.Result)
	if executionResult == "" {
		executionResult = firstNonEmpty(run.BaselinePlan.Result, ResultNoTrade)
	}

	warnings := append([]string{}, run.Warnings...)
	warnings = append(warnings, run.Validation.Warnings...)
	warnings = append(warnings, run.Validation.Errors...)
	warnings = append(warnings, run.BaselinePlan.Warnings...)

	return TradeReviewStartInput{
		AsOf:               dateString(asOf),
		StrategyID:         strategyID,
		StrategyConfigText: strings.TrimSpace(configText),
		PortfolioScope:     "custom",
		UniverseTickers:    NormalizeTickers(universe.Tickers),
		ExecutionResult:    executionResult,
		AgentSummary:       run.Summary,
		WarningsText:       strings.Join(nonEmptyStrings(warnings), "\n"),
		Positions:          strategyRunReviewPositions(run, cfg, firstOptionalPortfolio(portfolios)),
	}
}

func strategyRunReviewPositions(run StrategyRunResult, cfg StrategyConfig, portfolio Portfolio) []TradeReviewPositionInput {
	signalsByTicker := make(map[string]SignalResult, len(run.Signals))
	for _, signal := range run.Signals {
		signalsByTicker[normalizeJournalTicker(signal.Ticker)] = signal
	}
	sharePricesByTicker := portfolioSharePrices(portfolio)

	seen := map[string]bool{}
	positions := make([]TradeReviewPositionInput, 0, len(run.BaselinePlan.ProposedTrades)+len(run.BaselinePlan.Rejected))
	for _, trade := range run.BaselinePlan.ProposedTrades {
		ticker := normalizeJournalTicker(trade.Ticker)
		seen[ticker] = true
		position := TradeReviewPositionInput{
			Ticker:              ticker,
			ModelBucket:         "proposed",
			ModelIntent:         trade.Intent,
			ModelWeight:         reviewFloatPtr(trade.TargetWeight),
			ModelDeltaWeight:    reviewFloatPtr(trade.DeltaWeight),
			ModelEstimatedValue: reviewFloatPtr(trade.EstimatedValue),
			ModelReason:         strings.TrimSpace(trade.Reason),
		}
		signal, hasSignal := signalsByTicker[ticker]
		if hasSignal {
			position.ModelScore = reviewFloatPtr(signal.FinalScore)
			if position.ModelIntent == "" {
				position.ModelIntent = signal.ActionHint
			}
			if position.ModelReason == "" {
				position.ModelReason = strings.Join(signal.Reasons, "; ")
			}
			applySignalKellyContext(&position, signal, cfg)
		}
		if price, ok := sharePricesByTicker[ticker]; ok {
			position.ModelSharePrice = reviewFloatPtr(price)
		} else if hasSignal && signal.CurrentPrice != nil && *signal.CurrentPrice > 0 {
			position.ModelSharePrice = reviewFloatPtr(*signal.CurrentPrice)
		}
		applySizingDecision(&position, trade.Sizing)
		positions = append(positions, position)
	}

	for _, rejected := range run.BaselinePlan.Rejected {
		ticker := normalizeJournalTicker(rejected.Ticker)
		if seen[ticker] {
			continue
		}
		seen[ticker] = true
		position := TradeReviewPositionInput{
			Ticker:      ticker,
			ModelBucket: "rejected",
			ModelReason: strings.TrimSpace(rejected.Reason),
		}
		signal, hasSignal := signalsByTicker[ticker]
		if hasSignal {
			position.ModelScore = reviewFloatPtr(signal.FinalScore)
			position.ModelIntent = signal.ActionHint
			if position.ModelReason == "" {
				position.ModelReason = strings.Join(signal.Reasons, "; ")
			}
			applySignalKellyContext(&position, signal, cfg)
		}
		if price, ok := sharePricesByTicker[ticker]; ok {
			position.ModelSharePrice = reviewFloatPtr(price)
		} else if hasSignal && signal.CurrentPrice != nil && *signal.CurrentPrice > 0 {
			position.ModelSharePrice = reviewFloatPtr(*signal.CurrentPrice)
		}
		positions = append(positions, position)
	}

	return positions
}

func firstOptionalPortfolio(portfolios []Portfolio) Portfolio {
	if len(portfolios) == 0 {
		return Portfolio{}
	}
	return portfolios[0]
}

func portfolioSharePrices(portfolio Portfolio) map[string]float64 {
	prices := make(map[string]float64, len(portfolio.Positions))
	for _, position := range portfolio.Positions {
		ticker := normalizeJournalTicker(position.Ticker)
		if ticker == "" {
			continue
		}
		price := position.CurrentPrice
		if price == 0 && position.Shares > 0 {
			price = position.MarketValue / position.Shares
		}
		if price > 0 {
			prices[ticker] = price
		}
	}
	return prices
}

func applySignalKellyContext(position *TradeReviewPositionInput, signal SignalResult, cfg StrategyConfig) {
	if position == nil || signal.RawKelly == nil {
		return
	}
	if position.SizingMethod == "" {
		position.SizingMethod = SizingMethodFractionalKelly
	}
	if position.RawKelly == nil {
		position.RawKelly = reviewFloatPtr(signal.RawKelly.RawKelly)
	}
	applied := applyRiskProfileDefaultsCopy(cfg)
	decision, _ := fractionalKellyDecision(signal, normalizeSizingConfig(applied.Risk))
	if position.FractionalKelly == nil {
		position.FractionalKelly = reviewFloatPtr(decision.FractionalKelly)
	}
	if position.KellyTargetWeight == nil {
		position.KellyTargetWeight = reviewFloatPtr(decision.KellyTargetWeight)
	}
	if position.BindingConstraint == "" {
		position.BindingConstraint = decision.BindingConstraint
	}
	if position.CalibrationStatus == "" {
		position.CalibrationStatus = firstNonEmpty(signal.RawKelly.CalibrationStatus, decision.CalibrationStatus)
	}
}

func applySizingDecision(position *TradeReviewPositionInput, sizing *SizingDecision) {
	if position == nil || sizing == nil {
		return
	}
	position.SizingMethod = sizing.Method
	if sizing.Method == SizingMethodFractionalKelly || sizing.RawKelly != 0 || sizing.FractionalKelly != 0 || sizing.KellyTargetWeight != 0 {
		position.RawKelly = reviewFloatPtr(sizing.RawKelly)
		position.FractionalKelly = reviewFloatPtr(sizing.FractionalKelly)
		position.KellyTargetWeight = reviewFloatPtr(sizing.KellyTargetWeight)
	}
	position.FinalTargetWeight = reviewFloatPtr(sizing.FinalTargetWeight)
	position.BindingConstraint = sizing.BindingConstraint
	position.CalibrationStatus = sizing.CalibrationStatus
}

func reviewFloatPtr(value float64) *float64 {
	return &value
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
