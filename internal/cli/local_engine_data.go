package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type apiMarketData struct {
	client client.API
}

func (a apiMarketData) Bars(ctx context.Context, ticker string, start, end time.Time) (mpal.BarsResult, error) {
	payload, err := a.client.GetTickerBars(ctx, &marketpalv1.MpalTickerBarsRequest{
		Ticker: strings.ToUpper(strings.TrimSpace(ticker)),
		Start:  timestamppb.New(start),
		End:    timestamppb.New(end),
	})
	if err != nil {
		return mpal.BarsResult{}, err
	}
	var result mpal.BarsResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return mpal.BarsResult{}, fmt.Errorf("decode ticker bars: %w", err)
	}
	return result, nil
}

type apiProfileScores struct {
	client client.API

	mu    sync.Mutex
	cache map[string]mpal.ProfileScore
}

func newAPIProfileScores(api client.API) *apiProfileScores {
	return &apiProfileScores{
		client: api,
		cache:  map[string]mpal.ProfileScore{},
	}
}

func (a *apiProfileScores) Score(ctx context.Context, ticker string, asOf time.Time) (mpal.ProfileScore, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	a.mu.Lock()
	if score, ok := a.cache[ticker]; ok {
		a.mu.Unlock()
		return score, nil
	}
	a.mu.Unlock()

	payload, err := a.client.GetTickerProfile(ctx, &marketpalv1.MpalTickerProfileRequest{
		Ticker: ticker,
		Date:   timestamppb.New(asOf),
	})
	if err != nil {
		return mpal.ProfileScore{}, err
	}
	score, err := decodeProfileScore(payload, ticker)
	if err != nil {
		return mpal.ProfileScore{}, err
	}
	a.mu.Lock()
	a.cache[ticker] = score
	a.mu.Unlock()
	return score, nil
}

func decodeProfileScore(payload string, ticker string) (mpal.ProfileScore, error) {
	var single mpal.ProfileScore
	if err := json.Unmarshal([]byte(payload), &single); err == nil && single.Ticker != "" {
		return single, nil
	}

	var wrapped struct {
		Profiles map[string]mpal.ProfileScore `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(payload), &wrapped); err != nil {
		return mpal.ProfileScore{}, fmt.Errorf("decode ticker profile: %w", err)
	}
	for key, score := range wrapped.Profiles {
		if strings.EqualFold(key, ticker) || strings.EqualFold(score.Ticker, ticker) {
			if score.Ticker == "" {
				score.Ticker = strings.ToUpper(ticker)
			}
			return score, nil
		}
	}
	return mpal.ProfileScore{}, fmt.Errorf("profile unavailable for %s", ticker)
}
