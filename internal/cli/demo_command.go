package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	demofixtures "github.com/revrost/mpal-cli/examples/demo"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/spf13/cobra"
)

const (
	demoOutputDir = "tmp/mpal-demo"
	demoReviewID  = "demo_review_20260511"
)

type demoWorkflowOutput struct {
	Demo        bool                         `json:"demo"`
	DataSource  string                       `json:"data_source"`
	Label       string                       `json:"label"`
	NoLiveAPI   bool                         `json:"no_live_api"`
	OutputDir   string                       `json:"output_dir"`
	Fixtures    map[string]string            `json:"fixtures"`
	Artifacts   map[string]string            `json:"artifacts"`
	SetupCheck  map[string]any               `json:"setup_check"`
	Steps       []demoWorkflowStep           `json:"steps"`
	StrategyRun mpal.StrategyRunResult       `json:"strategy_run"`
	Decision    mpal.DecisionGateResult      `json:"decision_gate"`
	Validation  mpal.ValidationResult        `json:"validation"`
	Report      mpal.TradeReviewReportResult `json:"report"`
	Journal     map[string]any               `json:"journal"`
}

type demoWorkflowStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func (a *app) demoCommand(ctx context.Context) *cobra.Command {
	cmd := parentCommand("demo", "missing demo subcommand")
	cmd.AddCommand(
		a.demoRunCommand(ctx),
		a.demoReportCommand(ctx),
		a.demoJournalCommand(ctx),
	)
	return cmd
}

func (a *app) demoRunCommand(ctx context.Context) *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the no-key fixture workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.runDemoWorkflow(ctx, outputDir)
			if err != nil {
				return err
			}
			jsonOut, err := cmd.Flags().GetBool("json")
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(a.out, result)
			}
			_, err = fmt.Fprintf(a.out, "MarketPal demo complete\nReport: %s\nJournal: %s\n", result.Report.ReportPath, result.Artifacts["journal_db"])
			return err
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", demoOutputDir, "demo artifact output directory")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) demoReportCommand(ctx context.Context) *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate the no-key demo HTML report",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.runDemoWorkflow(ctx, outputDir)
			if err != nil {
				return err
			}
			jsonOut, err := cmd.Flags().GetBool("json")
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(a.out, result.Report)
			}
			_, err = fmt.Fprintln(a.out, result.Report.ReportPath)
			return err
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", demoOutputDir, "demo artifact output directory")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) demoJournalCommand(ctx context.Context) *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Show the finalized no-key demo journal entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.runDemoWorkflow(ctx, outputDir)
			if err != nil {
				return err
			}
			jsonOut, err := cmd.Flags().GetBool("json")
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(a.out, result.Journal)
			}
			review, _ := result.Journal["review"].(map[string]any)
			_, err = fmt.Fprintf(a.out, "Demo journal finalized: %s\n", review["id"])
			return err
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", demoOutputDir, "demo artifact output directory")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) runDemoWorkflow(ctx context.Context, outputDir string) (demoWorkflowOutput, error) {
	if outputDir == "" {
		outputDir = demoOutputDir
	}
	dbPath := filepath.Join(outputDir, "demo-mpal.db")
	reportPath := filepath.Join(outputDir, "demo-trade-review.html")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return demoWorkflowOutput{}, err
	}
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return demoWorkflowOutput{}, err
	}

	portfolio, err := demoFixture[mpal.Portfolio](demofixtures.PortfolioJSON)
	if err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo portfolio: %w", err)
	}
	universe, err := demoFixture[mpal.Universe](demofixtures.UniverseJSON)
	if err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo universe: %w", err)
	}
	cfg, _, err := mpal.LoadStrategyBytes(demofixtures.StrategyYAML)
	if err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo strategy: %w", err)
	}
	run, err := demoFixture[mpal.StrategyRunResult](demofixtures.StrategyRunJSON)
	if err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo strategy run: %w", err)
	}
	var events any
	if err := json.Unmarshal(demofixtures.TickerEventsJSON, &events); err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo ticker events: %w", err)
	}
	finalPlan, err := demoFixture[mpal.PortfolioPlanResult](demofixtures.FinalPlanJSON)
	if err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo final plan: %w", err)
	}
	finalInput, err := demoFixture[mpal.TradeReviewFinalizeInput](demofixtures.FinalJournalJSON)
	if err != nil {
		return demoWorkflowOutput{}, fmt.Errorf("load demo journal finalization: %w", err)
	}

	decision := mpal.BuildDecisionGateEvidence(run, mpal.DecisionGateOptions{
		Alternates: 1,
		Strategy:   &cfg,
		Events:     events,
	})
	decision.RunID = "demo_decision_gate_20260511"
	validation := mpal.ValidatePlan(finalPlan, universe, portfolio, cfg)

	journal, err := mpal.OpenSQLiteReviewJournal(dbPath)
	if err != nil {
		return demoWorkflowOutput{}, err
	}
	defer journal.Close()
	if err := journal.Migrate(ctx); err != nil {
		return demoWorkflowOutput{}, err
	}

	startInput := mpal.TradeReviewStartInputFromStrategyRun(run, cfg, string(demofixtures.StrategyYAML), universe, run.AsOf)
	startInput.ID = demoReviewID
	startInput.PortfolioScope = "custom"
	startInput.AgentHarness = "mpal-demo"
	startInput.AgentModel = "fixture"
	startInput.AgentSkill = "marketpal-onboarding"
	startInput.UserPromptText = "mpal demo run --json"
	startInput.ChatHistoryText = "No-key demo fixture workflow."
	startInput.AgentSummary = "Demo fixture strategy packet reviewed; final plan validates without live API access."
	for i := range startInput.Positions {
		startInput.Positions[i].ID = fmt.Sprintf("demo_pos_%03d", i+1)
	}
	review, positions, err := startInput.ToCreateParams(run.AsOf)
	if err != nil {
		return demoWorkflowOutput{}, err
	}
	if err := journal.AppendReview(ctx, review, positions); err != nil {
		return demoWorkflowOutput{}, err
	}
	got, gotPositions, err := journal.GetReview(ctx, demoReviewID)
	if err != nil {
		return demoWorkflowOutput{}, err
	}
	report, err := mpal.WriteTradeReviewHTMLReport(got, gotPositions, mpal.TradeReviewReportOptions{
		OutputPath: reportPath,
		Notes:      "Demo fixture data. No live MarketPal API call was made.",
	})
	if err != nil {
		return demoWorkflowOutput{}, err
	}
	if err := journal.SetReportPath(ctx, demoReviewID, report.ReportPath); err != nil {
		return demoWorkflowOutput{}, err
	}
	finalInput.ReportPath = report.ReportPath
	final, finalPositions, err := finalInput.ToFinalizeParams(demoReviewID, run.AsOf)
	if err != nil {
		return demoWorkflowOutput{}, err
	}
	if err := journal.FinalizeReview(ctx, final, finalPositions); err != nil {
		return demoWorkflowOutput{}, err
	}
	finalReview, finalReviewPositions, err := journal.GetReview(ctx, demoReviewID)
	if err != nil {
		return demoWorkflowOutput{}, err
	}
	journalOut := mpal.ReviewJournalOutput(finalReview, finalReviewPositions)
	normalizeDemoJournalOutput(journalOut)

	return demoWorkflowOutput{
		Demo:       true,
		DataSource: "fixture",
		Label:      "Demo fixture data. No live MarketPal API call was made.",
		NoLiveAPI:  true,
		OutputDir:  outputDir,
		Fixtures: map[string]string{
			"portfolio":     "examples/demo/portfolio.json",
			"universe":      "examples/demo/universe.json",
			"strategy":      "examples/demo/strategy.yaml",
			"strategy_run":  "examples/demo/strategy_run.json",
			"ticker_events": "examples/demo/ticker_events.json",
			"final_plan":    "examples/demo/final_plan.json",
			"final_journal": "examples/demo/final_journal.json",
		},
		Artifacts: map[string]string{
			"journal_db": dbPath,
			"report":     report.ReportPath,
		},
		SetupCheck: map[string]any{
			"status":           "ok",
			"api_key_required": false,
			"api_reachability": "skipped",
			"api_key_used":     false,
		},
		Steps: []demoWorkflowStep{
			{Name: "setup_check", Status: "ok", Detail: "local fixture workflow; API skipped"},
			{Name: "strategy_packet", Status: "ok", Detail: run.RunID},
			{Name: "decision_gate", Status: "ok", Detail: decision.EvidenceHash},
			{Name: "validation", Status: demoValidationStatus(validation)},
			{Name: "report", Status: "ok", Detail: report.ReportPath},
			{Name: "journal_finalization", Status: "ok", Detail: demoReviewID},
		},
		StrategyRun: run,
		Decision:    decision,
		Validation:  validation,
		Report:      report,
		Journal:     journalOut,
	}, nil
}

func demoFixture[T any](raw []byte) (T, error) {
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, err
	}
	return value, nil
}

func demoValidationStatus(validation mpal.ValidationResult) string {
	if validation.Valid {
		return "ok"
	}
	return "error"
}

func normalizeDemoJournalOutput(out map[string]any) {
	review, ok := out["review"].(map[string]any)
	if !ok {
		return
	}
	review["created_at"] = "2026-05-11T00:00:00.000Z"
}
