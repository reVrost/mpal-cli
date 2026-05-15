package mpal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/revrost/mpal-cli/pkg/mpal/sqlitejournal"
)

type TradeReviewStartInput struct {
	ID                     string                     `json:"id,omitempty"`
	AsOf                   string                     `json:"as_of"`
	StrategyID             string                     `json:"strategy_id"`
	StrategyConfigText     string                     `json:"strategy_config_text"`
	PortfolioScope         string                     `json:"portfolio_scope,omitempty"`
	UniverseTickers        []string                   `json:"universe_tickers"`
	UserRequestedTickers   []string                   `json:"user_requested_tickers,omitempty"`
	ExecutionResult        string                     `json:"execution_result"`
	AgentHarness           string                     `json:"agent_harness,omitempty"`
	AgentModel             string                     `json:"agent_model,omitempty"`
	AgentSkill             string                     `json:"agent_skill,omitempty"`
	UserPromptText         string                     `json:"user_prompt_text,omitempty"`
	ChatHistoryText        string                     `json:"chat_history_text,omitempty"`
	AgentSummary           string                     `json:"agent_summary,omitempty"`
	FinalDecision          string                     `json:"final_decision,omitempty"`
	HumanReasoningText     string                     `json:"human_reasoning_text,omitempty"`
	FinalValidationValid   *bool                      `json:"final_validation_valid,omitempty"`
	FinalValidationSummary string                     `json:"final_validation_summary,omitempty"`
	ReportPath             string                     `json:"report_path,omitempty"`
	WarningsText           string                     `json:"warnings_text,omitempty"`
	Positions              []TradeReviewPositionInput `json:"positions,omitempty"`
}

type TradeReviewPositionInput struct {
	ID                  string   `json:"id,omitempty"`
	Ticker              string   `json:"ticker"`
	ModelBucket         string   `json:"model_bucket,omitempty"`
	ModelIntent         string   `json:"model_intent,omitempty"`
	ModelScore          *float64 `json:"model_score,omitempty"`
	ModelWeight         *float64 `json:"model_weight,omitempty"`
	ModelDeltaWeight    *float64 `json:"model_delta_weight,omitempty"`
	ModelEstimatedValue *float64 `json:"model_estimated_value,omitempty"`
	ModelSharePrice     *float64 `json:"model_share_price,omitempty"`
	ModelReason         string   `json:"model_reason,omitempty"`
	SizingMethod        string   `json:"sizing_method,omitempty"`
	RawKelly            *float64 `json:"raw_kelly,omitempty"`
	FractionalKelly     *float64 `json:"fractional_kelly,omitempty"`
	KellyTargetWeight   *float64 `json:"kelly_target_weight,omitempty"`
	FinalTargetWeight   *float64 `json:"final_target_weight,omitempty"`
	BindingConstraint   string   `json:"binding_constraint,omitempty"`
	CalibrationStatus   string   `json:"calibration_status,omitempty"`
	AgentDecision       string   `json:"agent_decision,omitempty"`
	AgentWeight         *float64 `json:"agent_weight,omitempty"`
	AgentReason         string   `json:"agent_reason,omitempty"`
	HumanDecision       string   `json:"human_decision,omitempty"`
	HumanWeight         *float64 `json:"human_weight,omitempty"`
	ExecutionPrice      *float64 `json:"execution_price,omitempty"`
	ExecutionDate       string   `json:"execution_date,omitempty"`
	HumanReason         string   `json:"human_reason,omitempty"`
}

type TradeReviewFinalizeInput struct {
	FinalDecision          string                          `json:"final_decision"`
	HumanReasoningText     string                          `json:"human_reasoning_text,omitempty"`
	FinalValidationValid   *bool                           `json:"final_validation_valid,omitempty"`
	FinalValidationSummary string                          `json:"final_validation_summary,omitempty"`
	ReportPath             string                          `json:"report_path,omitempty"`
	WarningsText           string                          `json:"warnings_text,omitempty"`
	Positions              []TradeReviewHumanPositionInput `json:"positions,omitempty"`
}

type TradeReviewHumanPositionInput struct {
	ID             string   `json:"id,omitempty"`
	Ticker         string   `json:"ticker"`
	HumanDecision  string   `json:"human_decision"`
	HumanWeight    *float64 `json:"human_weight,omitempty"`
	ExecutionPrice *float64 `json:"execution_price,omitempty"`
	ExecutionDate  string   `json:"execution_date,omitempty"`
	HumanReason    string   `json:"human_reason,omitempty"`
}

func (in TradeReviewStartInput) ToCreateParams(now time.Time) (sqlitejournal.CreateTradeReviewParams, []sqlitejournal.CreateTradeReviewPositionParams, error) {
	if strings.TrimSpace(in.ID) == "" {
		in.ID = RunID("review", now.UTC())
	}
	if strings.TrimSpace(in.AsOf) == "" {
		return sqlitejournal.CreateTradeReviewParams{}, nil, fmt.Errorf("as_of is required")
	}
	if strings.TrimSpace(in.StrategyID) == "" {
		return sqlitejournal.CreateTradeReviewParams{}, nil, fmt.Errorf("strategy_id is required")
	}
	if strings.TrimSpace(in.StrategyConfigText) == "" {
		return sqlitejournal.CreateTradeReviewParams{}, nil, fmt.Errorf("strategy_config_text is required")
	}
	if len(in.UniverseTickers) == 0 {
		return sqlitejournal.CreateTradeReviewParams{}, nil, fmt.Errorf("universe_tickers is required")
	}
	if strings.TrimSpace(in.ExecutionResult) == "" {
		return sqlitejournal.CreateTradeReviewParams{}, nil, fmt.Errorf("execution_result is required")
	}
	universeTickers, err := json.Marshal(NormalizeTickers(in.UniverseTickers))
	if err != nil {
		return sqlitejournal.CreateTradeReviewParams{}, nil, err
	}
	requestedTickers := sql.NullString{}
	if len(in.UserRequestedTickers) > 0 {
		raw, err := json.Marshal(NormalizeTickers(in.UserRequestedTickers))
		if err != nil {
			return sqlitejournal.CreateTradeReviewParams{}, nil, err
		}
		requestedTickers = nullSQLString(string(raw))
	}
	review := sqlitejournal.CreateTradeReviewParams{
		ID:                       in.ID,
		AsOf:                     in.AsOf,
		StrategyID:               in.StrategyID,
		StrategyConfigText:       in.StrategyConfigText,
		PortfolioScope:           nullSQLString(in.PortfolioScope),
		UniverseTickersJson:      string(universeTickers),
		UserRequestedTickersJson: requestedTickers,
		ExecutionResult:          in.ExecutionResult,
		AgentHarness:             nullSQLString(in.AgentHarness),
		AgentModel:               nullSQLString(in.AgentModel),
		AgentSkill:               nullSQLString(in.AgentSkill),
		UserPromptText:           nullSQLString(in.UserPromptText),
		ChatHistoryText:          nullSQLString(in.ChatHistoryText),
		AgentSummary:             nullSQLString(in.AgentSummary),
		FinalDecision:            nullSQLString(in.FinalDecision),
		HumanReasoningText:       nullSQLString(in.HumanReasoningText),
		FinalValidationValid:     nullSQLBool(in.FinalValidationValid),
		FinalValidationSummary:   nullSQLString(in.FinalValidationSummary),
		ReportPath:               nullSQLString(in.ReportPath),
		WarningsText:             nullSQLString(in.WarningsText),
	}
	positions := make([]sqlitejournal.CreateTradeReviewPositionParams, 0, len(in.Positions))
	for i, position := range in.Positions {
		params, err := position.toCreateParams(in.ID, now, i)
		if err != nil {
			return sqlitejournal.CreateTradeReviewParams{}, nil, err
		}
		positions = append(positions, params)
	}
	return review, positions, nil
}

func (in TradeReviewPositionInput) toCreateParams(reviewID string, now time.Time, index int) (sqlitejournal.CreateTradeReviewPositionParams, error) {
	if strings.TrimSpace(in.Ticker) == "" {
		return sqlitejournal.CreateTradeReviewPositionParams{}, fmt.Errorf("positions[%d].ticker is required", index)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = RunID("pos", now.UTC()) + fmt.Sprintf("_%03d", index+1)
	}
	return sqlitejournal.CreateTradeReviewPositionParams{
		ID:                  id,
		TradeReviewID:       reviewID,
		Ticker:              normalizeJournalTicker(in.Ticker),
		ModelBucket:         nullSQLString(in.ModelBucket),
		ModelIntent:         nullSQLString(in.ModelIntent),
		ModelScore:          nullSQLFloat(in.ModelScore),
		ModelWeight:         nullSQLFloat(in.ModelWeight),
		ModelDeltaWeight:    nullSQLFloat(in.ModelDeltaWeight),
		ModelEstimatedValue: nullSQLFloat(in.ModelEstimatedValue),
		ModelSharePrice:     nullSQLFloat(in.ModelSharePrice),
		ModelReason:         nullSQLString(in.ModelReason),
		SizingMethod:        nullSQLString(in.SizingMethod),
		RawKelly:            nullSQLFloat(in.RawKelly),
		FractionalKelly:     nullSQLFloat(in.FractionalKelly),
		KellyTargetWeight:   nullSQLFloat(in.KellyTargetWeight),
		FinalTargetWeight:   nullSQLFloat(in.FinalTargetWeight),
		BindingConstraint:   nullSQLString(in.BindingConstraint),
		CalibrationStatus:   nullSQLString(in.CalibrationStatus),
		AgentDecision:       nullSQLString(in.AgentDecision),
		AgentWeight:         nullSQLFloat(in.AgentWeight),
		AgentReason:         nullSQLString(in.AgentReason),
		HumanDecision:       nullSQLString(in.HumanDecision),
		HumanWeight:         nullSQLFloat(in.HumanWeight),
		ExecutionPrice:      nullSQLFloat(in.ExecutionPrice),
		ExecutionDate:       nullSQLString(in.ExecutionDate),
		HumanReason:         nullSQLString(in.HumanReason),
	}, nil
}

func (in TradeReviewFinalizeInput) ToFinalizeParams(reviewID string, now time.Time) (sqlitejournal.FinalizeTradeReviewParams, []sqlitejournal.UpsertHumanTradeReviewPositionParams, error) {
	if strings.TrimSpace(reviewID) == "" {
		return sqlitejournal.FinalizeTradeReviewParams{}, nil, fmt.Errorf("review id is required")
	}
	params := sqlitejournal.FinalizeTradeReviewParams{
		ID:                     reviewID,
		FinalDecision:          nullSQLString(in.FinalDecision),
		HumanReasoningText:     nullSQLString(in.HumanReasoningText),
		FinalValidationValid:   nullSQLBool(in.FinalValidationValid),
		FinalValidationSummary: nullSQLString(in.FinalValidationSummary),
		ReportPath:             nullSQLString(in.ReportPath),
		WarningsText:           nullSQLString(in.WarningsText),
	}
	positions := make([]sqlitejournal.UpsertHumanTradeReviewPositionParams, 0, len(in.Positions))
	for i, position := range in.Positions {
		upsert, err := position.toUpsertParams(reviewID, now, i)
		if err != nil {
			return sqlitejournal.FinalizeTradeReviewParams{}, nil, err
		}
		positions = append(positions, upsert)
	}
	return params, positions, nil
}

func (in TradeReviewHumanPositionInput) toUpsertParams(reviewID string, now time.Time, index int) (sqlitejournal.UpsertHumanTradeReviewPositionParams, error) {
	if strings.TrimSpace(in.Ticker) == "" {
		return sqlitejournal.UpsertHumanTradeReviewPositionParams{}, fmt.Errorf("positions[%d].ticker is required", index)
	}
	if strings.TrimSpace(in.HumanDecision) == "" {
		return sqlitejournal.UpsertHumanTradeReviewPositionParams{}, fmt.Errorf("positions[%d].human_decision is required", index)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = RunID("pos", now.UTC()) + fmt.Sprintf("_human_%03d", index+1)
	}
	return sqlitejournal.UpsertHumanTradeReviewPositionParams{
		ID:             id,
		TradeReviewID:  reviewID,
		Ticker:         normalizeJournalTicker(in.Ticker),
		HumanDecision:  nullSQLString(in.HumanDecision),
		HumanWeight:    nullSQLFloat(in.HumanWeight),
		ExecutionPrice: nullSQLFloat(in.ExecutionPrice),
		ExecutionDate:  nullSQLString(in.ExecutionDate),
		HumanReason:    nullSQLString(in.HumanReason),
	}, nil
}

func nullSQLString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func nullSQLFloat(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func nullSQLBool(value *bool) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	if *value {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func normalizeJournalTicker(ticker string) string {
	return strings.ToUpper(strings.TrimSpace(ticker))
}
