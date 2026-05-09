package mpal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

type PriceData interface {
	Bars(ctx context.Context, ticker string, start, end time.Time) (BarsResult, error)
}

type ProfileScorer interface {
	Score(ctx context.Context, ticker string, asOf time.Time) (ProfileScore, error)
}

type FactorSnapshotReader interface {
	SnapshotsAsOf(ctx context.Context, tickers []string, asOf time.Time, profileVersion string) (map[string]FactorSnapshot, error)
	Coverage(ctx context.Context, tickers []string, profileVersion string) ([]FactorSnapshotCoverage, error)
}

type EventScoreReader interface {
	ScoresAsOf(ctx context.Context, tickers []string, asOf time.Time, lookbackDays int) (map[string]EventScore, error)
}

type PortfolioReader interface {
	Snapshot(ctx context.Context, userID string) (Portfolio, error)
	Watchlist(ctx context.Context, userID string) (Universe, error)
}

type JournalStore interface {
	Append(ctx context.Context, entry JournalEntry) (JournalEntry, error)
	List(ctx context.Context, limit int) ([]JournalEntry, error)
	Get(ctx context.Context, id string) (JournalEntry, error)
}

type Engine struct {
	Prices   PriceData
	Profiles ProfileScorer
	Factors  FactorSnapshotReader
	Events   EventScoreReader
	Journal  JournalStore
}

func (e Engine) SignalScore(
	ctx context.Context,
	ticker string,
	asOf time.Time,
	cfg StrategyConfig,
) (SignalResult, error) {
	start := asOf.AddDate(0, 0, -365)
	bars, err := e.Prices.Bars(ctx, ticker, start, asOf)
	if err != nil {
		return SignalResult{}, fmt.Errorf("load bars for %s: %w", ticker, err)
	}
	profile, err := e.Profiles.Score(ctx, ticker, asOf)
	if err != nil {
		profile = ProfileScore{
			Ticker:       ticker,
			AsOf:         asOf,
			ProfileScore: 0,
			ScoreSource:  "neutral_missing_profile",
			Warnings:     []string{"profile unavailable: " + err.Error()},
		}
	}

	momentum := simpleMomentumScore(bars.Bars)
	if profile.MomentumScore != nil {
		momentum = *profile.MomentumScore
	}
	finalScore := cfg.Scoring.MomentumWeight*momentum + cfg.Scoring.ProfileWeight*profile.ProfileScore
	reasons := []string{fmt.Sprintf("combined %.2f momentum and %.2f profile weights", cfg.Scoring.MomentumWeight, cfg.Scoring.ProfileWeight)}
	warnings := append([]string{}, bars.Warnings...)
	warnings = append(warnings, profile.Warnings...)
	var eventScore *EventScore
	if cfg.Events.Enabled {
		var eventWarnings []string
		eventScore, eventWarnings = e.eventScoreForSignal(ctx, ticker, asOf, cfg)
		warnings = append(warnings, eventWarnings...)
		finalScore, reasons = applyEventGuardrail(finalScore, eventScore, cfg, reasons)
		if eventScore != nil && eventScore.Score <= normalizedEventGuardrails(cfg).VetoScore {
			warnings = append(warnings, fmt.Sprintf("%s event veto active from latest scored article", strings.ToUpper(ticker)))
		}
	}

	return signalResult(ticker, asOf, momentum, profile, finalScore, cfg, reasons, warnings, bars.Freshness, eventScore), nil
}

func (e Engine) eventScoreForSignal(ctx context.Context, ticker string, asOf time.Time, cfg StrategyConfig) (*EventScore, []string) {
	if e.Events == nil {
		return nil, []string{"event guardrail skipped: event score reader is not configured"}
	}
	events := normalizedEventGuardrails(cfg)
	scores, err := e.Events.ScoresAsOf(ctx, []string{ticker}, asOf, events.LookbackDays)
	if err != nil {
		return nil, []string{"event guardrail skipped: " + err.Error()}
	}
	score, ok := scores[strings.ToUpper(strings.TrimSpace(ticker))]
	if !ok {
		return nil, nil
	}
	return &score, nil
}

func applyEventGuardrail(finalScore float64, eventScore *EventScore, cfg StrategyConfig, reasons []string) (float64, []string) {
	if eventScore == nil {
		return finalScore, reasons
	}
	events := normalizedEventGuardrails(cfg)
	switch {
	case eventScore.Score <= events.VetoScore:
		reasons = append(reasons, fmt.Sprintf("event veto score %.2f <= %.2f", eventScore.Score, events.VetoScore))
	case eventScore.Score >= events.BoostScore:
		finalScore = math.Min(1, finalScore+events.BoostAmount)
		reasons = append(reasons, fmt.Sprintf("event boost %.2f from latest scored article", events.BoostAmount))
	}
	return finalScore, reasons
}

func signalResult(
	ticker string,
	asOf time.Time,
	momentum float64,
	profile ProfileScore,
	finalScore float64,
	cfg StrategyConfig,
	reasons []string,
	warnings []string,
	barFreshness *Freshness,
	eventScore *EventScore,
) SignalResult {
	eventVeto := false
	if cfg.Events.Enabled && eventScore != nil {
		eventVeto = eventScore.Score <= normalizedEventGuardrails(cfg).VetoScore
	}
	actionHint := "NO_TRADE"
	if !eventVeto && finalScore >= cfg.Scoring.MinBuyScore {
		actionHint = "BUY"
	} else if finalScore >= cfg.Scoring.MinHoldScore {
		actionHint = "HOLD"
	}

	freshness := make([]Freshness, 0, 3)
	if barFreshness != nil {
		freshness = append(freshness, *barFreshness)
	}
	if profile.Freshness != nil {
		freshness = append(freshness, *profile.Freshness)
	}
	var score *float64
	var confidence *float64
	if eventScore != nil {
		scoreValue := round(eventScore.Score, 6)
		score = &scoreValue
		confidence = eventScore.Confidence
		publishedAt := eventScore.PublishedAt
		scoredAt := eventScore.ScoredAt
		freshness = append(freshness, Freshness{
			Source:      "article_insights",
			Provider:    "marketpal",
			Storage:     "postgres",
			AsOf:        &publishedAt,
			UpdatedAt:   &scoredAt,
			Stale:       false,
			Description: "latest scored article event",
		})
	}
	return SignalResult{
		Ticker:               strings.ToUpper(ticker),
		AsOf:                 asOf,
		MomentumScore:        round(momentum, 6),
		ProfileScore:         round(profile.ProfileScore, 6),
		EventScore:           score,
		EventScoreConfidence: confidence,
		FinalScore:           round(finalScore, 6),
		ActionHint:           actionHint,
		EventVeto:            eventVeto,
		Reasons:              reasons,
		Warnings:             warnings,
		Freshness:            freshness,
	}
}

func (e Engine) RankSignals(
	ctx context.Context,
	tickers []string,
	asOf time.Time,
	cfg StrategyConfig,
) ([]SignalResult, []string) {
	var signals []SignalResult
	var warnings []string
	for _, ticker := range NormalizeTickers(tickers) {
		signal, err := e.SignalScore(ctx, ticker, asOf, cfg)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s signal failed: %v", ticker, err))
			continue
		}
		signals = append(signals, signal)
	}
	SortSignals(signals)
	return signals, warnings
}

func (e Engine) StrategyRun(
	ctx context.Context,
	asOf time.Time,
	universe Universe,
	portfolio Portfolio,
	cfg StrategyConfig,
	ref StrategyRef,
) (StrategyRunResult, error) {
	if validation := ValidateStrategyConfig(cfg); !validation.Valid {
		return StrategyRunResult{}, fmt.Errorf("invalid strategy config: %s", strings.Join(validation.Errors, "; "))
	}
	if err := EnsureApproved(cfg); err != nil {
		return StrategyRunResult{}, err
	}
	if len(universe.Tickers) == 0 {
		return StrategyRunResult{}, fmt.Errorf("universe is empty")
	}
	signals, warnings := e.RankSignals(ctx, planningTickers(universe, portfolio), asOf, cfg)
	plan := PlanPortfolio(asOf, universe, portfolio, signals, cfg)
	validation := ValidatePlan(plan, universe, portfolio, cfg)
	executionResult := executableResult(plan.Result, validation)
	modelResult := signalModelResult(signals, cfg)
	if !validation.Valid {
		for _, validationErr := range validation.Errors {
			warnings = AppendWarnings(warnings, "baseline plan failed validation: "+validationErr)
		}
	}
	result := StrategyRunResult{
		RunID:           RunID("strategy_run", asOf),
		Mode:            "strategy_run",
		AsOf:            asOf,
		Strategy:        ref,
		Result:          executionResult,
		ModelResult:     modelResult,
		ExecutionResult: executionResult,
		Summary:         plan.Summary,
		Signals:         signals,
		BaselinePlan:    plan,
		Validation:      validation,
		Warnings:        warnings,
	}
	if e.Journal != nil {
		entry, err := e.Journal.Append(ctx, JournalEntry{
			ID:        RunID("jrnl", asOf),
			RunID:     result.RunID,
			Type:      JournalTypeBaselinePlan,
			CreatedAt: time.Now().UTC(),
			AsOf:      &asOf,
			Strategy:  &ref,
			Input:     map[string]any{"universe": universe, "portfolio": portfolio},
			Output:    result,
			Warnings:  result.Warnings,
		})
		if err != nil {
			result.Warnings = append(result.Warnings, "journal append failed: "+err.Error())
		} else {
			result.JournalEntryID = entry.ID
		}
	}
	return result, nil
}

func executableResult(planResult string, validation ValidationResult) string {
	if planResult != ResultTrade {
		return ResultNoTrade
	}
	if !validation.Valid {
		return ResultNoTrade
	}
	return ResultTrade
}

func signalModelResult(signals []SignalResult, cfg StrategyConfig) string {
	for _, signal := range signals {
		if signal.FinalScore >= cfg.Scoring.MinBuyScore {
			return ResultTrade
		}
	}
	return ResultNoTrade
}

func planningTickers(universe Universe, portfolio Portfolio) []string {
	tickers := append([]string{}, universe.Tickers...)
	for _, position := range portfolio.Positions {
		tickers = append(tickers, position.Ticker)
	}
	return NormalizeTickers(tickers)
}

func PlanPortfolio(
	asOf time.Time,
	universe Universe,
	portfolio Portfolio,
	signals []SignalResult,
	cfg StrategyConfig,
) PortfolioPlanResult {
	planner := newRebalancePlanner(asOf, universe, portfolio, signals, cfg)
	planner.planReductions()
	planner.planStarters()
	planner.planTopUps()
	return planner.result()
}

func ValidatePlan(plan PortfolioPlanResult, universe Universe, portfolio Portfolio, cfg StrategyConfig) ValidationResult {
	var errs []string
	if !cfg.Approved {
		errs = append(errs, "strategy config is not approved")
	}
	allowed := allowedTickerSet(universe.Tickers)
	current := CurrentWeights(portfolio)
	for _, target := range plan.Targets {
		currentWeight := current[strings.ToUpper(target.Ticker)]
		if currentWeight == 0 && target.TargetWeight > 0 && !tickerAllowed(allowed, target.Ticker) {
			errs = append(errs, target.Ticker+" is not in universe")
		}
		if target.TargetWeight > cfg.Portfolio.MaxPositionPct && target.TargetWeight >= currentWeight-0.000001 {
			errs = append(errs, target.Ticker+" exceeds max position weight")
		}
		if target.TargetWeight < 0 && cfg.Portfolio.LongOnly {
			errs = append(errs, target.Ticker+" has negative target weight in long-only strategy")
		}
	}
	turnover := 0.0
	newPositions := 0
	for _, trade := range plan.ProposedTrades {
		currentWeight := current[strings.ToUpper(trade.Ticker)]
		if currentWeight == 0 && trade.Side == SideBuy && !tickerAllowed(allowed, trade.Ticker) {
			errs = append(errs, trade.Ticker+" trade is not in universe")
		}
		if math.Abs(trade.DeltaWeight) > cfg.Risk.MaxSingleTradePct+0.000001 {
			errs = append(errs, trade.Ticker+" exceeds max single trade size")
		}
		if currentWeight == 0 && trade.Side == SideBuy {
			newPositions++
		}
		turnover += math.Abs(trade.DeltaWeight)
	}
	if newPositions > cfg.Risk.MaxNewPositionsPerRun {
		errs = append(errs, "plan exceeds max new positions per run")
	}
	if turnover > cfg.Risk.TurnoverBudgetPct+0.000001 {
		errs = append(errs, "plan exceeds turnover budget")
	}
	if portfolio.Equity < 0 {
		errs = append(errs, "portfolio equity must be non-negative")
	}
	return ValidationResult{Valid: len(errs) == 0, Errors: errs}
}

func LoadSignals(pathOrJSON string) ([]SignalResult, error) {
	raw := []byte(strings.TrimSpace(pathOrJSON))
	if !strings.HasPrefix(string(raw), "[") && !strings.HasPrefix(string(raw), "{") {
		fileRaw, err := os.ReadFile(pathOrJSON)
		if err != nil {
			return nil, err
		}
		raw = fileRaw
	}
	var signals []SignalResult
	if err := json.Unmarshal(raw, &signals); err == nil {
		return signals, nil
	}
	var wrapped struct {
		Signals []SignalResult `json:"signals"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Signals, nil
}

func LoadPlan(pathOrJSON string) (PortfolioPlanResult, error) {
	raw := []byte(strings.TrimSpace(pathOrJSON))
	if !strings.HasPrefix(string(raw), "{") {
		fileRaw, err := os.ReadFile(pathOrJSON)
		if err != nil {
			return PortfolioPlanResult{}, err
		}
		raw = fileRaw
	}
	var plan PortfolioPlanResult
	if err := json.Unmarshal(raw, &plan); err != nil {
		return PortfolioPlanResult{}, err
	}
	return plan, nil
}

func LoadStrategyRunResult(pathOrJSON string) (StrategyRunResult, error) {
	raw := []byte(strings.TrimSpace(pathOrJSON))
	if !strings.HasPrefix(string(raw), "{") {
		fileRaw, err := os.ReadFile(pathOrJSON)
		if err != nil {
			return StrategyRunResult{}, err
		}
		raw = fileRaw
	}
	var result StrategyRunResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return StrategyRunResult{}, err
	}
	return result, nil
}

func simpleMomentumScore(bars []Bar) float64 {
	if len(bars) < 2 {
		return 0
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })
	latest := bars[len(bars)-1].Close
	lookbackIndex := max(0, len(bars)-61)
	past := bars[lookbackIndex].Close
	if latest <= 0 || past <= 0 {
		return 0
	}
	raw := latest/past - 1
	return clamp(raw/0.20, -1, 1)
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func round(v float64, places int) float64 {
	pow := math.Pow10(places)
	return math.Round(v*pow) / pow
}
