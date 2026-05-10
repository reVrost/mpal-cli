package mpal

import (
	"fmt"
	"math"
	"strings"
)

const (
	SizingMethodFixed           = "fixed"
	SizingMethodFractionalKelly = "fractional_kelly"

	KellyMissingEdgePolicyFixed = "fixed"
	KellyMissingEdgePolicySkip  = "skip"
)

const (
	SizingBindingKellyTarget       = "kelly_target"
	SizingBindingKellyMaxFraction  = "kelly_max_fraction"
	SizingBindingMaxSingleTradePct = "max_single_trade_pct"
	SizingBindingMaxPositionPct    = "max_position_pct"
	SizingBindingTurnoverBudgetPct = "turnover_budget_pct"
	SizingBindingCashBufferPct     = "cash_buffer_pct"
	SizingBindingMinTradeValue     = "min_trade_value"
	SizingBindingFixedFallback     = "fixed_fallback"
	SizingBindingValidationFailure = "validation_failure"
)

const (
	defaultKellyFraction           = 0.25
	defaultKellyMinEdge            = 0.0
	defaultKellyMaxFraction        = 0.05
	defaultKellyDefaultPayoffRatio = 1.0
	defaultKellyMinConfidence      = 0.25
	defaultKellyMinSampleCount     = 30
	defaultKellyMissingEdgePolicy  = KellyMissingEdgePolicyFixed
)

type normalizedSizingConfig struct {
	Method                  string
	KellyFraction           float64
	KellyMinEdge            float64
	KellyMaxFraction        float64
	KellyDefaultPayoffRatio float64
	KellyMinConfidence      float64
	KellyMinSampleCount     int
	KellyMissingEdgePolicy  string
}

func normalizeSizingConfig(risk RiskConfig) normalizedSizingConfig {
	cfg := normalizedSizingConfig{
		Method:                  normalizeSizingMethod(risk.SizingMethod),
		KellyFraction:           defaultKellyFraction,
		KellyMinEdge:            defaultKellyMinEdge,
		KellyMaxFraction:        defaultKellyMaxFraction,
		KellyDefaultPayoffRatio: defaultKellyDefaultPayoffRatio,
		KellyMinConfidence:      defaultKellyMinConfidence,
		KellyMinSampleCount:     defaultKellyMinSampleCount,
		KellyMissingEdgePolicy:  normalizeKellyMissingEdgePolicy(risk.KellyMissingEdgePolicy),
	}
	if cfg.Method == "" {
		cfg.Method = SizingMethodFixed
	}
	if cfg.KellyMissingEdgePolicy == "" {
		cfg.KellyMissingEdgePolicy = defaultKellyMissingEdgePolicy
	}
	if risk.KellyFraction != nil {
		cfg.KellyFraction = *risk.KellyFraction
	}
	if risk.KellyMinEdge != nil {
		cfg.KellyMinEdge = *risk.KellyMinEdge
	}
	if risk.KellyMaxFraction != nil {
		cfg.KellyMaxFraction = *risk.KellyMaxFraction
	}
	if risk.KellyDefaultPayoffRatio != nil {
		cfg.KellyDefaultPayoffRatio = *risk.KellyDefaultPayoffRatio
	}
	if risk.KellyMinConfidence != nil {
		cfg.KellyMinConfidence = *risk.KellyMinConfidence
	}
	if risk.KellyMinSampleCount != nil {
		cfg.KellyMinSampleCount = *risk.KellyMinSampleCount
	}
	return cfg
}

func normalizeSizingMethod(method string) string {
	return strings.ToLower(strings.TrimSpace(method))
}

func normalizeKellyMissingEdgePolicy(policy string) string {
	return strings.ToLower(strings.TrimSpace(policy))
}

func fractionalKellyDecision(signal SignalResult, cfg normalizedSizingConfig) (SizingDecision, bool) {
	decision := SizingDecision{
		Method:            SizingMethodFractionalKelly,
		Source:            "markov",
		CalibrationStatus: "heuristic_markov",
		PayoffRatio:       round(cfg.KellyDefaultPayoffRatio, 6),
	}
	if signal.Markov == nil {
		decision.Warnings = []string{"missing Markov edge data"}
		return decision, false
	}
	markov := signal.Markov
	decision.Confidence = round(markov.Confidence, 6)
	decision.SampleCount = markov.SampleCount
	pWin := markov.FavorableProbability
	pLoss := markov.UnfavorableProbability
	decision.FavorableProbability = round(pWin, 6)
	decision.UnfavorableProbability = round(pLoss, 6)
	if pWin <= 0 || pLoss < 0 || pWin+pLoss <= 0 {
		decision.Warnings = []string{"Markov favorable/unfavorable probabilities do not provide usable edge data"}
		return decision, false
	}
	if markov.SampleCount < cfg.KellyMinSampleCount {
		decision.Warnings = []string{fmt.Sprintf("Markov sample_count %d below Kelly minimum %d", markov.SampleCount, cfg.KellyMinSampleCount)}
		return decision, false
	}
	if markov.Confidence < cfg.KellyMinConfidence {
		decision.Warnings = []string{fmt.Sprintf("Markov confidence %.3f below Kelly minimum %.3f", markov.Confidence, cfg.KellyMinConfidence)}
		return decision, false
	}
	rawKelly := (cfg.KellyDefaultPayoffRatio*pWin - pLoss) / (cfg.KellyDefaultPayoffRatio * (pWin + pLoss))
	decision.RawKelly = round(rawKelly, 6)
	if rawKelly <= cfg.KellyMinEdge {
		decision.Warnings = []string{fmt.Sprintf("Kelly raw edge %.3f not above minimum %.3f", rawKelly, cfg.KellyMinEdge)}
		return decision, false
	}
	shrunkKelly := math.Max(0, rawKelly*markov.Confidence)
	unclampedFractionalKelly := shrunkKelly * cfg.KellyFraction
	target := math.Min(unclampedFractionalKelly, cfg.KellyMaxFraction)
	decision.FractionalKelly = round(unclampedFractionalKelly, 6)
	decision.TargetWeight = round(target, 6)
	decision.KellyTargetWeight = decision.TargetWeight
	if unclampedFractionalKelly > cfg.KellyMaxFraction+0.000001 {
		decision.BindingConstraint = SizingBindingKellyMaxFraction
	} else {
		decision.BindingConstraint = SizingBindingKellyTarget
	}
	return decision, target > 0
}

func kellyAuditReason(prefix string, decision SizingDecision, cfg normalizedSizingConfig) string {
	return fmt.Sprintf("%s sized by fractional Kelly: raw=%.3f, confidence=%.2f, fraction=%.2f, target=%.3f",
		prefix,
		decision.RawKelly,
		decision.Confidence,
		cfg.KellyFraction,
		decision.TargetWeight,
	)
}
