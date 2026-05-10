package localmarkov

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	DefaultLookbackDays     = 365
	defaultFetchConcurrency = 8
)

func Run(
	ctx context.Context,
	api client.API,
	tickers []string,
	asOf time.Time,
	rebalance string,
	lookbackDays int,
) (mpal.TickerMarkovResult, error) {
	return runWithConcurrency(ctx, api, tickers, asOf, rebalance, lookbackDays, defaultFetchConcurrency)
}

func runWithConcurrency(
	ctx context.Context,
	api client.API,
	tickers []string,
	asOf time.Time,
	rebalance string,
	lookbackDays int,
	concurrency int,
) (mpal.TickerMarkovResult, error) {
	tickers = mpal.NormalizeTickers(tickers)
	if len(tickers) == 0 {
		return mpal.TickerMarkovResult{}, fmt.Errorf("expected at least one ticker")
	}
	if api == nil {
		return mpal.TickerMarkovResult{}, fmt.Errorf("marketpal api client is not configured")
	}
	if lookbackDays <= 0 {
		lookbackDays = DefaultLookbackDays
	}
	horizon, horizonBars := mpal.MarkovHorizon(rebalance)
	if strings.TrimSpace(rebalance) == "" {
		rebalance = horizon
	}
	result := mpal.TickerMarkovResult{
		RunID:        mpal.RunID("ticker_markov", asOf),
		Mode:         "ticker_markov",
		AsOf:         asOf,
		Rebalance:    strings.ToLower(strings.TrimSpace(rebalance)),
		Horizon:      horizon,
		HorizonBars:  horizonBars,
		LookbackDays: lookbackDays,
	}
	start := asOf.AddDate(0, 0, -lookbackDays)
	if concurrency <= 0 {
		concurrency = defaultFetchConcurrency
	}
	if concurrency > len(tickers) {
		concurrency = len(tickers)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	items := make([]mpal.TickerMarkovItem, len(tickers))
	errs := make([]error, len(tickers))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				ticker := tickers[index]
				bars, err := tickerBars(ctx, api, ticker, start, asOf)
				if err != nil {
					errs[index] = err
					cancel()
					continue
				}
				item := mpal.TickerMarkovItem{
					Ticker:    ticker,
					BarCount:  len(bars.Bars),
					Freshness: bars.Freshness,
					Warnings:  append([]string{}, bars.Warnings...),
				}
				item.Markov = mpal.ComputeMarkovRead(bars.Bars, rebalance)
				if item.Markov == nil {
					item.Warnings = mpal.AppendWarnings(item.Warnings, "markov unavailable: insufficient valid price history")
				}
				items[index] = item
			}
		}()
	}
	for index := range tickers {
		select {
		case <-ctx.Done():
			break
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	var canceledErr error
	for _, err := range errs {
		if err != nil {
			if errors.Is(err, context.Canceled) {
				if canceledErr == nil {
					canceledErr = err
				}
				continue
			}
			return result, err
		}
	}
	if canceledErr != nil {
		return result, canceledErr
	}
	result.Results = items
	return result, nil
}

func tickerBars(ctx context.Context, api client.API, ticker string, start time.Time, end time.Time) (mpal.BarsResult, error) {
	payload, err := api.GetTickerBars(ctx, &marketpalv1.MpalTickerBarsRequest{
		Ticker: ticker,
		Start:  timestamppb.New(start),
		End:    timestamppb.New(end),
	})
	if err != nil {
		return mpal.BarsResult{}, fmt.Errorf("load bars for %s: %w", ticker, err)
	}
	var result mpal.BarsResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &result); err != nil {
		return mpal.BarsResult{}, fmt.Errorf("decode bars for %s: %w", ticker, err)
	}
	if result.Ticker == "" {
		result.Ticker = ticker
	}
	return result, nil
}
