package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
)

func capabilitiesTool(context.Context, *mcp.CallToolRequest, noInput) (*mcp.CallToolResult, any, error) {
	return nil, object(map[string]any{
		"commands":             mpalCapabilityCommands(),
		"mcp_tools":            mpalMCPTools(),
		"live_trade_execution": false,
	}), nil
}

func readOnlyTool(name, description string) *mcp.Tool {
	notDestructive := false
	openWorld := true
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &notDestructive,
			OpenWorldHint:   &openWorld,
			ReadOnlyHint:    true,
			Title:           name,
		},
	}
}

func additiveTool(name, description string) *mcp.Tool {
	notDestructive := false
	openWorld := true
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &notDestructive,
			OpenWorldHint:   &openWorld,
			ReadOnlyHint:    false,
			Title:           name,
		},
	}
}

func decodePayload(payload string) (any, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return map[string]any{}, nil
	}
	var value any
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		return nil, err
	}
	if _, ok := value.(map[string]any); ok {
		return value, nil
	}
	return map[string]any{"payload": value}, nil
}

func object(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"error": err.Error()}
	}
	if _, ok := out.(map[string]any); ok {
		return out
	}
	return map[string]any{"payload": out}
}

func loadStrategy(path, inline string) (mpal.StrategyConfig, string, error) {
	if strings.TrimSpace(inline) != "" {
		return mpal.LoadStrategyBytes([]byte(inline))
	}
	if strings.TrimSpace(path) == "" {
		return mpal.StrategyConfig{}, "", fmt.Errorf("expected config_path or config_json")
	}
	return mpal.LoadStrategyFile(path)
}

func loadUniverse(path, inline string) (mpal.Universe, error) {
	if strings.TrimSpace(path) != "" {
		return mpal.LoadUniverse(path)
	}
	if strings.TrimSpace(inline) == "" {
		return mpal.Universe{}, fmt.Errorf("expected universe_path or universe_json")
	}
	var universe mpal.Universe
	raw := []byte(inline)
	if err := json.Unmarshal(raw, &universe); err == nil && len(universe.Tickers) > 0 {
		universe.Tickers = mpal.NormalizeTickers(universe.Tickers)
		return universe, nil
	}
	var tickers []string
	if err := json.Unmarshal(raw, &tickers); err != nil {
		return mpal.Universe{}, err
	}
	return mpal.Universe{Tickers: mpal.NormalizeTickers(tickers)}, nil
}

func loadPortfolio(path, inline string) (mpal.Portfolio, error) {
	if strings.TrimSpace(path) != "" {
		return mpal.LoadPortfolio(path)
	}
	if strings.TrimSpace(inline) == "" {
		return mpal.Portfolio{}, fmt.Errorf("expected portfolio_path or portfolio_json")
	}
	var portfolio mpal.Portfolio
	if err := json.Unmarshal([]byte(inline), &portfolio); err != nil {
		return mpal.Portfolio{}, err
	}
	if portfolio.Equity == 0 {
		for _, position := range portfolio.Positions {
			portfolio.Equity += position.MarketValue
		}
		portfolio.Equity += portfolio.Cash
	}
	return portfolio, nil
}

func loadPlan(path, inline string) (mpal.PortfolioPlanResult, error) {
	if strings.TrimSpace(path) != "" {
		return mpal.LoadPlan(path)
	}
	if strings.TrimSpace(inline) == "" {
		return mpal.PortfolioPlanResult{}, fmt.Errorf("expected plan_path or plan_json")
	}
	return mpal.LoadPlan(inline)
}

func readJSONInput(path, inline string) (any, error) {
	raw := []byte(strings.TrimSpace(inline))
	if strings.TrimSpace(path) != "" {
		fileRaw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		raw = fileRaw
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func tickerEventsRequest(in tickerEventsInput) (*marketpalv1.MpalTickerEventsRequest, error) {
	req := &marketpalv1.MpalTickerEventsRequest{
		Tickers:           mpal.NormalizeTickers(in.Tickers),
		Scope:             in.Scope,
		Days:              withDefaultInt32(in.Days, 14),
		Limit:             withDefaultInt32(in.Limit, 80),
		Alternates:        withDefaultInt32(in.Alternates, 5),
		InsightsPerTicker: withDefaultInt32(in.InsightsPerTicker, 2),
	}
	runArg := firstNonEmpty(in.RunPath, in.RunJSON)
	if runArg != "" {
		run, err := mpal.LoadStrategyRunResult(runArg)
		if err != nil {
			return nil, err
		}
		req.RunJson = mustJSON(run)
		if in.PortfolioPath != "" || in.PortfolioJSON != "" {
			portfolio, err := loadPortfolio(in.PortfolioPath, in.PortfolioJSON)
			if err != nil {
				return nil, err
			}
			req.PortfolioJson = mustJSON(portfolio)
		}
	}
	if len(req.Tickers) == 0 && strings.TrimSpace(req.RunJson) == "" && strings.TrimSpace(req.Scope) == "" {
		return nil, fmt.Errorf("expected tickers, run_path, run_json, or scope")
	}
	return req, nil
}

func targetTickers(plan mpal.PortfolioPlanResult) []string {
	tickers := make([]string, 0, len(plan.Targets)+len(plan.ProposedTrades))
	for _, target := range plan.Targets {
		tickers = append(tickers, target.Ticker)
	}
	for _, trade := range plan.ProposedTrades {
		tickers = append(tickers, trade.Ticker)
	}
	return mpal.NormalizeTickers(tickers)
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func sourceLabel(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func withDefaultInt32(value, fallback int32) int32 {
	if value == 0 {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mpalCapabilityCommands() []string {
	return []string{
		"capabilities", "strategy list", "strategy show", "strategy validate", "strategy run",
		"ticker events", "ticker bars", "ticker profile", "ticker financials",
		"ticker fundamentals", "ticker insiders", "ticker ownership", "ticker markov",
		"portfolio snapshot", "portfolio validate", "watchlist get", "backtest run",
		"journal append", "journal list", "journal get",
	}
}

func mpalMCPTools() []string {
	return []string{
		"mpal_capabilities", "mpal_strategy_list", "mpal_strategy_show", "mpal_strategy_validate",
		"mpal_strategy_run", "mpal_portfolio_snapshot", "mpal_watchlist_get", "mpal_ticker_bars",
		"mpal_ticker_profile", "mpal_ticker_markov", "mpal_ticker_events", "mpal_portfolio_validate", "mpal_backtest_run",
		"mpal_journal_append", "mpal_journal_list", "mpal_journal_get",
	}
}
