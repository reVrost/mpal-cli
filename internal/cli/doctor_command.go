package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	marketpalv1 "github.com/revrost/mpal-cli/gen/marketpal/v1"
	mpal "github.com/revrost/mpal-cli/pkg/mpal"
	"github.com/spf13/cobra"
)

type doctorResult struct {
	Mode      string          `json:"mode"`
	OK        bool            `json:"ok"`
	Sections  []doctorSection `json:"sections"`
	Checks    []doctorCheck   `json:"checks"`
	Errors    []string        `json:"errors,omitempty"`
	Warnings  []string        `json:"warnings,omitempty"`
	NextSteps []string        `json:"next_steps,omitempty"`
}

type doctorCheck struct {
	Category string         `json:"category"`
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

type doctorSection struct {
	Name    string        `json:"name"`
	Status  string        `json:"status"`
	Checks  []doctorCheck `json:"checks"`
	Summary string        `json:"summary,omitempty"`
}

func (a *app) doctorCommand(ctx context.Context) *cobra.Command {
	var skipAPI bool
	var strict bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local MarketPal setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := a.runDoctor(ctx, skipAPI)
			jsonOut, err := cmd.Flags().GetBool("json")
			if err != nil {
				return err
			}
			if jsonOut {
				if err := writeJSON(a.out, result); err != nil {
					return err
				}
			} else {
				if err := writeDoctorHuman(a.out, result); err != nil {
					return err
				}
			}
			if strict && !result.OK {
				return errDoctorUnhealthy{}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipAPI, "skip-api", false, "skip API reachability check")
	cmd.Flags().BoolVar(&strict, "strict", false, "return non-zero when required checks fail")
	addJSONFlag(cmd)
	return cmd
}

func (a *app) runDoctor(ctx context.Context, skipAPI bool) doctorResult {
	result := doctorResult{
		Mode: "doctor",
		OK:   true,
	}
	result.addCheck(a.doctorPathCheck("local_install", "mpal"))
	result.addCheck(a.doctorBaseURLCheck())
	result.addCheck(a.doctorMCPPluginCheck())
	result.addCheck(a.doctorPathCheck("mcp_plugin_readiness", "mpal-mcp"))
	result.addCheck(a.doctorAPIKeyCheck())
	result.addCheck(a.doctorStrategyCheck())
	result.addCheck(a.doctorJournalCheck())
	result.addCheck(a.doctorPrivatePolicyCheck())
	result.addCheck(a.doctorAPIReachabilityCheck(ctx, skipAPI))
	result.Sections = doctorSections(result.Checks)
	result.NextSteps = doctorNextSteps(result)
	return result
}

func (r *doctorResult) addCheck(check doctorCheck) {
	r.Checks = append(r.Checks, check)
	switch check.Status {
	case "error":
		r.OK = false
		r.Errors = append(r.Errors, check.Message)
	case "warning":
		r.Warnings = append(r.Warnings, check.Message)
	}
}

func (a *app) doctorPathCheck(category, name string) doctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		status := "warning"
		message := name + " is not on PATH"
		if name == "mpal" {
			message = "mpal is not on PATH; use go run ./cmd/mpal for development or install the CLI"
		}
		return doctorCheck{
			Category: category,
			Name:     name + "_path",
			Status:   status,
			Message:  message,
		}
	}
	return doctorCheck{
		Category: category,
		Name:     name + "_path",
		Status:   "ok",
		Message:  name + " found on PATH",
		Details:  map[string]any{"path": path},
	}
}

func (a *app) doctorAPIKeyCheck() doctorCheck {
	if firstNonEmpty(os.Getenv("MPAL_API_KEY"), os.Getenv("MPAL_API_KEYS")) == "" {
		return doctorCheck{
			Category: "api_availability",
			Name:     "api_key",
			Status:   "warning",
			Message:  "MPAL_API_KEY is not set; hosted API commands are unavailable, but local demo and validation commands still work",
			Details: map[string]any{
				"demo_mode_recommended": true,
				"local_commands": []string{
					"mpal doctor --skip-api",
					"mpal capabilities",
					"mpal strategy list",
					"mpal strategy show --id engine_weekly_swing_v1",
					"mpal strategy validate --config strategies/engine_weekly_swing_v1.yaml",
					"mpal portfolio validate --plan examples/final_plan.json --portfolio examples/portfolio.json --universe examples/universe.json --config strategies/momentum_profile_v1.yaml",
				},
			},
		}
	}
	return doctorCheck{
		Category: "api_availability",
		Name:     "api_key",
		Status:   "ok",
		Message:  "MPAL_API_KEY is set",
	}
}

func (a *app) doctorBaseURLCheck() doctorCheck {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MPAL_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api.marketpal.ai"
	}
	return doctorCheck{
		Category: "local_install",
		Name:     "base_url",
		Status:   "ok",
		Message:  "MarketPal API base URL resolved",
		Details:  map[string]any{"base_url": baseURL},
	}
}

func (a *app) doctorMCPPluginCheck() doctorCheck {
	details := map[string]any{
		"mcp_config":       ".mcp.json",
		"codex_plugin":     ".codex-plugin/plugin.json",
		"skills_directory": "skills",
	}
	missing := make([]string, 0)
	for _, path := range []string{".mcp.json", ".codex-plugin/plugin.json", "skills"} {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, path)
				continue
			}
			return doctorCheck{
				Category: "mcp_plugin_readiness",
				Name:     "mcp_plugin_files",
				Status:   "error",
				Message:  "MCP/plugin files cannot be inspected: " + err.Error(),
				Details:  details,
			}
		}
	}
	if len(missing) > 0 {
		details["missing"] = missing
		return doctorCheck{
			Category: "mcp_plugin_readiness",
			Name:     "mcp_plugin_files",
			Status:   "warning",
			Message:  "MCP/plugin files are incomplete in the current directory",
			Details:  details,
		}
	}
	return doctorCheck{
		Category: "mcp_plugin_readiness",
		Name:     "mcp_plugin_files",
		Status:   "ok",
		Message:  "MCP/plugin files are present in the current directory",
		Details:  details,
	}
}

func (a *app) doctorStrategyCheck() doctorCheck {
	infos, err := a.registry.List()
	if err != nil {
		return doctorCheck{
			Category: "strategy_config_readiness",
			Name:     "strategies",
			Status:   "error",
			Message:  "strategy registry could not be read: " + err.Error(),
		}
	}
	approved := 0
	apiCompatible := 0
	invalid := make([]string, 0)
	byID := make(map[string]bool, len(infos))
	for _, info := range infos {
		byID[info.ID] = true
		if info.Approved {
			approved++
		}
		if info.APICompatible {
			apiCompatible++
		}
		if !info.Validation.Valid {
			invalid = append(invalid, info.ID)
		}
	}
	missingCore := make([]string, 0)
	for _, id := range []string{"engine_weekly_swing_v1", "portfolio_low_churn_swing_v1"} {
		if !byID[id] {
			missingCore = append(missingCore, id)
		}
	}
	status := "ok"
	message := "strategy registry loaded"
	if len(invalid) > 0 {
		status = "error"
		message = "one or more strategies are invalid"
	} else if approved == 0 || apiCompatible == 0 || len(missingCore) > 0 {
		status = "warning"
		message = "strategy registry loaded with setup warnings"
	}
	return doctorCheck{
		Category: "strategy_config_readiness",
		Name:     "strategies",
		Status:   status,
		Message:  message,
		Details: map[string]any{
			"count":          len(infos),
			"approved":       approved,
			"api_compatible": apiCompatible,
			"invalid":        invalid,
			"missing_core":   missingCore,
			"user_dir":       a.registry.UserDir,
		},
	}
}

func (a *app) doctorJournalCheck() doctorCheck {
	path := a.reviewJournalPath
	if strings.TrimSpace(path) == "" {
		path = mpal.DefaultReviewJournalPath()
	}
	parent := filepath.Dir(path)
	status := "ok"
	message := "SQLite review journal path resolved"
	details := map[string]any{"path": path, "parent": parent}
	if _, err := os.Stat(parent); err != nil {
		if os.IsNotExist(err) {
			status = "warning"
			message = "SQLite review journal parent directory does not exist yet; it will be created on first use"
		} else {
			status = "error"
			message = "SQLite review journal parent directory cannot be inspected: " + err.Error()
		}
	}
	if status != "error" {
		journal, err := mpal.OpenSQLiteReviewJournal(path)
		if err != nil {
			status = "error"
			message = "SQLite review journal cannot be opened: " + err.Error()
		} else {
			defer journal.Close()
			if err := journal.Migrate(context.Background()); err != nil {
				status = "error"
				message = "SQLite review journal schema cannot be applied: " + err.Error()
			}
		}
	}
	return doctorCheck{
		Category: "journal_database_readiness",
		Name:     "review_journal",
		Status:   status,
		Message:  message,
		Details:  details,
	}
}

func (a *app) doctorPrivatePolicyCheck() doctorCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{
			Category: "local_install",
			Name:     "private_policy",
			Status:   "warning",
			Message:  "home directory could not be resolved for private portfolio policy",
		}
	}
	path := filepath.Join(home, ".marketpal", "portfolio-policy.md")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return doctorCheck{
				Category: "local_install",
				Name:     "private_policy",
				Status:   "warning",
				Message:  "optional private portfolio policy was not found",
				Details:  map[string]any{"path": path},
			}
		}
		return doctorCheck{
			Category: "local_install",
			Name:     "private_policy",
			Status:   "error",
			Message:  "private portfolio policy path cannot be inspected: " + err.Error(),
			Details:  map[string]any{"path": path},
		}
	}
	return doctorCheck{
		Category: "local_install",
		Name:     "private_policy",
		Status:   "ok",
		Message:  "private portfolio policy found",
		Details:  map[string]any{"path": path},
	}
}

func (a *app) doctorAPIReachabilityCheck(ctx context.Context, skipAPI bool) doctorCheck {
	if skipAPI {
		return doctorCheck{
			Category: "api_availability",
			Name:     "api_reachability",
			Status:   "skipped",
			Message:  "API reachability check skipped",
		}
	}
	if firstNonEmpty(os.Getenv("MPAL_API_KEY"), os.Getenv("MPAL_API_KEYS")) == "" {
		return doctorCheck{
			Category: "api_availability",
			Name:     "api_reachability",
			Status:   "skipped",
			Message:  "API reachability check skipped because MPAL_API_KEY is not set",
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := a.client.GetWatchlist(checkCtx, &marketpalv1.MpalWatchlistRequest{}); err != nil {
		return doctorCheck{
			Category: "api_availability",
			Name:     "api_reachability",
			Status:   "error",
			Message:  "MarketPal API check failed: " + err.Error(),
		}
	}
	return doctorCheck{
		Category: "api_availability",
		Name:     "api_reachability",
		Status:   "ok",
		Message:  "MarketPal API accepted a read-only watchlist request",
	}
}

func doctorSections(checks []doctorCheck) []doctorSection {
	order := []string{
		"local_install",
		"api_availability",
		"mcp_plugin_readiness",
		"journal_database_readiness",
		"strategy_config_readiness",
	}
	labels := map[string]string{
		"local_install":              "local install health",
		"api_availability":           "API availability",
		"mcp_plugin_readiness":       "MCP/plugin readiness",
		"journal_database_readiness": "journal database readiness",
		"strategy_config_readiness":  "strategy config readiness",
	}
	grouped := make(map[string][]doctorCheck, len(order))
	for _, check := range checks {
		grouped[check.Category] = append(grouped[check.Category], check)
	}
	sections := make([]doctorSection, 0, len(order))
	for _, category := range order {
		sectionChecks := grouped[category]
		section := doctorSection{
			Name:   labels[category],
			Status: doctorAggregateStatus(sectionChecks),
			Checks: sectionChecks,
		}
		section.Summary = doctorSectionSummary(section)
		sections = append(sections, section)
	}
	return sections
}

func doctorAggregateStatus(checks []doctorCheck) string {
	if len(checks) == 0 {
		return "skipped"
	}
	status := "ok"
	for _, check := range checks {
		switch check.Status {
		case "error":
			return "error"
		case "warning":
			if status != "error" {
				status = "warning"
			}
		case "skipped":
			if status == "ok" {
				status = "skipped"
			}
		case "ok":
			if status == "skipped" {
				status = "ok"
			}
		}
	}
	return status
}

func doctorSectionSummary(section doctorSection) string {
	switch section.Status {
	case "ok":
		return section.Name + " is ready"
	case "warning":
		return section.Name + " has non-blocking setup warnings"
	case "error":
		return section.Name + " has setup errors"
	default:
		return section.Name + " was skipped"
	}
}

func doctorNextSteps(result doctorResult) []string {
	steps := make([]string, 0)
	for _, check := range result.Checks {
		switch check.Name {
		case "api_key":
			if check.Status == "warning" {
				steps = append(steps, "Start in demo/local mode with mpal doctor --skip-api, mpal capabilities, and mpal strategy list.")
				steps = append(steps, "Set MPAL_API_KEY in the shell or app process when you are ready to use hosted portfolio, ticker, watchlist, backtest, or strategy API commands.")
			}
		case "mpal_path":
			if check.Status == "warning" {
				steps = append(steps, "Install the CLI with go install github.com/revrost/mpal-cli/cmd/mpal@latest or use go run ./cmd/mpal.")
			}
		case "mpal-mcp_path":
			if check.Status == "warning" {
				steps = append(steps, "Install the MCP server with go install github.com/revrost/mpal-cli/cmd/mpal-mcp@latest if using agents.")
			}
		case "mcp_plugin_files":
			if check.Status == "warning" {
				steps = append(steps, "Run mpal doctor from the plugin/repo root or reinstall the plugin package so .mcp.json, .codex-plugin/plugin.json, and skills/ are present.")
			}
		case "private_policy":
			if check.Status == "warning" {
				steps = append(steps, "Optional: create ~/.marketpal/portfolio-policy.md for sleeve rules and fixed holdings.")
			}
		case "api_reachability":
			if check.Status == "error" {
				steps = append(steps, "Check MPAL_API_KEY, MPAL_BASE_URL, network access, and MarketPal account permissions.")
			}
		}
	}
	if len(steps) == 0 {
		steps = append(steps, "Run the MarketPal trader skill for a weekly or monthly review.")
	}
	return steps
}

func writeDoctorHuman(w io.Writer, result doctorResult) error {
	status := "OK"
	if !result.OK {
		status = "ERROR"
	} else if len(result.Warnings) > 0 {
		status = "WARN"
	}
	if _, err := fmt.Fprintf(w, "MarketPal doctor: %s\n\n", status); err != nil {
		return err
	}
	for _, section := range result.Sections {
		if _, err := fmt.Fprintf(w, "%s: %s\n", section.Name, section.Status); err != nil {
			return err
		}
		for _, check := range section.Checks {
			if _, err := fmt.Fprintf(w, "  - %s: %s\n", check.Name, check.Message); err != nil {
				return err
			}
		}
	}
	if len(result.NextSteps) > 0 {
		if _, err := fmt.Fprintln(w, "\nNext steps:"); err != nil {
			return err
		}
		for _, step := range result.NextSteps {
			if _, err := fmt.Fprintf(w, "  - %s\n", step); err != nil {
				return err
			}
		}
	}
	return nil
}

type errDoctorUnhealthy struct{}

func (errDoctorUnhealthy) Error() string {
	return "mpal doctor found setup errors"
}
