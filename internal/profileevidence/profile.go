package profileevidence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func MarkovContexts(
	ctx context.Context,
	api client.API,
	tickers []string,
	asOf time.Time,
	horizons []string,
) ([]mpal.TickerMarkovResult, error) {
	tickers = mpal.NormalizeTickers(tickers)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("expected at least one ticker")
	}
	if api == nil {
		return nil, fmt.Errorf("marketpal api client is not configured")
	}
	profiles, err := Fetch(ctx, api, tickers, asOf)
	if err != nil {
		return nil, err
	}
	horizons = normalizeHorizons(horizons)
	results := make([]mpal.TickerMarkovResult, 0, len(horizons))
	for _, horizon := range horizons {
		horizonName, horizonBars := mpal.MarkovHorizon(horizon)
		result := mpal.TickerMarkovResult{
			RunID:       mpal.RunID("ticker_profile_markov", asOf),
			Mode:        "ticker_profile_markov",
			AsOf:        asOf,
			Rebalance:   horizonName,
			Horizon:     horizonName,
			HorizonBars: horizonBars,
			Results:     make([]mpal.TickerMarkovItem, 0, len(tickers)),
		}
		for _, ticker := range tickers {
			profile := profiles[ticker]
			item := mpal.TickerMarkovItem{Ticker: ticker, Freshness: profile.Freshness, Warnings: append([]string(nil), profile.Warnings...)}
			if markov, ok := profile.Markov[horizonName]; ok {
				copy := markov
				item.Markov = &copy
			} else {
				item.Warnings = mpal.AppendWarnings(item.Warnings, "server profile did not return "+horizonName+" Markov evidence")
			}
			if rawKelly, ok := profile.RawKelly[horizonName]; ok {
				copy := rawKelly
				item.RawKelly = &copy
			} else {
				item.Warnings = mpal.AppendWarnings(item.Warnings, "server profile did not return "+horizonName+" raw Kelly evidence")
			}
			result.Results = append(result.Results, item)
		}
		results = append(results, result)
	}
	return results, nil
}

func Fetch(ctx context.Context, api client.API, tickers []string, asOf time.Time) (map[string]mpal.ProfileScore, error) {
	tickers = mpal.NormalizeTickers(tickers)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("expected at least one ticker")
	}
	payload, err := api.GetTickerProfile(ctx, &marketpalv1.MpalTickerProfileRequest{
		Ticker:  tickers[0],
		Tickers: tickers,
		Date:    timestamppb.New(asOf),
	})
	if err != nil {
		return nil, err
	}
	return decodeProfiles(payload, tickers)
}

func decodeProfiles(payload string, tickers []string) (map[string]mpal.ProfileScore, error) {
	raw := []byte(strings.TrimSpace(payload))
	var packet struct {
		Profiles map[string]mpal.ProfileScore `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &packet); err == nil && len(packet.Profiles) > 0 {
		result := make(map[string]mpal.ProfileScore, len(packet.Profiles))
		for ticker, profile := range packet.Profiles {
			normalized := strings.ToUpper(strings.TrimSpace(ticker))
			if normalized == "" {
				normalized = strings.ToUpper(strings.TrimSpace(profile.Ticker))
			}
			profile.Ticker = normalized
			result[normalized] = profile
		}
		return result, nil
	}
	var profile mpal.ProfileScore
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("decode ticker profile evidence: %w", err)
	}
	if profile.Ticker == "" && len(tickers) > 0 {
		profile.Ticker = tickers[0]
	}
	return map[string]mpal.ProfileScore{strings.ToUpper(strings.TrimSpace(profile.Ticker)): profile}, nil
}

func normalizeHorizons(horizons []string) []string {
	if len(horizons) == 0 {
		return []string{"weekly"}
	}
	out := make([]string, 0, len(horizons))
	seen := map[string]struct{}{}
	for _, horizon := range horizons {
		name, _ := mpal.MarkovHorizon(horizon)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
