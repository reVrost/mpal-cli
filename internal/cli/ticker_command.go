package cli

import (
	"context"
	"fmt"
	"strings"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *app) tickerCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("ticker", "missing ticker subcommand")
	cmd.AddCommand(
		a.tickerEventsCommand(ctx),
		a.tickerBarsCommand(ctx),
		a.tickerProfileCommand(ctx),
	)
	return cmd
}

func (a *app) tickerEventsCommand(ctx context.Context) *cobra.Command {
	var tickersArg, runArg, portfolioPath, scope string
	var days, limit, alternatesLimit, insightsPerTicker int
	cmd := &cobra.Command{
		Use: "events",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &marketpalv1.MpalTickerEventsRequest{
				Tickers:           parseTickerCSV(tickersArg),
				Scope:             scope,
				Days:              int32(days),
				Limit:             int32(limit),
				Alternates:        int32(alternatesLimit),
				InsightsPerTicker: int32(insightsPerTicker),
			}
			if strings.TrimSpace(runArg) != "" {
				run, err := mpal.LoadStrategyRunResult(runArg)
				if err != nil {
					return err
				}
				req.RunJson = mustJSON(run)
				if strings.TrimSpace(portfolioPath) != "" {
					portfolio, err := mpal.LoadPortfolio(portfolioPath)
					if err != nil {
						return err
					}
					req.PortfolioJson = mustJSON(portfolio)
				}
			}
			if len(req.Tickers) == 0 && strings.TrimSpace(req.RunJson) == "" && strings.TrimSpace(req.Scope) == "" {
				return fmt.Errorf("expected --tickers, --run, or --scope")
			}
			payload, err := a.client.GetTickerEvents(ctx, req)
			if err != nil {
				return err
			}
			return writePayload(a.out, payload)
		},
	}
	cmd.Flags().StringVar(&tickersArg, "tickers", "", "comma-separated tickers")
	cmd.Flags().StringVar(&runArg, "run", "", "strategy run path or json")
	cmd.Flags().StringVar(&portfolioPath, "portfolio", "", "portfolio path for --run")
	cmd.Flags().StringVar(&scope, "scope", "", "tracked ticker scope: portfolio or watchlist")
	cmd.Flags().IntVar(&days, "days", 14, "lookback days")
	cmd.Flags().IntVar(&limit, "limit", 80, "maximum source-backed updates")
	cmd.Flags().IntVar(&alternatesLimit, "alternates", 5, "maximum alternate candidates for --run")
	cmd.Flags().IntVar(&insightsPerTicker, "insights-per-ticker", 2, "maximum cached article insights per ticker for --run")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) tickerBarsCommand(ctx context.Context) *cobra.Command {
	var ticker, startArg, endArg string
	cmd := &cobra.Command{
		Use: "bars",
		RunE: func(cmd *cobra.Command, args []string) error {
			start, err := mpal.ParseDate(startArg)
			if err != nil {
				return err
			}
			end, err := mpal.ParseDate(endArg)
			if err != nil {
				return err
			}
			payload, err := a.client.GetTickerBars(ctx, &marketpalv1.MpalTickerBarsRequest{
				Ticker: ticker,
				Start:  timestamppb.New(start),
				End:    timestamppb.New(end),
			})
			if err != nil {
				return err
			}
			return writePayload(a.out, payload)
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "ticker")
	cmd.Flags().StringVar(&startArg, "start", "", "start date")
	cmd.Flags().StringVar(&endArg, "end", "", "end date")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) tickerProfileCommand(ctx context.Context) *cobra.Command {
	var ticker, dateArg string
	cmd := &cobra.Command{
		Use: "profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			asOf, err := mpal.ParseDate(dateArg)
			if err != nil {
				return err
			}
			payload, err := a.client.GetTickerProfile(ctx, &marketpalv1.MpalTickerProfileRequest{
				Ticker: ticker,
				Date:   timestamppb.New(asOf),
			})
			if err != nil {
				return err
			}
			return writePayload(a.out, payload)
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "ticker")
	cmd.Flags().StringVar(&dateArg, "date", "", "date")
	addJSONFlag(cmd)
	return cmd
}
