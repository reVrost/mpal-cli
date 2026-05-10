package mpal

import (
	"math"
	"sort"
	"strings"
)

const (
	markovModelTrendBucketV1 = "trend_bucket_v1"

	MarkovStateStrongUp  = "STRONG_UP"
	MarkovStateUp        = "UP"
	MarkovStateFlat      = "FLAT"
	MarkovStateDown      = "DOWN"
	MarkovStateSharpDown = "SHARP_DOWN"
)

var markovStates = []string{
	MarkovStateStrongUp,
	MarkovStateUp,
	MarkovStateFlat,
	MarkovStateDown,
	MarkovStateSharpDown,
}

func markovRead(bars []Bar, cfg StrategyConfig) *MarkovRead {
	horizon, horizonBars := markovHorizon(cfg.Portfolio.Rebalance)
	sorted := validMarkovBars(bars)
	if len(sorted) < horizonBars+1 {
		return nil
	}
	return markovReadForHorizon(sorted, horizon, horizonBars)
}

func ComputeMarkovRead(bars []Bar, rebalance string) *MarkovRead {
	horizon, horizonBars := markovHorizon(rebalance)
	sorted := validMarkovBars(bars)
	if len(sorted) < horizonBars+1 {
		return nil
	}
	return markovReadForHorizon(sorted, horizon, horizonBars)
}

func MarkovHorizon(rebalance string) (string, int) {
	return markovHorizon(rebalance)
}

func markovReadForHorizon(bars []Bar, horizon string, horizonBars int) *MarkovRead {
	if horizonBars <= 0 || len(bars) < horizonBars+1 {
		return nil
	}
	volatility := markovHorizonVolatility(bars, horizonBars)
	states := make([]string, 0, len(bars)-horizonBars)
	returns := make([]float64, 0, len(bars)-horizonBars)
	for i := horizonBars; i < len(bars); i++ {
		past := bars[i-horizonBars].Close
		latest := bars[i].Close
		if past <= 0 || latest <= 0 {
			continue
		}
		value := latest/past - 1
		returns = append(returns, value)
		states = append(states, markovTrendState(value, volatility))
	}
	if len(states) == 0 {
		return nil
	}

	currentState := states[len(states)-1]
	currentReturn := returns[len(returns)-1]
	counts := make(map[string]int, len(markovStates))
	currentStateTransitions := 0
	for i := 0; i < len(states)-1; i++ {
		if states[i] != currentState {
			continue
		}
		counts[states[i+1]]++
		currentStateTransitions++
	}
	totalTransitions := max(0, len(states)-1)
	probabilities := smoothedMarkovProbabilities(counts)
	warnings := markovWarnings(len(bars), currentStateTransitions, horizonBars)
	return &MarkovRead{
		Model:                   markovModelTrendBucketV1,
		Horizon:                 horizon,
		HorizonBars:             horizonBars,
		CurrentState:            currentState,
		CurrentReturn:           round(currentReturn, 6),
		TransitionProbabilities: probabilities,
		FavorableProbability:    round(probabilities[MarkovStateStrongUp]+probabilities[MarkovStateUp], 6),
		UnfavorableProbability:  round(probabilities[MarkovStateDown]+probabilities[MarkovStateSharpDown], 6),
		ExpectedStateScore:      round(expectedMarkovStateScore(probabilities), 6),
		SampleCount:             currentStateTransitions,
		TotalTransitionCount:    totalTransitions,
		Confidence:              round(markovConfidence(currentStateTransitions), 6),
		Warnings:                warnings,
	}
}

func markovHorizon(rebalance string) (string, int) {
	switch strings.ToLower(strings.TrimSpace(rebalance)) {
	case "daily":
		return "daily", 1
	case "monthly":
		return "monthly", 21
	default:
		return "weekly", 5
	}
}

func validMarkovBars(bars []Bar) []Bar {
	sorted := append([]Bar{}, bars...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	out := make([]Bar, 0, len(sorted))
	for _, bar := range sorted {
		if bar.Close > 0 {
			out = append(out, bar)
		}
	}
	return out
}

func markovHorizonVolatility(bars []Bar, horizonBars int) float64 {
	if len(bars) < 2 {
		return 0.01
	}
	returns := make([]float64, 0, len(bars)-1)
	for i := 1; i < len(bars); i++ {
		previous := bars[i-1].Close
		latest := bars[i].Close
		if previous <= 0 || latest <= 0 {
			continue
		}
		returns = append(returns, latest/previous-1)
	}
	if len(returns) < 2 {
		return 0.01
	}
	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, value := range returns {
		diff := value - mean
		variance += diff * diff
	}
	volatility := math.Sqrt(variance/float64(len(returns)-1)) * math.Sqrt(float64(horizonBars))
	return math.Max(volatility, 0.01)
}

func markovTrendState(value float64, volatility float64) string {
	strong := math.Max(0.04, volatility)
	normal := math.Max(0.01, volatility*0.25)
	switch {
	case value >= strong:
		return MarkovStateStrongUp
	case value >= normal:
		return MarkovStateUp
	case value <= -strong:
		return MarkovStateSharpDown
	case value <= -normal:
		return MarkovStateDown
	default:
		return MarkovStateFlat
	}
}

func smoothedMarkovProbabilities(counts map[string]int) map[string]float64 {
	total := len(markovStates)
	for _, state := range markovStates {
		total += counts[state]
	}
	probabilities := make(map[string]float64, len(markovStates))
	for _, state := range markovStates {
		probabilities[state] = round(float64(counts[state]+1)/float64(total), 6)
	}
	return probabilities
}

func expectedMarkovStateScore(probabilities map[string]float64) float64 {
	return probabilities[MarkovStateStrongUp]*1.0 +
		probabilities[MarkovStateUp]*0.75 +
		probabilities[MarkovStateFlat]*0.5 +
		probabilities[MarkovStateDown]*0.25
}

func markovConfidence(sampleCount int) float64 {
	return clamp(float64(sampleCount)/30, 0, 1)
}

func markovWarnings(barCount int, sampleCount int, horizonBars int) []string {
	var warnings []string
	if barCount < 80 {
		warnings = append(warnings, "markov sample is thin: fewer than 80 valid bars")
	}
	if sampleCount < 10 {
		warnings = append(warnings, "markov current-state transition sample is thin")
	}
	if horizonBars > 1 && barCount < horizonBars*16 {
		warnings = append(warnings, "markov horizon has limited history for transition estimation")
	}
	return warnings
}
