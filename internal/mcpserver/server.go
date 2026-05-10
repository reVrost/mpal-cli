package mcpserver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const Version = "0.1.0"

type Config struct {
	Registry mpal.StrategyRegistry
	Journal  mpal.FileJournal
	Client   client.API
}

func DefaultConfig() Config {
	return Config{
		Registry: mpal.DefaultStrategyRegistry(),
		Journal:  mpal.FileJournal{Path: firstNonEmpty(os.Getenv("MPAL_JOURNAL"), mpal.DefaultJournalPath())},
		Client:   client.NewFromEnv(),
	}
}

func RunStdio(ctx context.Context, cfg Config) error {
	return New(cfg).Run(ctx, &mcp.StdioTransport{})
}

func New(cfg Config) *mcp.Server {
	if cfg.Registry.UserDir == "" {
		cfg.Registry = mpal.DefaultStrategyRegistry()
	}
	if cfg.Journal.Path == "" {
		cfg.Journal = mpal.FileJournal{Path: mpal.DefaultJournalPath()}
	}
	if cfg.Client == nil {
		cfg.Client = client.NewFromEnv()
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "mpal", Version: Version}, nil)
	registerTools(server, cfg)
	return server
}

func registerTools(server *mcp.Server, cfg Config) {
	mcp.AddTool(server, readOnlyTool("mpal_capabilities", "List deterministic MarketPal capabilities. This server never executes live trades."), capabilitiesTool)
	mcp.AddTool(server, readOnlyTool("mpal_strategy_list", "List built-in and user strategy configs."), func(ctx context.Context, req *mcp.CallToolRequest, in noInput) (*mcp.CallToolResult, any, error) {
		infos, err := cfg.Registry.List()
		if err != nil {
			return nil, nil, err
		}
		return nil, object(map[string]any{"strategies": infos}), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_strategy_show", "Show a strategy config by approved strategy ID."), func(ctx context.Context, req *mcp.CallToolRequest, in strategyShowInput) (*mcp.CallToolResult, any, error) {
		info, strategy, err := cfg.Registry.Show(in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(map[string]any{"strategy": info, "config": strategy}), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_strategy_validate", "Validate a strategy config from a path or inline YAML/JSON."), func(ctx context.Context, req *mcp.CallToolRequest, in strategyValidateInput) (*mcp.CallToolResult, any, error) {
		strategy, hash, err := loadStrategy(in.ConfigPath, in.ConfigJSON)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(map[string]any{
			"valid":                 mpal.ValidateStrategyConfig(strategy),
			"api_compatibility":     mpal.ValidateHostedStrategyAPICompatibility(strategy),
			"api_contract":          mpal.HostedStrategyAPIContract,
			"scoring_contract":      mpal.StrategyScoringContract(strategy),
			"config_hash":           hash,
			"config_hash_algorithm": mpal.StrategyConfigHashAlgorithm,
		}), nil
	})
	mcp.AddTool(server, additiveTool("mpal_strategy_run", "Run an explicit versioned MarketPal strategy config and return the baseline plan. This can append a MarketPal journal entry, but cannot execute live trades."), func(ctx context.Context, req *mcp.CallToolRequest, in strategyRunInput) (*mcp.CallToolResult, any, error) {
		asOf, err := mpal.ParseDate(in.Date)
		if err != nil {
			return nil, nil, err
		}
		universe, err := loadUniverse(in.UniversePath, in.UniverseJSON)
		if err != nil {
			return nil, nil, err
		}
		portfolio, err := loadPortfolio(in.PortfolioPath, in.PortfolioJSON)
		if err != nil {
			return nil, nil, err
		}
		strategy, hash, err := loadStrategy(in.ConfigPath, in.ConfigJSON)
		if err != nil {
			return nil, nil, err
		}
		if validation := mpal.ValidateStrategyConfig(strategy); !validation.Valid {
			return nil, nil, fmt.Errorf("invalid strategy config: %s", strings.Join(validation.Errors, "; "))
		}
		if err := mpal.EnsureHostedStrategyAPICompatible(strategy); err != nil {
			return nil, nil, err
		}
		wireConfig := mpal.CanonicalStrategyConfig(strategy)
		payload, err := cfg.Client.RunStrategy(ctx, &marketpalv1.MpalStrategyRunRequest{
			Date:          timestamppb.New(asOf),
			UniverseJson:  mustJSON(universe),
			PortfolioJson: mustJSON(portfolio),
			ConfigJson:    mustJSON(wireConfig),
			ConfigPath:    sourceLabel(in.ConfigPath, "inline"),
			ConfigHash:    hash,
		})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_portfolio_snapshot", "Fetch the authenticated user's current MarketPal portfolio snapshot."), func(ctx context.Context, req *mcp.CallToolRequest, in noInput) (*mcp.CallToolResult, any, error) {
		payload, err := cfg.Client.GetPortfolioSnapshot(ctx, &marketpalv1.MpalPortfolioSnapshotRequest{})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_watchlist_get", "Fetch the authenticated user's current MarketPal watchlist universe."), func(ctx context.Context, req *mcp.CallToolRequest, in noInput) (*mcp.CallToolResult, any, error) {
		payload, err := cfg.Client.GetWatchlist(ctx, &marketpalv1.MpalWatchlistRequest{})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_ticker_bars", "Fetch historical bars for a ticker and expose freshness/staleness metadata returned by MarketPal."), func(ctx context.Context, req *mcp.CallToolRequest, in tickerBarsInput) (*mcp.CallToolResult, any, error) {
		start, err := mpal.ParseDate(in.Start)
		if err != nil {
			return nil, nil, err
		}
		end, err := mpal.ParseDate(in.End)
		if err != nil {
			return nil, nil, err
		}
		payload, err := cfg.Client.GetTickerBars(ctx, &marketpalv1.MpalTickerBarsRequest{Ticker: in.Ticker, Start: timestamppb.New(start), End: timestamppb.New(end)})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_ticker_profile", "Fetch MarketPal profile/fundamental scoring for a ticker and date."), func(ctx context.Context, req *mcp.CallToolRequest, in tickerProfileInput) (*mcp.CallToolResult, any, error) {
		asOf, err := mpal.ParseDate(in.Date)
		if err != nil {
			return nil, nil, err
		}
		payload, err := cfg.Client.GetTickerProfile(ctx, &marketpalv1.MpalTickerProfileRequest{Ticker: in.Ticker, Date: timestamppb.New(asOf)})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_ticker_events", "Fetch source-backed ticker events for explicit tickers, a strategy run, or a tracked portfolio/watchlist scope."), func(ctx context.Context, req *mcp.CallToolRequest, in tickerEventsInput) (*mcp.CallToolResult, any, error) {
		message, err := tickerEventsRequest(in)
		if err != nil {
			return nil, nil, err
		}
		payload, err := cfg.Client.GetTickerEvents(ctx, message)
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_portfolio_validate", "Validate a final plan against strategy, universe, and portfolio risk rules. This does not execute trades."), func(ctx context.Context, req *mcp.CallToolRequest, in portfolioValidateInput) (*mcp.CallToolResult, any, error) {
		plan, err := loadPlan(in.PlanPath, in.PlanJSON)
		if err != nil {
			return nil, nil, err
		}
		portfolio := mpal.Portfolio{}
		if in.PortfolioPath != "" || in.PortfolioJSON != "" {
			portfolio, err = loadPortfolio(in.PortfolioPath, in.PortfolioJSON)
			if err != nil {
				return nil, nil, err
			}
		}
		universe := mpal.Universe{Tickers: targetTickers(plan)}
		if in.UniversePath != "" || in.UniverseJSON != "" {
			universe, err = loadUniverse(in.UniversePath, in.UniverseJSON)
			if err != nil {
				return nil, nil, err
			}
		}
		strategy, _, err := loadStrategy(in.ConfigPath, in.ConfigJSON)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(mpal.ValidatePlan(plan, universe, portfolio, strategy)), nil
	})
	mcp.AddTool(server, additiveTool("mpal_backtest_run", "Run a backtest for an explicit strategy config. This reuses the same planning logic as strategy runs and can append a journal entry."), func(ctx context.Context, req *mcp.CallToolRequest, in backtestRunInput) (*mcp.CallToolResult, any, error) {
		start, err := mpal.ParseDate(in.Start)
		if err != nil {
			return nil, nil, err
		}
		end, err := mpal.ParseDate(in.End)
		if err != nil {
			return nil, nil, err
		}
		universe, err := loadUniverse(in.UniversePath, in.UniverseJSON)
		if err != nil {
			return nil, nil, err
		}
		strategy, hash, err := loadStrategy(in.ConfigPath, in.ConfigJSON)
		if err != nil {
			return nil, nil, err
		}
		if err := mpal.EnsureHostedStrategyAPICompatible(strategy); err != nil {
			return nil, nil, err
		}
		wireConfig := mpal.CanonicalStrategyConfig(strategy)
		payload, err := cfg.Client.RunBacktest(ctx, &marketpalv1.MpalBacktestRunRequest{
			Start:          timestamppb.New(start),
			End:            timestamppb.New(end),
			UniverseJson:   mustJSON(universe),
			ConfigJson:     mustJSON(wireConfig),
			ConfigPath:     sourceLabel(in.ConfigPath, "inline"),
			ConfigHash:     hash,
			TrustedOnly:    !in.AllowUntrusted,
			AllowUntrusted: in.AllowUntrusted,
			Benchmark:      in.Benchmark,
		})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, additiveTool("mpal_journal_append", "Append a local structured MarketPal agent decision journal entry."), func(ctx context.Context, req *mcp.CallToolRequest, in journalAppendInput) (*mcp.CallToolResult, any, error) {
		input, err := readJSONInput(in.InputPath, in.InputJSON)
		if err != nil {
			return nil, nil, err
		}
		entry, err := cfg.Journal.Append(ctx, mpal.JournalEntry{
			ID:                mpal.RunID("jrnl", time.Now().UTC()),
			Type:              in.Type,
			BaselineJournalID: in.BaselineJournalID,
			CreatedAt:         time.Now().UTC(),
			Input:             input,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, object(entry), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_journal_list", "List recent local MarketPal journal entries."), func(ctx context.Context, req *mcp.CallToolRequest, in journalListInput) (*mcp.CallToolResult, any, error) {
		entries, err := cfg.Journal.List(ctx, in.Limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(map[string]any{"entries": entries}), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_journal_get", "Get a local MarketPal journal entry by ID."), func(ctx context.Context, req *mcp.CallToolRequest, in journalGetInput) (*mcp.CallToolResult, any, error) {
		entry, err := cfg.Journal.Get(ctx, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(entry), nil
	})
}
