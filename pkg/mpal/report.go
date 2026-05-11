package mpal

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/revrost/mpal-cli/pkg/mpal/sqlitejournal"
)

//go:embed templates/trade_review.html.go.tpl
var tradeReviewHTMLTemplate string

type TradeReviewReportOptions struct {
	OutputPath string
	Notes      string
}

type TradeReviewReportResult struct {
	TradeReviewID string `json:"trade_review_id"`
	ReportPath    string `json:"report_path"`
	PositionCount int    `json:"position_count"`
}

func DefaultTradeReviewReportPath(review sqlitejournal.TradeReview) string {
	base := ".marketpal"
	if home, _ := os.UserHomeDir(); home != "" {
		base = filepath.Join(home, ".marketpal")
	}
	asOf := safePathSegment(review.AsOf)
	if asOf == "" {
		asOf = "unknown-date"
	}
	id := safePathSegment(review.ID)
	if id == "" {
		id = "trade-review"
	}
	return filepath.Join(base, "reports", asOf, "trade-review-"+id+".html")
}

func WriteTradeReviewHTMLReport(review sqlitejournal.TradeReview, positions []sqlitejournal.TradeReviewPosition, opts TradeReviewReportOptions) (TradeReviewReportResult, error) {
	path := strings.TrimSpace(opts.OutputPath)
	if path == "" {
		path = DefaultTradeReviewReportPath(review)
	}
	html, err := RenderTradeReviewHTMLReport(review, positions, opts)
	if err != nil {
		return TradeReviewReportResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return TradeReviewReportResult{}, err
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return TradeReviewReportResult{}, err
	}
	return TradeReviewReportResult{
		TradeReviewID: review.ID,
		ReportPath:    path,
		PositionCount: len(positions),
	}, nil
}

func RenderTradeReviewHTMLReport(review sqlitejournal.TradeReview, positions []sqlitejournal.TradeReviewPosition, opts TradeReviewReportOptions) (string, error) {
	data := tradeReviewReportData{
		ID:              review.ID,
		AsOf:            review.AsOf,
		StrategyID:      review.StrategyID,
		ExecutionResult: review.ExecutionResult,
		FinalDecision:   displayNullString(review.FinalDecision),
		AgentSummary:    displayMultiline(review.AgentSummary),
		HumanReasoning:  displayMultiline(review.HumanReasoningText),
		Warnings:        displayMultiline(review.WarningsText),
		Notes:           strings.TrimSpace(opts.Notes),
		Positions:       make([]tradeReviewReportPosition, 0, len(positions)),
	}
	for _, position := range positions {
		data.Positions = append(data.Positions, tradeReviewReportPositionFromDB(position))
	}
	data.TradeCount = countReportTrades(data.Positions)
	tpl, err := template.New("trade_review").Parse(tradeReviewHTMLTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type tradeReviewReportData struct {
	ID              string
	AsOf            string
	StrategyID      string
	ExecutionResult string
	FinalDecision   string
	AgentSummary    string
	HumanReasoning  string
	Warnings        string
	Notes           string
	TradeCount      int
	Positions       []tradeReviewReportPosition
}

type tradeReviewReportPosition struct {
	Ticker            string
	IsTrade           string
	Decision          string
	Score             string
	Role              string
	Intent            string
	SizingMethod      string
	RawKelly          string
	FractionalKelly   string
	KellyTargetWeight string
	AcceptedSizing    string
	EstimatedValue    string
	BindingConstraint string
	CalibrationStatus string
	Read              string
	DecisionClass     string
}

func tradeReviewReportPositionFromDB(position sqlitejournal.TradeReviewPosition) tradeReviewReportPosition {
	decision := inferredReviewDecision(position)
	return tradeReviewReportPosition{
		Ticker:            position.Ticker,
		IsTrade:           yesNo(decision == "trade"),
		Decision:          displayText(decision),
		Score:             displayScore(position.ModelScore),
		Role:              displayNullString(position.ModelBucket),
		Intent:            displayNullString(position.ModelIntent),
		SizingMethod:      displayNullString(position.SizingMethod),
		RawKelly:          displayPct(position.RawKelly),
		FractionalKelly:   displayPct(position.FractionalKelly),
		KellyTargetWeight: displayPct(position.KellyTargetWeight),
		AcceptedSizing:    displayPct(firstValidFloat(position.HumanWeight, position.AgentWeight, position.FinalTargetWeight, position.ModelWeight)),
		EstimatedValue:    displayNumber(position.ModelEstimatedValue),
		BindingConstraint: displayNullString(position.BindingConstraint),
		CalibrationStatus: displayNullString(position.CalibrationStatus),
		Read:              firstDisplayString(position.HumanReason, position.AgentReason, position.ModelReason),
		DecisionClass:     decisionClass(decision),
	}
}

func inferredReviewDecision(position sqlitejournal.TradeReviewPosition) string {
	if position.HumanDecision.Valid {
		return position.HumanDecision.String
	}
	if position.AgentDecision.Valid {
		return position.AgentDecision.String
	}
	if position.ModelBucket.Valid {
		switch position.ModelBucket.String {
		case "proposed":
			return "trade"
		case "rejected":
			return "skip"
		case "user_requested", "alternate":
			return "watchlist"
		}
	}
	return ""
}

func countReportTrades(positions []tradeReviewReportPosition) int {
	count := 0
	for _, position := range positions {
		if strings.EqualFold(position.IsTrade, "yes") {
			count++
		}
	}
	return count
}

func decisionClass(decision string) string {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "trade":
		return "decision-trade"
	case "skip", "veto", "no_trade":
		return "decision-skip"
	case "resize", "delay", "watchlist":
		return "decision-watch"
	default:
		return "decision-empty"
	}
}

func displayNullString(value sql.NullString) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return "NA"
	}
	return value.String
}

func displayMultiline(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func firstDisplayString(values ...sql.NullString) string {
	for _, value := range values {
		if value.Valid && strings.TrimSpace(value.String) != "" {
			return value.String
		}
	}
	return "NA"
}

func firstValidFloat(values ...sql.NullFloat64) sql.NullFloat64 {
	for _, value := range values {
		if value.Valid {
			return value
		}
	}
	return sql.NullFloat64{}
}

func displayPct(value sql.NullFloat64) string {
	if !value.Valid {
		return "NA"
	}
	return fmt.Sprintf("%.2f%%", value.Float64*100)
}

func displayScore(value sql.NullFloat64) string {
	if !value.Valid {
		return "NA"
	}
	return fmt.Sprintf("%.1f", value.Float64*100)
}

func displayNumber(value sql.NullFloat64) string {
	if !value.Valid {
		return "NA"
	}
	return fmt.Sprintf("%.0f", value.Float64)
}

func displayText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "NA"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, string(filepath.Separator), "_")
	value = strings.ReplaceAll(value, "..", "_")
	return value
}
