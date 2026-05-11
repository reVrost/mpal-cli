package mpal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const (
	DecisionGateMode = "decision_gate"

	DecisionGateRoleProposedTrade   = "proposed_trade"
	DecisionGateRoleRejected        = "rejected"
	DecisionGateRoleAlternateSignal = "alternate_signal"

	DecisionGateStatusBaselineExecutable = "baseline_executable"
	DecisionGateStatusBaselineInvalid    = "baseline_invalid"
	DecisionGateStatusRejectedByStrategy = "rejected_by_strategy"
	DecisionGateStatusAlternateContext   = "alternate_context"
)

type DecisionGateOptions struct {
	Alternates     int
	Strategy       *StrategyConfig
	Events         any
	MarkovContexts []TickerMarkovResult
}

type DecisionGateResult struct {
	RunID               string             `json:"run_id"`
	Mode                string             `json:"mode"`
	SourceRunID         string             `json:"source_run_id,omitempty"`
	EvidenceHash        string             `json:"evidence_hash"`
	AsOf                time.Time          `json:"as_of"`
	Strategy            StrategyRef        `json:"strategy"`
	StrategyHorizon     string             `json:"strategy_horizon,omitempty"`
	StrategyHorizonBars int                `json:"strategy_horizon_bars,omitempty"`
	Result              string             `json:"result"`
	Validation          ValidationResult   `json:"validation"`
	Items               []DecisionGateItem `json:"items"`
	Warnings            []string           `json:"warnings,omitempty"`
}

type DecisionGateItem struct {
	Ticker         string                      `json:"ticker"`
	Role           string                      `json:"role"`
	BaselineStatus string                      `json:"baseline_status"`
	IsExecutable   bool                        `json:"is_executable"`
	Side           string                      `json:"side,omitempty"`
	Intent         string                      `json:"intent,omitempty"`
	CurrentWeight  float64                     `json:"current_weight,omitempty"`
	TargetWeight   float64                     `json:"target_weight,omitempty"`
	DeltaWeight    float64                     `json:"delta_weight,omitempty"`
	EstimatedValue float64                     `json:"estimated_value,omitempty"`
	Score          float64                     `json:"score,omitempty"`
	ActionHint     string                      `json:"action_hint,omitempty"`
	Signal         *SignalResult               `json:"signal,omitempty"`
	Sizing         *SizingDecision             `json:"sizing,omitempty"`
	StrategyMarkov *MarkovRead                 `json:"strategy_markov,omitempty"`
	MarkovContext  []DecisionGateMarkovContext `json:"markov_context,omitempty"`
	EventContext   any                         `json:"event_context,omitempty"`
	Reason         string                      `json:"reason,omitempty"`
	Warnings       []string                    `json:"warnings,omitempty"`
}

type DecisionGateMarkovContext struct {
	Rebalance              string   `json:"rebalance,omitempty"`
	Horizon                string   `json:"horizon,omitempty"`
	HorizonBars            int      `json:"horizon_bars,omitempty"`
	ContextOnly            bool     `json:"context_only"`
	FavorableProbability   float64  `json:"favorable_probability,omitempty"`
	UnfavorableProbability float64  `json:"unfavorable_probability,omitempty"`
	RawKelly               float64  `json:"raw_kelly,omitempty"`
	FractionalKelly        float64  `json:"fractional_kelly,omitempty"`
	KellyTargetWeight      float64  `json:"kelly_target_weight,omitempty"`
	CalibrationStatus      string   `json:"calibration_status,omitempty"`
	Confidence             float64  `json:"confidence,omitempty"`
	SampleCount            int      `json:"sample_count,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

func BuildDecisionGateEvidence(run StrategyRunResult, opts DecisionGateOptions) DecisionGateResult {
	if opts.Alternates < 0 {
		opts.Alternates = 0
	}
	signalByTicker := make(map[string]SignalResult, len(run.Signals))
	signals := append([]SignalResult{}, run.Signals...)
	SortSignals(signals)
	for _, signal := range signals {
		ticker := strings.ToUpper(strings.TrimSpace(signal.Ticker))
		if ticker == "" {
			continue
		}
		signal.Ticker = ticker
		signalByTicker[ticker] = signal
	}

	items := make([]DecisionGateItem, 0, len(run.BaselinePlan.ProposedTrades)+len(run.BaselinePlan.Rejected)+opts.Alternates)
	used := map[string]struct{}{}
	for _, trade := range run.BaselinePlan.ProposedTrades {
		ticker := strings.ToUpper(strings.TrimSpace(trade.Ticker))
		if ticker == "" {
			continue
		}
		used[ticker] = struct{}{}
		item := DecisionGateItem{
			Ticker:         ticker,
			Role:           DecisionGateRoleProposedTrade,
			BaselineStatus: baselineTradeStatus(run.Validation),
			IsExecutable:   run.Validation.Valid,
			Side:           trade.Side,
			Intent:         trade.Intent,
			CurrentWeight:  trade.CurrentWeight,
			TargetWeight:   trade.TargetWeight,
			DeltaWeight:    trade.DeltaWeight,
			EstimatedValue: trade.EstimatedValue,
			Sizing:         cloneSizingDecision(trade.Sizing),
			Reason:         trade.Reason,
		}
		if signal, ok := signalByTicker[ticker]; ok {
			item.Signal = cloneSignal(signal)
			item.Score = signal.FinalScore
			item.ActionHint = signal.ActionHint
			item.StrategyMarkov = cloneMarkov(signal.Markov)
		}
		items = append(items, item)
	}

	for _, rejected := range run.BaselinePlan.Rejected {
		ticker := strings.ToUpper(strings.TrimSpace(rejected.Ticker))
		if ticker == "" {
			continue
		}
		used[ticker] = struct{}{}
		item := DecisionGateItem{
			Ticker:         ticker,
			Role:           DecisionGateRoleRejected,
			BaselineStatus: DecisionGateStatusRejectedByStrategy,
			Reason:         rejected.Reason,
		}
		if signal, ok := signalByTicker[ticker]; ok {
			item.Signal = cloneSignal(signal)
			item.Score = signal.FinalScore
			item.ActionHint = signal.ActionHint
			item.StrategyMarkov = cloneMarkov(signal.Markov)
		}
		items = append(items, item)
	}

	if opts.Alternates > 0 {
		alternateCount := 0
		for _, signal := range signals {
			ticker := strings.ToUpper(strings.TrimSpace(signal.Ticker))
			if ticker == "" {
				continue
			}
			if _, ok := used[ticker]; ok {
				continue
			}
			if alternateCount >= opts.Alternates {
				break
			}
			alternateCount++
			used[ticker] = struct{}{}
			items = append(items, DecisionGateItem{
				Ticker:         ticker,
				Role:           DecisionGateRoleAlternateSignal,
				BaselineStatus: DecisionGateStatusAlternateContext,
				Score:          signal.FinalScore,
				ActionHint:     signal.ActionHint,
				Signal:         cloneSignal(signal),
				StrategyMarkov: cloneMarkov(signal.Markov),
				Reason:         "alternate signal context only; not executable without a validated override",
			})
		}
	}

	attachDecisionGateContext(items, opts)
	horizon, horizonBars := decisionGateStrategyHorizon(run)
	result := DecisionGateResult{
		RunID:               RunID(DecisionGateMode, run.AsOf),
		Mode:                DecisionGateMode,
		SourceRunID:         run.RunID,
		AsOf:                run.AsOf,
		Strategy:            run.Strategy,
		StrategyHorizon:     horizon,
		StrategyHorizonBars: horizonBars,
		Result:              run.Result,
		Validation:          run.Validation,
		Items:               items,
		Warnings:            append([]string{}, run.Warnings...),
	}
	result.EvidenceHash = decisionGateEvidenceHash(result)
	return result
}

func decisionGateStrategyHorizon(run StrategyRunResult) (string, int) {
	for _, trade := range run.BaselinePlan.ProposedTrades {
		if trade.Sizing != nil && strings.TrimSpace(trade.Sizing.Horizon) != "" {
			return trade.Sizing.Horizon, trade.Sizing.HorizonBars
		}
	}
	for _, signal := range run.Signals {
		if signal.Markov != nil && strings.TrimSpace(signal.Markov.Horizon) != "" {
			return signal.Markov.Horizon, signal.Markov.HorizonBars
		}
	}
	return "", 0
}

func DecisionGateTickers(run StrategyRunResult, alternates int) []string {
	result := BuildDecisionGateEvidence(run, DecisionGateOptions{Alternates: alternates})
	tickers := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		tickers = append(tickers, item.Ticker)
	}
	return NormalizeTickers(tickers)
}

func baselineTradeStatus(validation ValidationResult) string {
	if validation.Valid {
		return DecisionGateStatusBaselineExecutable
	}
	return DecisionGateStatusBaselineInvalid
}

func attachDecisionGateContext(items []DecisionGateItem, opts DecisionGateOptions) {
	for i := range items {
		ticker := items[i].Ticker
		if opts.Events != nil {
			items[i].EventContext = tickerContext(opts.Events, ticker)
		}
		for _, contextResult := range opts.MarkovContexts {
			if context := markovContextForTicker(contextResult, ticker, opts.Strategy); context != nil {
				items[i].MarkovContext = append(items[i].MarkovContext, *context)
			}
		}
	}
}

func markovContextForTicker(result TickerMarkovResult, ticker string, cfg *StrategyConfig) *DecisionGateMarkovContext {
	for _, item := range result.Results {
		if strings.EqualFold(item.Ticker, ticker) {
			context := DecisionGateMarkovContext{
				Rebalance:   result.Rebalance,
				Horizon:     result.Horizon,
				HorizonBars: result.HorizonBars,
				ContextOnly: true,
				Warnings:    append([]string{}, item.Warnings...),
			}
			if item.Markov == nil {
				return &context
			}
			context.Horizon = item.Markov.Horizon
			context.HorizonBars = item.Markov.HorizonBars
			context.FavorableProbability = round(item.Markov.FavorableProbability, 6)
			context.UnfavorableProbability = round(item.Markov.UnfavorableProbability, 6)
			context.Confidence = round(item.Markov.Confidence, 6)
			context.SampleCount = item.Markov.SampleCount
			if cfg != nil {
				applied := applyRiskProfileDefaultsCopy(*cfg)
				decision, _ := fractionalKellyDecision(SignalResult{Ticker: ticker, Markov: item.Markov}, normalizeSizingConfig(applied.Risk))
				context.RawKelly = decision.RawKelly
				context.FractionalKelly = decision.FractionalKelly
				context.KellyTargetWeight = decision.KellyTargetWeight
				context.CalibrationStatus = decision.CalibrationStatus
				context.Warnings = AppendWarnings(context.Warnings, decision.Warnings...)
			}
			return &context
		}
	}
	return nil
}

func tickerContext(value any, ticker string) any {
	matches := tickerContextMatches(value, strings.ToUpper(strings.TrimSpace(ticker)))
	switch len(matches) {
	case 0:
		return nil
	case 1:
		return matches[0]
	default:
		return matches
	}
}

func tickerContextMatches(value any, ticker string) []any {
	switch typed := value.(type) {
	case map[string]any:
		if tickerFieldMatches(typed, ticker) {
			return []any{typed}
		}
		if direct, ok := typed[ticker]; ok {
			return []any{direct}
		}
		if direct, ok := typed[strings.ToLower(ticker)]; ok {
			return []any{direct}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var matches []any
		for _, key := range keys {
			matches = append(matches, tickerContextMatches(typed[key], ticker)...)
		}
		return matches
	case []any:
		var matches []any
		for _, item := range typed {
			matches = append(matches, tickerContextMatches(item, ticker)...)
		}
		return matches
	default:
		return nil
	}
}

func tickerFieldMatches(value map[string]any, ticker string) bool {
	for _, key := range []string{"ticker", "symbol", "Ticker", "Symbol"} {
		raw, ok := value[key]
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(toString(raw)), ticker) {
			return true
		}
	}
	return false
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func cloneSignal(signal SignalResult) *SignalResult {
	copy := signal
	return &copy
}

func cloneMarkov(markov *MarkovRead) *MarkovRead {
	if markov == nil {
		return nil
	}
	copy := *markov
	return &copy
}

func cloneSizingDecision(sizing *SizingDecision) *SizingDecision {
	if sizing == nil {
		return nil
	}
	copy := *sizing
	return &copy
}

func decisionGateEvidenceHash(result DecisionGateResult) string {
	copy := result
	copy.RunID = ""
	copy.EvidenceHash = ""
	raw, err := json.Marshal(copy)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
