package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/revrost/mpal-cli/internal/profileevidence"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/spf13/cobra"
)

func (a *app) decisionCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("decision", "missing decision subcommand")
	cmd.AddCommand(a.decisionGateCommand(ctx))
	return cmd
}

func (a *app) decisionGateCommand(ctx context.Context) *cobra.Command {
	var runArg, configPath, eventsArg, includeMarkovContext string
	var alternates int
	cmd := &cobra.Command{
		Use: "gate",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(runArg) == "" {
				return fmt.Errorf("--run is required")
			}
			run, err := mpal.LoadStrategyRunResult(runArg)
			if err != nil {
				return err
			}
			opts := mpal.DecisionGateOptions{Alternates: alternates}
			if strings.TrimSpace(configPath) != "" {
				cfg, _, err := mpal.LoadStrategyFile(configPath)
				if err != nil {
					return err
				}
				opts.Strategy = &cfg
			}
			if strings.TrimSpace(eventsArg) != "" {
				events, err := readJSONArg(eventsArg)
				if err != nil {
					return err
				}
				opts.Events = events
			}
			horizons := parseMarkovContextList(includeMarkovContext)
			if len(horizons) > 0 {
				if opts.Strategy == nil {
					return fmt.Errorf("--config is required with --include-markov-context")
				}
				tickers := mpal.DecisionGateTickers(run, alternates)
				contextResults, err := profileevidence.MarkovContexts(ctx, a.client, tickers, run.AsOf, horizons)
				if err != nil {
					return err
				}
				opts.MarkovContexts = append(opts.MarkovContexts, contextResults...)
			}
			return writeJSON(a.out, mpal.BuildDecisionGateEvidence(run, opts))
		},
	}
	cmd.Flags().StringVar(&runArg, "run", "", "strategy run path or json")
	cmd.Flags().StringVar(&configPath, "config", "", "strategy config path, required for Markov context Kelly")
	cmd.Flags().StringVar(&eventsArg, "events", "", "ticker events path or json")
	cmd.Flags().IntVar(&alternates, "alternates", 5, "maximum alternate signal candidates")
	cmd.Flags().StringVar(&includeMarkovContext, "include-markov-context", "", "comma-separated Markov context horizons: daily, weekly, monthly")
	addJSONFlag(cmd)
	return cmd
}

func parseMarkovContextList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
