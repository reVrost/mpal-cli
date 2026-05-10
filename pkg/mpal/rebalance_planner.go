package mpal

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	listingRegionUS  = "US"
	listingRegionASX = "ASX"

	defaultListingRegionTiltMinExposure = 0.60
	defaultListingRegionTiltTolerance   = 0.10
)

type rebalancePlanner struct {
	asOf              time.Time
	portfolio         Portfolio
	cfg               StrategyConfig
	allowedNew        map[string]struct{}
	current           map[string]float64
	targetWeights     map[string]float64
	signalByTicker    map[string]SignalResult
	ranked            []SignalResult
	remainingTurnover float64
	cashWeight        float64
	newPositions      int
	targets           []TargetPosition
	trades            []ProposedTrade
	rejected          []RejectedTicker
	rejectedKeys      map[string]struct{}
	warnings          []string
}

func newRebalancePlanner(asOf time.Time, universe Universe, portfolio Portfolio, signals []SignalResult, cfg StrategyConfig) *rebalancePlanner {
	current := CurrentWeights(portfolio)
	targetWeights := make(map[string]float64, len(current))
	for ticker, weight := range current {
		targetWeights[ticker] = weight
	}
	signalByTicker := make(map[string]SignalResult, len(signals))
	ranked := append([]SignalResult{}, signals...)
	for i := range ranked {
		ranked[i].Ticker = strings.ToUpper(strings.TrimSpace(ranked[i].Ticker))
	}
	SortSignals(ranked)
	for _, signal := range ranked {
		if signal.Ticker == "" {
			continue
		}
		signalByTicker[signal.Ticker] = signal
	}
	return &rebalancePlanner{
		asOf:              asOf,
		portfolio:         portfolio,
		cfg:               cfg,
		allowedNew:        allowedTickerSet(universe.Tickers),
		current:           current,
		targetWeights:     targetWeights,
		signalByTicker:    signalByTicker,
		ranked:            ranked,
		remainingTurnover: math.Max(0, cfg.Risk.TurnoverBudgetPct),
		cashWeight:        cashWeight(portfolio),
		rejectedKeys:      map[string]struct{}{},
	}
}

func (p *rebalancePlanner) planReductions() {
	for _, ticker := range p.orderedHoldings() {
		weight := p.targetWeights[ticker]
		if weight <= 0 || p.remainingTurnover <= 0 {
			continue
		}
		signal, scored := p.signalByTicker[ticker]
		if !scored {
			p.handleUnscoredHolding(ticker, weight)
			continue
		}
		if p.protectsUnusableScore(signal) {
			p.warnings = AppendWarnings(p.warnings, ticker+" protected: no usable score")
			continue
		}
		if signal.EventVeto {
			p.warnings = AppendWarnings(p.warnings, ticker+" flagged for review: negative scored event")
		}
		if signal.FinalScore < p.cfg.Scoring.MinHoldScore {
			p.reduceWeakHolding(ticker, weight, signal.FinalScore)
			continue
		}
		if weight > p.cfg.Portfolio.MaxPositionPct {
			target := weight - minFloat(weight-p.cfg.Portfolio.MaxPositionPct, p.cfg.Risk.MaxSingleTradePct, p.remainingTurnover)
			p.addTrade(ticker, SideSell, TradeIntentTrim, target, "trim overweight position toward max position size")
		}
	}
}

func (p *rebalancePlanner) handleUnscoredHolding(ticker string, weight float64) {
	if p.cfg.Risk.ProtectUnscoredHoldings {
		p.warnings = AppendWarnings(p.warnings, ticker+" protected: no usable score")
		return
	}
	target := weight - minFloat(weight, p.cfg.Risk.MaxSingleTradePct, p.remainingTurnover)
	p.addTrade(ticker, SideSell, TradeIntentReduce, target, "reduce unscored holding because protection is disabled")
}

func (p *rebalancePlanner) protectsUnusableScore(signal SignalResult) bool {
	if !p.cfg.Risk.ProtectUnscoredHoldings || p.cfg.Scoring.ProfileWeight <= 0 {
		return false
	}
	for _, warning := range signal.Warnings {
		if strings.Contains(strings.ToLower(warning), "profile unavailable") {
			return true
		}
	}
	for _, freshness := range signal.Freshness {
		if freshness.Source == "ticker_profile" && strings.Contains(strings.ToLower(freshness.Warning), "missing profile") {
			return true
		}
	}
	return false
}

func (p *rebalancePlanner) reduceWeakHolding(ticker string, weight float64, score float64) {
	target := weight - minFloat(weight, p.cfg.Risk.MaxSingleTradePct, p.remainingTurnover)
	intent := TradeIntentReduce
	if target <= 0 {
		target = 0
		intent = TradeIntentExitCandidate
	}
	reason := fmt.Sprintf("reduce holding below hold threshold %.2f; score %.2f", p.cfg.Scoring.MinHoldScore, score)
	p.addTrade(ticker, SideSell, intent, target, reason)
}

func (p *rebalancePlanner) planStarters() {
	for _, signal := range p.orderedStarterCandidates() {
		if p.remainingTurnover <= 0 || p.availableCash() <= 0 {
			return
		}
		if p.newPositions >= p.cfg.Risk.MaxNewPositionsPerRun {
			p.reject(signal.Ticker, "max_new_positions_per_run reached")
			continue
		}
		if p.activePositions() >= p.cfg.Portfolio.MaxPositions {
			p.reject(signal.Ticker, "max_positions reached")
			continue
		}
		target := minFloat(p.cfg.Risk.StarterPositionPct, p.cfg.Risk.MaxSingleTradePct, p.cfg.Portfolio.MaxPositionPct, p.remainingTurnover, p.availableCash())
		if p.addTrade(signal.Ticker, SideBuy, TradeIntentStarter, target, p.starterReason(signal)) {
			p.newPositions++
		} else {
			p.reject(signal.Ticker, "insufficient funding or below min trade value")
		}
	}
}

func (p *rebalancePlanner) orderedStarterCandidates() []SignalResult {
	candidates := make([]SignalResult, 0, len(p.ranked))
	for _, signal := range p.ranked {
		if p.current[signal.Ticker] > 0 {
			continue
		}
		if signal.FinalScore < p.cfg.Scoring.MinBuyScore {
			p.reject(signal.Ticker, "below min_buy_score")
			continue
		}
		if signal.EventVeto {
			p.reject(signal.Ticker, "event veto")
			continue
		}
		if !tickerAllowed(p.allowedNew, signal.Ticker) {
			p.reject(signal.Ticker, "ticker not in universe")
			continue
		}
		candidates = append(candidates, signal)
	}
	preferred := p.activeListingRegionTilt()
	if preferred == "" {
		return candidates
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		leftScore := listingRegionTiltScore(candidates[i], preferred)
		rightScore := listingRegionTiltScore(candidates[j], preferred)
		if leftScore == rightScore {
			leftPreferred := listingRegion(candidates[i].Ticker) == preferred
			rightPreferred := listingRegion(candidates[j].Ticker) == preferred
			if leftPreferred != rightPreferred {
				return leftPreferred
			}
			if candidates[i].FinalScore == candidates[j].FinalScore {
				return candidates[i].Ticker < candidates[j].Ticker
			}
			return candidates[i].FinalScore > candidates[j].FinalScore
		}
		return leftScore > rightScore
	})
	return candidates
}

func (p *rebalancePlanner) activeListingRegionTilt() string {
	preferred := normalizeListingRegion(p.cfg.Portfolio.ListingRegionTilt)
	if preferred == "" || preferredRegionExposure(p.targetWeights, preferred) >= defaultListingRegionTiltMinExposure {
		return ""
	}
	return preferred
}

func (p *rebalancePlanner) starterReason(signal SignalResult) string {
	reason := "starter position from top-ranked score above buy threshold"
	preferred := p.activeListingRegionTilt()
	if preferred == "" || listingRegion(signal.Ticker) != preferred {
		return reason
	}
	for _, other := range p.ranked {
		if other.Ticker == signal.Ticker || p.current[other.Ticker] > 0 || !tickerAllowed(p.allowedNew, other.Ticker) || other.FinalScore < p.cfg.Scoring.MinBuyScore || other.EventVeto {
			continue
		}
		if listingRegion(other.Ticker) == preferred {
			continue
		}
		if other.FinalScore > signal.FinalScore && other.FinalScore-signal.FinalScore <= defaultListingRegionTiltTolerance {
			return fmt.Sprintf("%s; preferred by listing-region tilt toward %s within %.2f score tolerance while %s exposure is below %.0f%%", reason, preferred, defaultListingRegionTiltTolerance, preferred, defaultListingRegionTiltMinExposure*100)
		}
	}
	return reason
}

func (p *rebalancePlanner) planTopUps() {
	for _, signal := range p.ranked {
		if p.remainingTurnover <= 0 || p.availableCash() <= 0 {
			return
		}
		weight := p.targetWeights[signal.Ticker]
		if p.current[signal.Ticker] <= 0 || weight <= 0 || signal.FinalScore < p.cfg.Scoring.MinBuyScore || weight >= p.cfg.Portfolio.MaxPositionPct {
			continue
		}
		if signal.EventVeto {
			continue
		}
		target := weight + minFloat(p.cfg.Portfolio.MaxPositionPct-weight, p.cfg.Risk.MaxSingleTradePct, p.remainingTurnover, p.availableCash())
		p.addTrade(signal.Ticker, SideBuy, TradeIntentTopUp, target, "top up existing holding with score above buy threshold")
	}
}

func (p *rebalancePlanner) addTrade(ticker string, side string, intent string, targetWeight float64, reason string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	previousTarget := p.targetWeights[ticker]
	turnoverDelta := math.Abs(targetWeight - previousTarget)
	if turnoverDelta <= 0 || turnoverDelta > p.remainingTurnover {
		return false
	}
	estimated := turnoverDelta * p.portfolio.Equity
	if estimated < p.cfg.Portfolio.MinTradeValue {
		return false
	}
	if side == SideBuy && turnoverDelta > p.availableCash() {
		return false
	}
	p.targetWeights[ticker] = targetWeight
	p.remainingTurnover -= turnoverDelta
	if side == SideBuy {
		p.cashWeight -= turnoverDelta
	} else if side == SideSell {
		p.cashWeight += turnoverDelta
	}
	currentWeight := p.current[ticker]
	delta := targetWeight - currentWeight
	p.targets = append(p.targets, TargetPosition{Ticker: ticker, TargetWeight: round(targetWeight, 6), Reason: reason})
	p.trades = append(p.trades, ProposedTrade{
		Ticker:         ticker,
		Side:           side,
		Intent:         intent,
		CurrentWeight:  round(currentWeight, 6),
		TargetWeight:   round(targetWeight, 6),
		DeltaWeight:    round(delta, 6),
		EstimatedValue: round(math.Abs(delta)*p.portfolio.Equity, 2),
		Reason:         reason,
	})
	return true
}

func (p *rebalancePlanner) result() PortfolioPlanResult {
	sort.Slice(p.trades, func(i, j int) bool {
		if tradeExecutionPriority(p.trades[i].Side) == tradeExecutionPriority(p.trades[j].Side) {
			return p.trades[i].Ticker < p.trades[j].Ticker
		}
		return tradeExecutionPriority(p.trades[i].Side) < tradeExecutionPriority(p.trades[j].Side)
	})
	sort.Slice(p.targets, func(i, j int) bool { return p.targets[i].Ticker < p.targets[j].Ticker })
	result := ResultNoTrade
	summary := "No executable portfolio changes under current score, funding, and risk policy."
	if len(p.trades) > 0 {
		result = ResultTrade
		used := p.cfg.Risk.TurnoverBudgetPct - p.remainingTurnover
		summary = fmt.Sprintf("%d proposed trades using %.1f%% of %.1f%% turnover budget.", len(p.trades), used*100, p.cfg.Risk.TurnoverBudgetPct*100)
	}
	return PortfolioPlanResult{
		AsOf:           p.asOf,
		Result:         result,
		Targets:        p.targets,
		ProposedTrades: p.trades,
		Rejected:       p.rejected,
		Summary:        summary,
		Warnings:       p.warnings,
	}
}

func (p *rebalancePlanner) orderedHoldings() []string {
	tickers := make([]string, 0, len(p.current))
	for ticker := range p.current {
		tickers = append(tickers, ticker)
	}
	sort.Slice(tickers, func(i, j int) bool {
		left, leftOK := p.signalByTicker[tickers[i]]
		right, rightOK := p.signalByTicker[tickers[j]]
		if leftOK != rightOK {
			return !leftOK
		}
		if left.FinalScore == right.FinalScore {
			return tickers[i] < tickers[j]
		}
		return left.FinalScore < right.FinalScore
	})
	return tickers
}

func (p *rebalancePlanner) activePositions() int {
	count := 0
	for _, weight := range p.targetWeights {
		if weight > 0.000001 {
			count++
		}
	}
	return count
}

func (p *rebalancePlanner) availableCash() float64 {
	return math.Max(0, p.cashWeight-p.cfg.Risk.CashBufferPct)
}

func (p *rebalancePlanner) reject(ticker string, reason string) {
	key := strings.ToUpper(ticker) + "\x00" + reason
	if _, ok := p.rejectedKeys[key]; ok {
		return
	}
	p.rejectedKeys[key] = struct{}{}
	p.rejected = append(p.rejected, RejectedTicker{Ticker: strings.ToUpper(ticker), Reason: reason})
}

func allowedTickerSet(tickers []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(tickers))
	for _, ticker := range NormalizeTickers(tickers) {
		allowed[ticker] = struct{}{}
	}
	return allowed
}

func tickerAllowed(allowed map[string]struct{}, ticker string) bool {
	_, ok := allowed[strings.ToUpper(strings.TrimSpace(ticker))]
	return ok
}

func listingRegionTiltScore(signal SignalResult, preferred string) float64 {
	score := signal.FinalScore
	if listingRegion(signal.Ticker) == preferred {
		score += defaultListingRegionTiltTolerance
	}
	return score
}

func preferredRegionExposure(weights map[string]float64, preferred string) float64 {
	total := 0.0
	preferredTotal := 0.0
	for ticker, weight := range weights {
		if weight <= 0 {
			continue
		}
		total += weight
		if listingRegion(ticker) == preferred {
			preferredTotal += weight
		}
	}
	if total <= 0 {
		return 0
	}
	return preferredTotal / total
}

func listingRegion(ticker string) string {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if strings.HasSuffix(ticker, ".AX") {
		return listingRegionASX
	}
	return listingRegionUS
}

func normalizeListingRegion(region string) string {
	return strings.ToUpper(strings.TrimSpace(region))
}

func cashWeight(portfolio Portfolio) float64 {
	if portfolio.Equity <= 0 {
		return 0
	}
	return portfolio.Cash / portfolio.Equity
}

func minFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return math.Max(0, minimum)
}
