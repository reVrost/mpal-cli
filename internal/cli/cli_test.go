package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	"github.com/stretchr/testify/require"
)

var _ client.API = fakeMpalAPI{}

type fakeMpalAPI struct {
	strategyPayload string
}

func (f fakeMpalAPI) GetTickerEvents(context.Context, *marketpalv1.MpalTickerEventsRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerBars(context.Context, *marketpalv1.MpalTickerBarsRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerProfile(context.Context, *marketpalv1.MpalTickerProfileRequest) (string, error) {
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
	require.Contains(t, commands, "portfolio snapshot")
	require.Contains(t, commands, "portfolio validate")
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
