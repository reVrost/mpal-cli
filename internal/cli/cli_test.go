package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/stretchr/testify/require"
)

var _ client.API = fakeMpalAPI{}

type fakeMpalAPI struct {
	strategyPayload   string
	tickerBarsPayload string
}

func (f fakeMpalAPI) GetTickerEvents(context.Context, *marketpalv1.MpalTickerEventsRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerBars(context.Context, *marketpalv1.MpalTickerBarsRequest) (string, error) {
	if f.tickerBarsPayload != "" {
		return f.tickerBarsPayload, nil
	}
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerProfile(context.Context, *marketpalv1.MpalTickerProfileRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerFinancials(context.Context, *marketpalv1.MpalTickerFinancialsRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerFundamentals(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerInsiders(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerOwnership(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetPortfolioSnapshot(context.Context, *marketpalv1.MpalPortfolioSnapshotRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetWatchlist(context.Context, *marketpalv1.MpalWatchlistRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) RunStrategy(context.Context, *marketpalv1.MpalStrategyRunRequest) (string, error) {
	return f.strategyPayload, nil
}
func (f fakeMpalAPI) RunBacktest(context.Context, *marketpalv1.MpalBacktestRunRequest) (string, error) {
	return `{}`, nil
}

type recordingProfileAPI struct {
	fakeMpalAPI
	req *marketpalv1.MpalTickerProfileRequest
}

func (f *recordingProfileAPI) GetTickerProfile(_ context.Context, req *marketpalv1.MpalTickerProfileRequest) (string, error) {
	f.req = req
	return `{"ok":true}`, nil
}

func TestCapabilitiesReturnsValidJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	code := Main([]string{"capabilities", "--json"}, &out, &bytes.Buffer{})
	require.Equal(t, 0, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.Equal(t, false, payload["live_trade_execution"])
	commands, ok := payload["commands"].([]any)
	require.True(t, ok)
	require.Contains(t, commands, "ticker events")
	require.Contains(t, commands, "ticker bars")
	require.Contains(t, commands, "ticker profile")
	require.Contains(t, commands, "ticker financials")
	require.Contains(t, commands, "ticker fundamentals")
	require.Contains(t, commands, "ticker insiders")
	require.Contains(t, commands, "ticker ownership")
	require.Contains(t, commands, "ticker markov")
	require.Contains(t, commands, "portfolio snapshot")
	require.Contains(t, commands, "portfolio validate")
	require.Contains(t, commands, "decision gate")
	require.Contains(t, commands, "journal append")
	require.NotContains(t, commands, "data bars")
	require.NotContains(t, commands, "profile score")
	require.NotContains(t, commands, "signal score")
	require.NotContains(t, commands, "signal rank")
	require.NotContains(t, commands, "ticker score")
	require.NotContains(t, commands, "ticker rank")
	require.NotContains(t, commands, "portfolio plan")
	require.NotContains(t, commands, "research portfolio")
	require.NotContains(t, commands, "research watchlist")
	require.NotContains(t, commands, "research brief")
	require.NotContains(t, commands, "context trade-review")
	require.NotContains(t, commands, "admin api-keys backfill")
	require.NotContains(t, commands, "admin portfolios backfill")
	require.NotContains(t, commands, "admin portfolios compare")
}

func TestDecisionGateCommandReturnsEvidence(t *testing.T) {
	t.Parallel()

	run := mpal.StrategyRunResult{
		RunID:           "strategy_run_cli_test",
		Mode:            "strategy_run",
		AsOf:            time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
		Strategy:        mpal.StrategyRef{ID: "test_strategy", Version: "1.0.0", Approved: true},
		Result:          mpal.ResultTrade,
		ModelResult:     mpal.ResultTrade,
		ExecutionResult: mpal.ResultTrade,
		Signals: []mpal.SignalResult{
			{Ticker: "AAPL", FinalScore: 0.9, ActionHint: "BUY_CANDIDATE"},
			{Ticker: "MSFT", FinalScore: 0.8, ActionHint: "BUY_CANDIDATE"},
		},
		BaselinePlan: mpal.PortfolioPlanResult{
			Result: mpal.ResultTrade,
			ProposedTrades: []mpal.ProposedTrade{{
				Ticker:       "AAPL",
				Side:         mpal.SideBuy,
				Intent:       mpal.TradeIntentStarter,
				TargetWeight: 0.02,
				DeltaWeight:  0.02,
				Reason:       "starter position from top-ranked score above buy threshold",
			}},
		},
		Validation: mpal.ValidationResult{Valid: true},
	}
	dir := t.TempDir()
	runPath := filepath.Join(dir, "run.json")
	require.NoError(t, os.WriteFile(runPath, []byte(mustJSON(run)), 0o600))

	var out bytes.Buffer
	code := Main([]string{"decision", "gate", "--run", runPath, "--alternates", "1", "--json"}, &out, &bytes.Buffer{})
	require.Equal(t, 0, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.Equal(t, "decision_gate", payload["mode"])
	require.Equal(t, "strategy_run_cli_test", payload["source_run_id"])
	require.NotEmpty(t, payload["evidence_hash"])
	items := payload["items"].([]any)
	require.Len(t, items, 2)
	require.Equal(t, "proposed_trade", items[0].(map[string]any)["role"])
	require.Equal(t, "alternate_signal", items[1].(map[string]any)["role"])
}

func TestTickerProfileCommandAcceptsBatchTickers(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	api := &recordingProfileAPI{}
	a := &app{out: &out, client: api}
	cmd := a.tickerProfileCommand(context.Background())
	cmd.SetArgs([]string{
		"--tickers", "aapl,MSFT",
		"--date", "2026-05-10",
		"--json",
	})

	require.NoError(t, cmd.Execute())
	require.NotNil(t, api.req)
	require.Equal(t, "AAPL", api.req.Ticker)
	require.Equal(t, []string{"AAPL", "MSFT"}, api.req.Tickers)
}

func TestTickerMarkovReturnsLocalRead(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	bars := make([]mpal.Bar, 0, 120)
	price := 100.0
	for i := 0; i < 120; i++ {
		price *= 1.002
		bars = append(bars, mpal.Bar{Date: asOf.AddDate(0, 0, -119+i), Close: price})
	}
	payload := mustJSON(mpal.BarsResult{
		Ticker: "AAPL",
		Bars:   bars,
		Freshness: &mpal.Freshness{
			Source: "marketpal_historical_prices",
			Stale:  false,
		},
	})

	var out bytes.Buffer
	a := &app{
		out:    &out,
		client: fakeMpalAPI{tickerBarsPayload: payload},
	}
	cmd := a.tickerMarkovCommand(context.Background())
	cmd.SetArgs([]string{
		"--tickers", "AAPL",
		"--date", "2026-05-10",
		"--rebalance", "weekly",
		"--json",
	})

	require.NoError(t, cmd.Execute())
	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	require.Equal(t, "ticker_markov", result["mode"])
	require.Equal(t, "weekly", result["horizon"])
	results := result["results"].([]any)
	require.Len(t, results, 1)
	item := results[0].(map[string]any)
	require.Equal(t, "AAPL", item["ticker"])
	require.NotNil(t, item["markov"])
}

func TestStrategyValidateReturnsValidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "strategy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
id: test_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
  long_only: true
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
risk:
  turnover_budget_pct: 0.3
  max_single_trade_pct: 0.2
  starter_position_pct: 0.02
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
  protect_unscored_holdings: true
backtest:
  initial_cash: 100000
  fee_bps: 5
  slippage_bps: 10
`), 0o600))

	var out bytes.Buffer
	code := Main([]string{"strategy", "validate", "--config", path, "--json"}, &out, &bytes.Buffer{})
	require.Equal(t, 0, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.NotEmpty(t, payload["config_hash"])
	require.Equal(t, "hosted_strategy_api_v1", payload["api_contract"])
	compatibility, ok := payload["api_compatibility"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, compatibility["valid"])
}

func TestStrategyRunJSONIncludesExecutionValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "strategy.yaml")
	universePath := filepath.Join(dir, "universe.json")
	portfolioPath := filepath.Join(dir, "portfolio.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`
id: test_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.7
  profile_weight: 0.3
  min_buy_score: 0.6
  min_hold_score: 0.2
portfolio:
  long_only: true
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
risk:
  turnover_budget_pct: 0.1
  max_single_trade_pct: 0.2
  starter_position_pct: 0.02
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
  protect_unscored_holdings: true
backtest:
  initial_cash: 100000
  fee_bps: 5
  slippage_bps: 10
`), 0o600))
	require.NoError(t, os.WriteFile(universePath, []byte(`{"tickers":["AAPL"]}`), 0o600))
	require.NoError(t, os.WriteFile(portfolioPath, []byte(`{"cash":100000,"equity":100000,"positions":[]}`), 0o600))

	var out bytes.Buffer
	a := &app{
		out: &out,
		client: fakeMpalAPI{strategyPayload: `{
  "result": "TRADE",
  "model_result": "TRADE",
  "execution_result": "TRADE",
  "validation": {"valid": true}
}`},
	}
	cmd := a.strategyRunCommand(context.Background())
	cmd.SetArgs([]string{
		"--date", "2026-05-03",
		"--universe", universePath,
		"--portfolio", portfolioPath,
		"--config", configPath,
		"--json",
	})

	require.NoError(t, cmd.Execute())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.Equal(t, "TRADE", payload["result"])
	require.Equal(t, "TRADE", payload["model_result"])
	require.Equal(t, "TRADE", payload["execution_result"])
	validation, ok := payload["validation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, validation["valid"])
}

func TestStrategyRunRejectsHostedIncompatibleConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "strategy.yaml")
	universePath := filepath.Join(dir, "universe.json")
	portfolioPath := filepath.Join(dir, "portfolio.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`
id: local_v2_strategy
version: 1.0.0
approved: true
scoring:
  momentum_weight: 0.15
  profile_weight: 0.00
  quality_weight: 0.35
  value_weight: 0.35
  reversion_weight: 0.15
  min_buy_score: 0.62
  min_hold_score: 0.30
portfolio:
  long_only: true
  max_positions: 5
  max_position_pct: 0.2
  min_trade_value: 100
risk:
  turnover_budget_pct: 0.1
  max_single_trade_pct: 0.2
  starter_position_pct: 0.02
  max_new_positions_per_run: 2
  cash_buffer_pct: 0.02
  protect_unscored_holdings: true
backtest:
  initial_cash: 100000
  fee_bps: 5
  slippage_bps: 10
`), 0o600))
	require.NoError(t, os.WriteFile(universePath, []byte(`{"tickers":["AAPL"]}`), 0o600))
	require.NoError(t, os.WriteFile(portfolioPath, []byte(`{"cash":100000,"equity":100000,"positions":[]}`), 0o600))

	var out bytes.Buffer
	a := &app{out: &out, client: fakeMpalAPI{strategyPayload: `{"result":"TRADE"}`}}
	cmd := a.strategyRunCommand(context.Background())
	cmd.SetArgs([]string{
		"--date", "2026-05-03",
		"--universe", universePath,
		"--portfolio", portfolioPath,
		"--config", configPath,
		"--json",
	})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not compatible with hosted_strategy_api_v1")
}
