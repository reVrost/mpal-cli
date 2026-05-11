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
	strategyPayload      string
	tickerBarsPayload    string
	tickerProfilePayload string
	fundamentalsPayload  string
	transactionsPayload  string
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
	if f.tickerProfilePayload != "" {
		return f.tickerProfilePayload, nil
	}
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerFinancials(context.Context, *marketpalv1.MpalTickerFinancialsRequest) (string, error) {
	return `{}`, nil
}
func (f fakeMpalAPI) GetTickerFundamentals(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	if f.fundamentalsPayload != "" {
		return f.fundamentalsPayload, nil
	}
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
func (f fakeMpalAPI) GetPortfolioTransactions(context.Context, *marketpalv1.MpalPortfolioTransactionsRequest) (string, error) {
	if f.transactionsPayload != "" {
		return f.transactionsPayload, nil
	}
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

type recordingTransactionsAPI struct {
	fakeMpalAPI
	req *marketpalv1.MpalPortfolioTransactionsRequest
}

func (f *recordingTransactionsAPI) GetPortfolioTransactions(_ context.Context, req *marketpalv1.MpalPortfolioTransactionsRequest) (string, error) {
	f.req = req
	return `{"kind":"portfolio_transactions","transactions":[]}`, nil
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
	require.Contains(t, commands, "doctor")
	require.Contains(t, commands, "ticker events")
	require.Contains(t, commands, "ticker bars")
	require.Contains(t, commands, "ticker profile")
	require.Contains(t, commands, "ticker financials")
	require.Contains(t, commands, "ticker fundamentals")
	require.Contains(t, commands, "ticker insiders")
	require.Contains(t, commands, "ticker ownership")
	require.Contains(t, commands, "portfolio snapshot")
	require.Contains(t, commands, "portfolio transactions")
	require.Contains(t, commands, "portfolio validate")
	require.Contains(t, commands, "decision gate")
	require.Contains(t, commands, "journal start")
	require.Contains(t, commands, "journal finalize")
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

func TestPortfolioTransactionsCommandPassesLimit(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	api := &recordingTransactionsAPI{}
	a := &app{
		out:    &out,
		errOut: &bytes.Buffer{},
		client: api,
	}
	cmd := a.rootCommand(context.Background())
	cmd.SetArgs([]string{"portfolio", "transactions", "--page", "2", "--limit", "50", "--json"})

	require.NoError(t, cmd.Execute())
	require.NotNil(t, api.req)
	require.Equal(t, int32(2), api.req.Page)
	require.Equal(t, int32(50), api.req.Limit)
	require.Contains(t, out.String(), `"kind":"portfolio_transactions"`)
}

func TestDoctorReportsMissingAPIKey(t *testing.T) {
	t.Setenv("MPAL_API_KEY", "")
	t.Setenv("MPAL_API_KEYS", "")
	t.Setenv("MPAL_JOURNAL", filepath.Join(t.TempDir(), "journal.jsonl"))

	var out bytes.Buffer
	a := &app{
		out:      &out,
		errOut:   &bytes.Buffer{},
		registry: mpal.DefaultStrategyRegistry(),
		journal:  mpal.FileJournal{Path: os.Getenv("MPAL_JOURNAL")},
		client:   fakeMpalAPI{},
	}
	cmd := a.rootCommand(context.Background())
	cmd.SetArgs([]string{"doctor", "--skip-api", "--json"})

	require.NoError(t, cmd.Execute())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	require.Equal(t, "doctor", payload["mode"])
	require.Equal(t, false, payload["ok"])
	require.NotEmpty(t, payload["errors"])
	require.NotEmpty(t, payload["next_steps"])
}

func TestDoctorStrictFailsWhenUnhealthy(t *testing.T) {
	t.Setenv("MPAL_API_KEY", "")
	t.Setenv("MPAL_API_KEYS", "")
	t.Setenv("MPAL_JOURNAL", filepath.Join(t.TempDir(), "journal.jsonl"))

	var out bytes.Buffer
	a := &app{
		out:      &out,
		errOut:   &bytes.Buffer{},
		registry: mpal.DefaultStrategyRegistry(),
		journal:  mpal.FileJournal{Path: os.Getenv("MPAL_JOURNAL")},
		client:   fakeMpalAPI{},
	}
	cmd := a.rootCommand(context.Background())
	cmd.SetArgs([]string{"doctor", "--skip-api", "--strict", "--json"})

	require.Error(t, cmd.Execute())
	require.Contains(t, out.String(), `"ok": false`)
}

func TestTickerFundamentalsCommandPassesThroughCompactMetrics(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	a := &app{
		out:    &out,
		errOut: &bytes.Buffer{},
		client: fakeMpalAPI{
			fundamentalsPayload: `{"kind":"ticker_fundamentals","fundamentals":{"AAPL":{"ticker":"AAPL","pe":24.5,"ps":6.7,"short_interest":0.62,"revenue_growth_yoy":0.0834,"revenue_growth_yoy_pct":8.34}}}`,
		},
	}
	cmd := a.rootCommand(context.Background())
	cmd.SetArgs([]string{"ticker", "fundamentals", "--tickers", "AAPL", "--json"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), `"pe":24.5`)
	require.Contains(t, out.String(), `"ps":6.7`)
	require.Contains(t, out.String(), `"short_interest":0.62`)
	require.Contains(t, out.String(), `"revenue_growth_yoy_pct":8.34`)
}

func TestJournalStartFinalizeListGetUsesSQLiteReviewJournal(t *testing.T) {
	t.Setenv("MPAL_REVIEW_JOURNAL", filepath.Join(t.TempDir(), "mpal.db"))

	startInput := `{
		"id":"review_cli",
		"as_of":"2026-05-11",
		"strategy_id":"engine_weekly_swing_v1",
		"strategy_config_text":"id: engine_weekly_swing_v1\n",
		"portfolio_scope":"engine",
		"universe_tickers":["MU","META"],
		"user_requested_tickers":["DOCN"],
		"execution_result":"TRADE",
		"agent_harness":"codex",
		"agent_model":"gpt-5",
		"agent_skill":"marketpal-trader",
		"user_prompt_text":"review my engine",
		"chat_history_text":"agent reviewed the strategy output",
		"agent_summary":"Accept MU and watch DOCN.",
		"positions":[
			{"ticker":"MU","model_bucket":"proposed","model_intent":"STARTER","model_score":0.92,"model_weight":0.015,"agent_decision":"trade","agent_weight":0.01,"agent_reason":"clean enough"}
		]
	}`
	var startOut bytes.Buffer
	startCode := Main([]string{"journal", "start", "--input", startInput, "--json"}, &startOut, &bytes.Buffer{})
	require.Equal(t, 0, startCode, startOut.String())

	var startPayload map[string]any
	require.NoError(t, json.Unmarshal(startOut.Bytes(), &startPayload))
	review := startPayload["review"].(map[string]any)
	require.Equal(t, "review_cli", review["id"])
	require.Equal(t, "engine_weekly_swing_v1", review["strategy_id"])

	finalInput := `{
		"final_decision":"trade",
		"human_reasoning_text":"Accepted the agent plan with smaller sizing.",
		"final_validation_valid":true,
		"final_validation_summary":"Validated.",
		"positions":[
			{"ticker":"MU","human_decision":"trade","human_weight":0.01,"human_reason":"final human call"}
		]
	}`
	var finalOut bytes.Buffer
	finalCode := Main([]string{"journal", "finalize", "--id", "review_cli", "--input", finalInput, "--json"}, &finalOut, &bytes.Buffer{})
	require.Equal(t, 0, finalCode, finalOut.String())

	var finalPayload map[string]any
	require.NoError(t, json.Unmarshal(finalOut.Bytes(), &finalPayload))
	finalReview := finalPayload["review"].(map[string]any)
	require.Equal(t, "trade", finalReview["final_decision"])
	positions := finalPayload["positions"].([]any)
	require.Len(t, positions, 1)
	require.Equal(t, "trade", positions[0].(map[string]any)["human_decision"])

	var listOut bytes.Buffer
	require.Equal(t, 0, Main([]string{"journal", "list", "--limit", "1", "--json"}, &listOut, &bytes.Buffer{}))
	var listPayload map[string]any
	require.NoError(t, json.Unmarshal(listOut.Bytes(), &listPayload))
	require.Len(t, listPayload["reviews"], 1)

	var getOut bytes.Buffer
	require.Equal(t, 0, Main([]string{"journal", "get", "--id", "review_cli", "--json"}, &getOut, &bytes.Buffer{}))
	require.Contains(t, getOut.String(), `"id": "review_cli"`)
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
		out:               &out,
		reviewJournalPath: filepath.Join(dir, "mpal.db"),
		client: fakeMpalAPI{strategyPayload: `{
  "result": "TRADE",
  "model_result": "TRADE",
  "execution_result": "TRADE",
  "baseline_plan": {
    "result": "TRADE",
    "proposed_trades": [{
      "ticker": "AAPL",
      "side": "BUY",
      "intent": "STARTER",
      "target_weight": 0.015,
      "delta_weight": 0.015,
      "estimated_value": 1500,
      "reason": "starter position",
      "sizing": {
        "method": "fractional_kelly",
        "raw_kelly": 0.08,
        "fractional_kelly": 0.02,
        "kelly_target_weight": 0.02,
        "final_target_weight": 0.015,
        "binding_constraint": "max_single_trade_pct",
        "calibration_status": "heuristic_markov"
      }
    }]
  },
  "signals": [{"ticker":"AAPL","final_score":0.91,"action_hint":"BUY"}],
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
	journalID, ok := payload["journal_entry_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, journalID)
	validation, ok := payload["validation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, validation["valid"])

	out.Reset()
	reportPath := filepath.Join(dir, "review.html")
	reportCmd := a.reportCommand(context.Background())
	reportCmd.SetArgs([]string{journalID, "--output", reportPath, "--notes", "manual review note", "--json"})
	require.NoError(t, reportCmd.Execute())
	require.FileExists(t, reportPath)
	reportHTML, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Contains(t, string(reportHTML), "<th>Ticker</th>")
	require.Contains(t, string(reportHTML), "<th class=\"num\">Raw Kelly</th>")
	require.Contains(t, string(reportHTML), "manual review note")
}

func TestStrategyRunExecutesAdvancedScoringLocally(t *testing.T) {
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
	a := &app{
		out:               &out,
		reviewJournalPath: filepath.Join(dir, "mpal.db"),
		client: fakeMpalAPI{
			tickerBarsPayload: `{
  "ticker": "AAPL",
  "start": "2025-05-03T00:00:00Z",
  "end": "2026-05-03T00:00:00Z",
  "bars": [
    {"date":"2025-09-01T00:00:00Z","open":100,"high":100,"low":100,"close":100,"volume":1000},
    {"date":"2026-05-03T00:00:00Z","open":75,"high":75,"low":75,"close":75,"volume":1000}
  ],
  "freshness": {"source":"marketpal_historical_prices","provider":"marketpal","storage":"marketpal_api","stale":false}
}`,
			tickerProfilePayload: `{
  "ticker": "AAPL",
  "as_of": "2026-05-03T00:00:00Z",
  "profile_score": 0.1,
  "momentum_score": 0.2,
  "quality_score": 0.9,
  "value_score": 0.8,
  "score_source": "qvm_components"
}`,
		},
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
	require.Contains(t, payload["warnings"], "strategy executed locally because hosted_strategy_api_v1 does not support scoring_v2_quality_value_reversion")

	signals := payload["signals"].([]any)
	require.Len(t, signals, 1)
	signal := signals[0].(map[string]any)
	require.Equal(t, "AAPL", signal["ticker"])
	require.Equal(t, 0.9, signal["quality_score"])
	require.Equal(t, 0.8, signal["value_score"])
	require.Equal(t, 1.0, signal["reversion_score"])
	require.Equal(t, 0.775, signal["final_score"])
}
