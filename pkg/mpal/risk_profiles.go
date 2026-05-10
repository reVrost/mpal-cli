package mpal

import "strings"

const (
	RiskProfileBasic       = "basic"
	RiskProfileLowChurn    = "low_churn"
	RiskProfileWeeklySwing = "weekly_swing"
	RiskProfileRebuild     = "rebuild"
)

type riskProfileDefaults struct {
	TurnoverBudgetPct  float64
	MaxSingleTradePct  float64
	StarterPositionPct float64
}

func applyRiskProfileDefaultsCopy(cfg StrategyConfig) StrategyConfig {
	applyRiskProfileDefaults(&cfg)
	return cfg
}

func applyRiskProfileDefaults(cfg *StrategyConfig) {
	profile, ok := riskProfileDefaultsFor(cfg.Risk.Profile)
	if !ok {
		return
	}
	if cfg.Risk.TurnoverBudgetPct == 0 {
		cfg.Risk.TurnoverBudgetPct = profile.TurnoverBudgetPct
	}
	if cfg.Risk.MaxSingleTradePct == 0 {
		cfg.Risk.MaxSingleTradePct = profile.MaxSingleTradePct
	}
	if cfg.Risk.StarterPositionPct == 0 {
		cfg.Risk.StarterPositionPct = profile.StarterPositionPct
	}
}

func riskProfileDefaultsFor(name string) (riskProfileDefaults, bool) {
	switch normalizeRiskProfile(name) {
	case RiskProfileBasic:
		return riskProfileDefaults{
			TurnoverBudgetPct:  0.20,
			MaxSingleTradePct:  0.05,
			StarterPositionPct: 0.02,
		}, true
	case RiskProfileLowChurn:
		return riskProfileDefaults{
			TurnoverBudgetPct:  0.06,
			MaxSingleTradePct:  0.025,
			StarterPositionPct: 0.012,
		}, true
	case RiskProfileWeeklySwing:
		return riskProfileDefaults{
			TurnoverBudgetPct:  0.12,
			MaxSingleTradePct:  0.035,
			StarterPositionPct: 0.015,
		}, true
	case RiskProfileRebuild:
		return riskProfileDefaults{
			TurnoverBudgetPct:  0.30,
			MaxSingleTradePct:  0.04,
			StarterPositionPct: 0.015,
		}, true
	default:
		return riskProfileDefaults{}, false
	}
}

func normalizeRiskProfile(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
