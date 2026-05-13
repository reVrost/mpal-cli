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
	"github.com/revrost/mpal-cli/internal/profileevidence"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const Version = "0.1.0"

type Config struct {
	Registry          mpal.StrategyRegistry
	Journal           mpal.FileJournal
	ReviewJournalPath string
	Client            client.API
}

func DefaultConfig() Config {
	return Config{
		Registry:          mpal.DefaultStrategyRegistry(),
		Journal:           mpal.FileJournal{Path: firstNonEmpty(os.Getenv("MPAL_JOURNAL"), mpal.DefaultJournalPath())},
		ReviewJournalPath: firstNonEmpty(os.Getenv("MPAL_REVIEW_JOURNAL"), os.Getenv("MPAL_DB"), mpal.DefaultReviewJournalPath()),
		Client:            client.NewFromEnv(),
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
	if cfg.ReviewJournalPath == "" {
		cfg.ReviewJournalPath = mpal.DefaultReviewJournalPath()
	}
	if cfg.Client == nil {
		cfg.Client = client.NewFromEnv()
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "mpal", Version: Version}, nil)
	registerTools(server, cfg)
	return server
}

func registerTools(server *mcp.Server, cfg Config) {
	mcp.AddTool(server, readOnlyTool("mpal_capabilities", "List deterministic Marketpal capabilities. This server never executes live trades."), capabilitiesTool)
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
	mcp.AddTool(server, additiveTool("mpal_strategy_run", "Run an explicit versioned Marketpal strategy config, auto-journal the baseline packet, and return the baseline plan. This cannot execute live trades."), func(ctx context.Context, req *mcp.CallToolRequest, in strategyRunInput) (*mcp.CallToolResult, any, error) {
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
		strategyText, err := loadStrategyText(in.ConfigPath, in.ConfigJSON)
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
		run, err := mpal.LoadStrategyRunResult(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("strategy run returned JSON that could not be auto-journaled: %w", err)
		}
		if run.AsOf.IsZero() {
			run.AsOf = asOf
		}
		if run.Strategy.ID == "" {
			run.Strategy.ID = strategy.ID
		}
		if run.Strategy.Version == "" {
			run.Strategy.Version = strategy.Version
		}
		if run.Strategy.ConfigHash == "" {
			run.Strategy.ConfigHash = hash
		}
		run.Strategy.Approved = run.Strategy.Approved || strategy.Approved
		if run.ExecutionResult == "" {
			run.ExecutionResult = firstNonEmpty(run.Result, run.BaselinePlan.Result, mpal.ResultNoTrade)
		}
		if run.Result == "" {
			run.Result = run.ExecutionResult
		}
		reviewInput := mpal.TradeReviewStartInputFromStrategyRun(run, strategy, strategyText, universe, asOf)
		review, positions, err := reviewInput.ToCreateParams(time.Now().UTC())
		if err != nil {
			return nil, nil, err
		}
		journal, err := openReviewJournal(ctx, cfg.ReviewJournalPath)
		if err != nil {
			return nil, nil, err
		}
		defer journal.Close()
		if err := journal.AppendReview(ctx, review, positions); err != nil {
			return nil, nil, err
		}
		run.JournalEntryID = review.ID
		return nil, object(run), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_portfolio_snapshot", "Fetch the authenticated user's current Marketpal portfolio snapshot."), func(ctx context.Context, req *mcp.CallToolRequest, in noInput) (*mcp.CallToolResult, any, error) {
		payload, err := cfg.Client.GetPortfolioSnapshot(ctx, &marketpalv1.MpalPortfolioSnapshotRequest{})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_portfolio_transactions", "Fetch the authenticated user's Marketpal portfolio transactions."), func(ctx context.Context, req *mcp.CallToolRequest, in portfolioTransactionsInput) (*mcp.CallToolResult, any, error) {
		payload, err := cfg.Client.GetPortfolioTransactions(ctx, &marketpalv1.MpalPortfolioTransactionsRequest{
			Page:  in.Page,
			Limit: in.Limit,
		})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_watchlist_get", "Fetch the authenticated user's current Marketpal watchlist universe."), func(ctx context.Context, req *mcp.CallToolRequest, in noInput) (*mcp.CallToolResult, any, error) {
		payload, err := cfg.Client.GetWatchlist(ctx, &marketpalv1.MpalWatchlistRequest{})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_ticker_bars", "Fetch historical bars for a ticker and expose freshness/staleness metadata returned by Marketpal."), func(ctx context.Context, req *mcp.CallToolRequest, in tickerBarsInput) (*mcp.CallToolResult, any, error) {
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
	mcp.AddTool(server, readOnlyTool("mpal_ticker_profile", "Fetch Marketpal profile/fundamental scoring for one or more tickers and a date."), func(ctx context.Context, req *mcp.CallToolRequest, in tickerProfileInput) (*mcp.CallToolResult, any, error) {
		asOf, err := mpal.ParseDate(in.Date)
		if err != nil {
			return nil, nil, err
		}
		tickers := mpal.NormalizeTickers(append([]string{in.Ticker}, in.Tickers...))
		if len(tickers) == 0 {
			return nil, nil, fmt.Errorf("expected ticker or tickers")
		}
		payload, err := cfg.Client.GetTickerProfile(ctx, &marketpalv1.MpalTickerProfileRequest{Ticker: tickers[0], Tickers: tickers, Date: timestamppb.New(asOf)})
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
			Start:                 timestamppb.New(start),
			End:                   timestamppb.New(end),
			UniverseJson:          mustJSON(universe),
			ConfigJson:            mustJSON(wireConfig),
			ConfigPath:            sourceLabel(in.ConfigPath, "inline"),
			ConfigHash:            hash,
			TrustedOnly:           !in.AllowUntrusted,
			AllowUntrusted:        in.AllowUntrusted,
			Benchmark:             in.Benchmark,
			SnapshotFreshnessDays: in.SnapshotFreshnessDays,
			ProfileVersion:        in.ProfileVersion,
		})
		if err != nil {
			return nil, nil, err
		}
		out, err := decodePayload(payload)
		return nil, out, err
	})
	mcp.AddTool(server, readOnlyTool("mpal_decision_gate", "Build deterministic evidence for an agent decision gate from a previous strategy run. This does not execute or modify trades."), func(ctx context.Context, req *mcp.CallToolRequest, in decisionGateInput) (*mcp.CallToolResult, any, error) {
		runArg := firstNonEmpty(in.RunPath, in.RunJSON)
		if runArg == "" {
			return nil, nil, fmt.Errorf("expected run_path or run_json")
		}
		run, err := mpal.LoadStrategyRunResult(runArg)
		if err != nil {
			return nil, nil, err
		}
		alternates := 5
		if in.Alternates != nil {
			alternates = *in.Alternates
		}
		opts := mpal.DecisionGateOptions{Alternates: alternates}
		horizons := parseDecisionGateMarkovContext(in.IncludeMarkovContext)
		if in.ConfigPath != "" || in.ConfigJSON != "" {
			strategy, _, err := loadStrategy(in.ConfigPath, in.ConfigJSON)
			if err != nil {
				return nil, nil, err
			}
			opts.Strategy = &strategy
		}
		if len(horizons) > 0 && opts.Strategy == nil {
			return nil, nil, fmt.Errorf("config_path or config_json is required with include_markov_context")
		}
		if in.EventsPath != "" || in.EventsJSON != "" {
			events, err := readJSONInput(in.EventsPath, in.EventsJSON)
			if err != nil {
				return nil, nil, err
			}
			opts.Events = events
		}
		if len(horizons) > 0 {
			tickers := mpal.DecisionGateTickers(run, alternates)
			contextResults, err := profileevidence.MarkovContexts(ctx, cfg.Client, tickers, run.AsOf, horizons)
			if err != nil {
				return nil, nil, err
			}
			opts.MarkovContexts = append(opts.MarkovContexts, contextResults...)
		}
		return nil, object(mpal.BuildDecisionGateEvidence(run, opts)), nil
	})
	mcp.AddTool(server, additiveTool("mpal_journal_start", "Start a durable SQLite trade review journal entry from model and agent review data."), func(ctx context.Context, req *mcp.CallToolRequest, in journalStartInput) (*mcp.CallToolResult, any, error) {
		raw, err := readJSONInput(in.InputPath, in.InputJSON)
		if err != nil {
			return nil, nil, err
		}
		var input mpal.TradeReviewStartInput
		if err := decodeObject(raw, &input); err != nil {
			return nil, nil, err
		}
		review, positions, err := input.ToCreateParams(time.Now().UTC())
		if err != nil {
			return nil, nil, err
		}
		journal, err := openReviewJournal(ctx, cfg.ReviewJournalPath)
		if err != nil {
			return nil, nil, err
		}
		defer journal.Close()
		if err := journal.AppendReview(ctx, review, positions); err != nil {
			return nil, nil, err
		}
		got, gotPositions, err := journal.GetReview(ctx, review.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(mpal.ReviewJournalOutput(got, gotPositions)), nil
	})
	mcp.AddTool(server, additiveTool("mpal_journal_finalize", "Finalize a durable SQLite trade review journal entry with the human decision."), func(ctx context.Context, req *mcp.CallToolRequest, in journalFinalizeInput) (*mcp.CallToolResult, any, error) {
		raw, err := readJSONInput(in.InputPath, in.InputJSON)
		if err != nil {
			return nil, nil, err
		}
		var input mpal.TradeReviewFinalizeInput
		if err := decodeObject(raw, &input); err != nil {
			return nil, nil, err
		}
		final, positions, err := input.ToFinalizeParams(in.ID, time.Now().UTC())
		if err != nil {
			return nil, nil, err
		}
		journal, err := openReviewJournal(ctx, cfg.ReviewJournalPath)
		if err != nil {
			return nil, nil, err
		}
		defer journal.Close()
		if err := journal.FinalizeReview(ctx, final, positions); err != nil {
			return nil, nil, err
		}
		got, gotPositions, err := journal.GetReview(ctx, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(mpal.ReviewJournalOutput(got, gotPositions)), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_journal_list", "List recent local Marketpal journal entries."), func(ctx context.Context, req *mcp.CallToolRequest, in journalListInput) (*mcp.CallToolResult, any, error) {
		journal, err := openReviewJournal(ctx, cfg.ReviewJournalPath)
		if err != nil {
			return nil, nil, err
		}
		defer journal.Close()
		reviews, err := journal.ListReviews(ctx, int64(in.Limit))
		if err != nil {
			return nil, nil, err
		}
		return nil, object(mpal.ReviewListOutput(reviews)), nil
	})
	mcp.AddTool(server, readOnlyTool("mpal_journal_get", "Get a local Marketpal journal entry by ID."), func(ctx context.Context, req *mcp.CallToolRequest, in journalGetInput) (*mcp.CallToolResult, any, error) {
		journal, err := openReviewJournal(ctx, cfg.ReviewJournalPath)
		if err != nil {
			return nil, nil, err
		}
		defer journal.Close()
		review, positions, err := journal.GetReview(ctx, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, object(mpal.ReviewJournalOutput(review, positions)), nil
	})
}

func openReviewJournal(ctx context.Context, path string) (*mpal.SQLiteReviewJournal, error) {
	journal, err := mpal.OpenSQLiteReviewJournal(path)
	if err != nil {
		return nil, err
	}
	if err := journal.Migrate(ctx); err != nil {
		_ = journal.Close()
		return nil, err
	}
	return journal, nil
}
