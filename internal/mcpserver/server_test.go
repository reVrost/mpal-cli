package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/stretchr/testify/require"
)

type fakeAPI struct {
	runStrategyReq    *marketpalv1.MpalStrategyRunRequest
	tickerBarsPayload string
}

func (f *fakeAPI) GetTickerEvents(context.Context, *marketpalv1.MpalTickerEventsRequest) (string, error) {
	return `{"events":[]}`, nil
}

func (f *fakeAPI) GetTickerBars(context.Context, *marketpalv1.MpalTickerBarsRequest) (string, error) {
	if f.tickerBarsPayload != "" {
		return f.tickerBarsPayload, nil
	}
	return `{"bars":[]}`, nil
}

func (f *fakeAPI) GetTickerProfile(context.Context, *marketpalv1.MpalTickerProfileRequest) (string, error) {
	return `{"profile_score":0.5}`, nil
}

func (f *fakeAPI) GetTickerFinancials(context.Context, *marketpalv1.MpalTickerFinancialsRequest) (string, error) {
	return `{"financials":{}}`, nil
}

func (f *fakeAPI) GetTickerFundamentals(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{"fundamentals":{}}`, nil
}

func (f *fakeAPI) GetTickerInsiders(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{"transactions":[]}`, nil
}

func (f *fakeAPI) GetTickerOwnership(context.Context, *marketpalv1.MpalTickerDataRequest) (string, error) {
	return `{"flow":[],"events":[]}`, nil
}

func (f *fakeAPI) GetPortfolioSnapshot(context.Context, *marketpalv1.MpalPortfolioSnapshotRequest) (string, error) {
	return `{"cash":1000,"equity":1000,"positions":[]}`, nil
}

func (f *fakeAPI) GetWatchlist(context.Context, *marketpalv1.MpalWatchlistRequest) (string, error) {
	return `{"tickers":["AAPL"]}`, nil
}

func (f *fakeAPI) RunStrategy(_ context.Context, req *marketpalv1.MpalStrategyRunRequest) (string, error) {
	f.runStrategyReq = req
	return `{"result":"TRADE","journal_entry_id":"jrnl_test","warnings":[]}`, nil
}

func (f *fakeAPI) RunBacktest(context.Context, *marketpalv1.MpalBacktestRunRequest) (string, error) {
	return `{"trusted":true,"trades":[]}`, nil
}

func TestServerExposesCapabilityTools(t *testing.T) {
	t.Parallel()

	session, closeSession := testSession(t, &fakeAPI{})
	defer closeSession()

	var toolNames []string
	for tool, err := range session.Tools(context.Background(), nil) {
		require.NoError(t, err)
		toolNames = append(toolNames, tool.Name)
	}
	require.True(t, slices.Contains(toolNames, "mpal_capabilities"))
	require.True(t, slices.Contains(toolNames, "mpal_strategy_run"))
	require.True(t, slices.Contains(toolNames, "mpal_decision_gate"))
	require.False(t, slices.Contains(toolNames, "mpal_execute_trade"))
}

func TestCapabilitiesToolReturnsNoLiveTrading(t *testing.T) {
	t.Parallel()

	session, closeSession := testSession(t, &fakeAPI{})
	defer closeSession()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "mpal_capabilities"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	payload := result.StructuredContent.(map[string]any)
	require.Equal(t, false, payload["live_trade_execution"])
	require.Contains(t, payload["mcp_tools"], "mpal_portfolio_validate")
	require.Contains(t, payload["mcp_tools"], "mpal_decision_gate")
}

func TestDecisionGateToolReturnsEvidence(t *testing.T) {
	t.Parallel()

	session, closeSession := testSession(t, &fakeAPI{})
	defer closeSession()

	run := mpal.StrategyRunResult{
		RunID:           "strategy_run_mcp_test",
		Mode:            "strategy_run",
		AsOf:            time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
		Strategy:        mpal.StrategyRef{ID: "test_strategy", Version: "1.0.0", Approved: true},
		Result:          mpal.ResultTrade,
		ModelResult:     mpal.ResultTrade,
		ExecutionResult: mpal.ResultTrade,
		Signals: []mpal.SignalResult{
			{Ticker: "AAPL", FinalScore: 0.9, ActionHint: "BUY_CANDIDATE"},
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
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "mpal_decision_gate",
		Arguments: map[string]any{
			"run_json": mustJSON(run),
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	payload := result.StructuredContent.(map[string]any)
	require.Equal(t, "decision_gate", payload["mode"])
	require.Equal(t, "strategy_run_mcp_test", payload["source_run_id"])
	require.NotEmpty(t, payload["evidence_hash"])
}

func TestStrategyRunToolSendsConfigHash(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{}
	session, closeSession := testSession(t, api)
	defer closeSession()

	configPath := writeStrategyConfig(t)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "mpal_strategy_run",
		Arguments: map[string]any{
			"date":           "2026-05-09",
			"universe_json":  `{"tickers":["AAPL"]}`,
			"portfolio_json": `{"cash":100000,"equity":100000,"positions":[]}`,
			"config_path":    configPath,
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotNil(t, api.runStrategyReq)
	require.NotEmpty(t, api.runStrategyReq.ConfigHash)
	require.JSONEq(t, `{"tickers":["AAPL"]}`, api.runStrategyReq.UniverseJson)

	payload := result.StructuredContent.(map[string]any)
	require.Equal(t, "TRADE", payload["result"])
	journalID, ok := payload["journal_entry_id"].(string)
	require.True(t, ok)
	require.Contains(t, journalID, "review_")
}

func TestJournalToolsStartFinalizeListGet(t *testing.T) {
	t.Parallel()

	session, closeSession := testSession(t, &fakeAPI{})
	defer closeSession()

	startResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "mpal_journal_start",
		Arguments: map[string]any{
			"input_json": `{"id":"review_mcp","as_of":"2026-05-11","strategy_id":"engine_weekly_swing_v1","strategy_config_text":"id: engine_weekly_swing_v1\n","universe_tickers":["MU","META"],"execution_result":"TRADE","agent_harness":"codex","positions":[{"ticker":"MU","model_bucket":"proposed","agent_decision":"trade"}]}`,
		},
	})
	require.NoError(t, err)
	require.False(t, startResult.IsError)
	startPayload := startResult.StructuredContent.(map[string]any)
	review := startPayload["review"].(map[string]any)
	require.Equal(t, "review_mcp", review["id"])

	finalizeResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "mpal_journal_finalize",
		Arguments: map[string]any{
			"id":         "review_mcp",
			"input_json": `{"final_decision":"trade","human_reasoning_text":"accepted","final_validation_valid":true,"positions":[{"ticker":"MU","human_decision":"trade","human_weight":0.01}]}`,
		},
	})
	require.NoError(t, err)
	require.False(t, finalizeResult.IsError)
	finalPayload := finalizeResult.StructuredContent.(map[string]any)
	finalReview := finalPayload["review"].(map[string]any)
	require.Equal(t, "trade", finalReview["final_decision"])

	listResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "mpal_journal_list",
		Arguments: map[string]any{"limit": 1},
	})
	require.NoError(t, err)
	require.False(t, listResult.IsError)
	listPayload := listResult.StructuredContent.(map[string]any)
	require.Len(t, listPayload["reviews"], 1)

	getResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "mpal_journal_get",
		Arguments: map[string]any{"id": "review_mcp"},
	})
	require.NoError(t, err)
	require.False(t, getResult.IsError)
	got := getResult.StructuredContent.(map[string]any)
	gotReview := got["review"].(map[string]any)
	require.Equal(t, "review_mcp", gotReview["id"])
}

func testSession(t *testing.T, api *fakeAPI) (*mcp.ClientSession, func()) {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := New(Config{
		Registry:          mpal.DefaultStrategyRegistry(),
		Journal:           mpal.FileJournal{Path: filepath.Join(t.TempDir(), "journal.jsonl")},
		ReviewJournalPath: filepath.Join(t.TempDir(), "mpal.db"),
		Client:            api,
	})
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	return clientSession, func() {
		require.NoError(t, clientSession.Close())
		require.NoError(t, serverSession.Wait())
	}
}

func writeStrategyConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "strategy.yaml")
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
  turnover_budget_pct: 0.2
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
	return path
}
