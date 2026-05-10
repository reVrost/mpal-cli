package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	"github.com/revrost/mpal-cli/internal/client"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type app struct {
	out      io.Writer
	errOut   io.Writer
	registry mpal.StrategyRegistry
	journal  mpal.FileJournal
	client   client.API
}

func Main(args []string, out, errOut io.Writer) int {
	a, cleanup := newApp(out, errOut)
	if cleanup != nil {
		defer cleanup()
	}
	root := a.rootCommand(context.Background())
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errOut)
	if err := root.Execute(); err != nil {
		_ = json.NewEncoder(errOut).Encode(map[string]any{"error": err.Error()})
		return 1
	}
	return 0
}

func newApp(out, errOut io.Writer) (*app, func()) {
	registry := mpal.DefaultStrategyRegistry()
	journal := mpal.FileJournal{Path: firstNonEmpty(os.Getenv("MPAL_JOURNAL"), mpal.DefaultJournalPath())}
	return &app{
		out:      out,
		errOut:   errOut,
		registry: registry,
		journal:  journal,
		client:   client.NewFromEnv(),
	}, nil
}

func (a *app) rootCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "mpal",
		Short:         "MarketPal capability CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("missing command")
		},
	}
	cmd.AddCommand(
		a.capabilitiesCommand(),
		a.strategyCommand(ctx),
		a.tickerCommand(ctx),
		a.portfolioCommand(ctx),
		a.watchlistCommand(ctx),
		a.backtestCommand(ctx),
		a.journalCommand(ctx),
	)
	return cmd
}

func (a *app) capabilitiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(a.out, map[string]any{
				"commands":             mpalCapabilityCommands(),
				"live_trade_execution": false,
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func (a *app) strategyCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("strategy", "missing strategy subcommand")
	cmd.AddCommand(
		a.strategyListCommand(),
		a.strategyShowCommand(),
		a.strategyValidateCommand(),
		a.strategyRunCommand(ctx),
	)
	return cmd
}

func (a *app) strategyListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := a.registry.List()
			if err != nil {
				return err
			}
			return writeJSON(a.out, map[string]any{"strategies": infos})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func (a *app) strategyShowCommand() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use: "show",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, cfg, err := a.registry.Show(id)
			if err != nil {
				return err
			}
			return writeJSON(a.out, map[string]any{"strategy": info, "config": cfg})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "strategy id")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) strategyValidateCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use: "validate",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, hash, err := mpal.LoadStrategyFile(configPath)
			if err != nil {
				return err
			}
			return writeJSON(a.out, map[string]any{
				"valid":                 mpal.ValidateStrategyConfig(cfg),
				"api_compatibility":     mpal.ValidateHostedStrategyAPICompatibility(cfg),
				"api_contract":          mpal.HostedStrategyAPIContract,
				"scoring_contract":      mpal.StrategyScoringContract(cfg),
				"config_hash":           hash,
				"config_hash_algorithm": mpal.StrategyConfigHashAlgorithm,
			})
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "strategy config path")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) strategyRunCommand(ctx context.Context) *cobra.Command {
	var dateArg, universePath, portfolioPath, configPath string
	cmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			asOf, err := mpal.ParseDate(dateArg)
			if err != nil {
				return err
			}
			universe, err := mpal.LoadUniverse(universePath)
			if err != nil {
				return err
			}
			portfolio, err := mpal.LoadPortfolio(portfolioPath)
			if err != nil {
				return err
			}
			cfg, hash, err := mpal.LoadStrategyFile(configPath)
			if err != nil {
				return err
			}
			if err := mpal.EnsureHostedStrategyAPICompatible(cfg); err != nil {
				return err
			}
			wireConfig := mpal.CanonicalStrategyConfig(cfg)
			result, err := a.client.RunStrategy(ctx, &marketpalv1.MpalStrategyRunRequest{
				Date:          timestamppb.New(asOf),
				UniverseJson:  mustJSON(universe),
				PortfolioJson: mustJSON(portfolio),
				ConfigJson:    mustJSON(wireConfig),
				ConfigPath:    configPath,
				ConfigHash:    hash,
			})
			if err != nil {
				return err
			}
			return writePayload(a.out, result)
		},
	}
	cmd.Flags().StringVar(&dateArg, "date", "", "as-of date")
	cmd.Flags().StringVar(&universePath, "universe", "", "universe path")
	cmd.Flags().StringVar(&portfolioPath, "portfolio", "", "portfolio path")
	cmd.Flags().StringVar(&configPath, "config", "", "strategy config path")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) portfolioCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("portfolio", "missing portfolio subcommand")
	cmd.AddCommand(
		a.portfolioSnapshotCommand(ctx),
		a.portfolioValidateCommand(),
	)
	return cmd
}

func (a *app) portfolioSnapshotCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use: "snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := a.client.GetPortfolioSnapshot(ctx, &marketpalv1.MpalPortfolioSnapshotRequest{})
			if err != nil {
				return err
			}
			return writePayload(a.out, payload)
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func (a *app) portfolioValidateCommand() *cobra.Command {
	var planArg, portfolioPath, universePath, configPath string
	cmd := &cobra.Command{
		Use: "validate",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := mpal.LoadPlan(planArg)
			if err != nil {
				return err
			}
			portfolio := mpal.Portfolio{}
			if portfolioPath != "" {
				portfolio, err = mpal.LoadPortfolio(portfolioPath)
				if err != nil {
					return err
				}
			}
			universe := mpal.Universe{Tickers: targetTickers(plan)}
			if universePath != "" {
				universe, err = mpal.LoadUniverse(universePath)
				if err != nil {
					return err
				}
			}
			cfg, _, err := mpal.LoadStrategyFile(configPath)
			if err != nil {
				return err
			}
			return writeJSON(a.out, mpal.ValidatePlan(plan, universe, portfolio, cfg))
		},
	}
	cmd.Flags().StringVar(&planArg, "plan", "", "plan path or json")
	cmd.Flags().StringVar(&portfolioPath, "portfolio", "", "portfolio path")
	cmd.Flags().StringVar(&universePath, "universe", "", "universe path")
	cmd.Flags().StringVar(&configPath, "config", "", "strategy config path")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) watchlistCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("watchlist", "expected watchlist get")
	cmd.AddCommand(a.watchlistGetCommand(ctx))
	return cmd
}

func (a *app) watchlistGetCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := a.client.GetWatchlist(ctx, &marketpalv1.MpalWatchlistRequest{})
			if err != nil {
				return err
			}
			return writePayload(a.out, payload)
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func (a *app) backtestCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("backtest", "expected backtest run")
	cmd.AddCommand(a.backtestRunCommand(ctx))
	return cmd
}

func (a *app) backtestRunCommand(ctx context.Context) *cobra.Command {
	var startArg, endArg, universePath, configPath, benchmark string
	trustedOnly := true
	allowUntrusted := false
	cmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			start, err := mpal.ParseDate(startArg)
			if err != nil {
				return err
			}
			end, err := mpal.ParseDate(endArg)
			if err != nil {
				return err
			}
			universe, err := mpal.LoadUniverse(universePath)
			if err != nil {
				return err
			}
			cfg, hash, err := mpal.LoadStrategyFile(configPath)
			if err != nil {
				return err
			}
			if err := mpal.EnsureHostedStrategyAPICompatible(cfg); err != nil {
				return err
			}
			wireConfig := mpal.CanonicalStrategyConfig(cfg)
			result, err := a.client.RunBacktest(ctx, &marketpalv1.MpalBacktestRunRequest{
				Start:          timestamppb.New(start),
				End:            timestamppb.New(end),
				UniverseJson:   mustJSON(universe),
				ConfigJson:     mustJSON(wireConfig),
				ConfigPath:     configPath,
				ConfigHash:     hash,
				TrustedOnly:    trustedOnly,
				AllowUntrusted: allowUntrusted || !trustedOnly,
				Benchmark:      benchmark,
			})
			if err != nil {
				return err
			}
			return writePayload(a.out, result)
		},
	}
	cmd.Flags().StringVar(&startArg, "start", "", "start date")
	cmd.Flags().StringVar(&endArg, "end", "", "end date")
	cmd.Flags().StringVar(&universePath, "universe", "", "universe path")
	cmd.Flags().StringVar(&configPath, "config", "", "strategy config path")
	cmd.Flags().BoolVar(&trustedOnly, "trusted-only", true, "fail if the backtest cannot be trusted")
	cmd.Flags().BoolVar(&allowUntrusted, "allow-untrusted", false, "return diagnostic output even when trust checks fail")
	cmd.Flags().StringVar(&benchmark, "benchmark", "", "optional benchmark ticker")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) journalCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("journal", "missing journal subcommand")
	cmd.AddCommand(
		a.journalAppendCommand(ctx),
		a.journalListCommand(ctx),
		a.journalGetCommand(ctx),
	)
	return cmd
}

func (a *app) journalAppendCommand(ctx context.Context) *cobra.Command {
	var entryType, baselineID, inputArg string
	cmd := &cobra.Command{
		Use: "append",
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readJSONArg(inputArg)
			if err != nil {
				return err
			}
			entry, err := a.journal.Append(ctx, mpal.JournalEntry{
				ID:                mpal.RunID("jrnl", time.Now().UTC()),
				Type:              entryType,
				BaselineJournalID: baselineID,
				CreatedAt:         time.Now().UTC(),
				Input:             input,
			})
			if err != nil {
				return err
			}
			return writeJSON(a.out, entry)
		},
	}
	cmd.Flags().StringVar(&entryType, "type", "", "entry type")
	cmd.Flags().StringVar(&baselineID, "baseline-journal-id", "", "baseline journal id")
	cmd.Flags().StringVar(&inputArg, "input", "", "input path or json")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) journalListCommand(ctx context.Context) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := a.journal.List(ctx, limit)
			if err != nil {
				return err
			}
			return writeJSON(a.out, map[string]any{"entries": entries})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "limit")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) journalGetCommand(ctx context.Context) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use: "get",
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := a.journal.Get(ctx, id)
			if err != nil {
				return err
			}
			return writeJSON(a.out, entry)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "journal id")
	addJSONFlag(cmd)
	return cmd
}

func addJSONFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "emit json")
}

func parentCommand(use, missing string) *cobra.Command {
	return &cobra.Command{
		Use: use,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New(missing)
		},
	}
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writePayload(w io.Writer, payload string) error {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		payload = "{}"
	}
	_, err := fmt.Fprintln(w, payload)
	return err
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func mpalCapabilityCommands() []string {
	return []string{
		"capabilities", "strategy list", "strategy show", "strategy validate", "strategy run",
		"ticker events", "ticker bars", "ticker profile",
		"portfolio snapshot", "portfolio validate", "watchlist get", "backtest run",
		"journal append", "journal list", "journal get",
	}
}

func readJSONArg(pathOrJSON string) (any, error) {
	raw := []byte(strings.TrimSpace(pathOrJSON))
	if len(raw) == 0 {
		return nil, nil
	}
	if !strings.HasPrefix(string(raw), "{") && !strings.HasPrefix(string(raw), "[") {
		fileRaw, err := os.ReadFile(pathOrJSON)
		if err != nil {
			return nil, err
		}
		raw = fileRaw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseTickerCSV(value string) []string {
	parts := strings.Split(value, ",")
	tickers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tickers = append(tickers, part)
		}
	}
	return mpal.NormalizeTickers(tickers)
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
