package localmarkov

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/stretchr/testify/require"
)

type fakeAPI struct {
	errTicker string
	active    atomic.Int32
	maxActive atomic.Int32
}

func (f *fakeAPI) GetTickerEvents(context.Context, *marketpalv1.MpalTickerEventsRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetTickerBars(ctx context.Context, req *marketpalv1.MpalTickerBarsRequest) (string, error) {
	if req.Ticker == f.errTicker {
		return "", fmt.Errorf("forced bars error")
	}
	active := f.active.Add(1)
	for {
		maxActive := f.maxActive.Load()
		if active <= maxActive || f.maxActive.CompareAndSwap(maxActive, active) {
			break
		}
	}
	defer f.active.Add(-1)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(20 * time.Millisecond):
	}
	raw, err := json.Marshal(mpal.BarsResult{Ticker: req.Ticker, Bars: testBars(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (f *fakeAPI) GetTickerProfile(context.Context, *marketpalv1.MpalTickerProfileRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetTickerFinancials(context.Context, *marketpalv1.MpalTickerFinancialsRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetTickerFundamentals(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetTickerInsiders(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetTickerOwnership(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetPortfolioSnapshot(context.Context, *marketpalv1.MpalPortfolioSnapshotRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) GetWatchlist(context.Context, *marketpalv1.MpalWatchlistRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) RunStrategy(context.Context, *marketpalv1.MpalStrategyRunRequest) (string, error) {
	return `{}`, nil
}

func (f *fakeAPI) RunBacktest(context.Context, *marketpalv1.MpalBacktestRunRequest) (string, error) {
	return `{}`, nil
}

func TestRunWithConcurrencyPreservesTickerOrder(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{}
	result, err := runWithConcurrency(
		context.Background(),
		api,
		[]string{"MSFT", "AAPL", "NVDA"},
		time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		"weekly",
		DefaultLookbackDays,
		3,
	)

	require.NoError(t, err)
	require.Greater(t, api.maxActive.Load(), int32(1))
	require.Len(t, result.Results, 3)
	require.Equal(t, "AAPL", result.Results[0].Ticker)
	require.Equal(t, "MSFT", result.Results[1].Ticker)
	require.Equal(t, "NVDA", result.Results[2].Ticker)
}

func TestRunWithConcurrencyReturnsFirstTickerError(t *testing.T) {
	t.Parallel()

	_, err := runWithConcurrency(
		context.Background(),
		&fakeAPI{errTicker: "MSFT"},
		[]string{"AAPL", "MSFT"},
		time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		"weekly",
		DefaultLookbackDays,
		2,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "forced bars error")
}

func testBars(asOf time.Time) []mpal.Bar {
	bars := make([]mpal.Bar, 0, 120)
	price := 100.0
	for i := 0; i < 120; i++ {
		price *= 1.002
		bars = append(bars, mpal.Bar{Date: asOf.AddDate(0, 0, -119+i), Close: price})
	}
	return bars
}
