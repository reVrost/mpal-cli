package mpal

import (
	"database/sql"

	"github.com/revrost/mpal-cli/pkg/mpal/sqlitejournal"
)

func ReviewJournalOutput(review sqlitejournal.TradeReview, positions []sqlitejournal.TradeReviewPosition) map[string]any {
	outPositions := make([]map[string]any, 0, len(positions))
	for _, position := range positions {
		outPositions = append(outPositions, map[string]any{
			"id":                    position.ID,
			"trade_review_id":       position.TradeReviewID,
			"ticker":                position.Ticker,
			"model_bucket":          nullableString(position.ModelBucket),
			"model_intent":          nullableString(position.ModelIntent),
			"model_score":           nullableFloat(position.ModelScore),
			"model_weight":          nullableFloat(position.ModelWeight),
			"model_delta_weight":    nullableFloat(position.ModelDeltaWeight),
			"model_estimated_value": nullableFloat(position.ModelEstimatedValue),
			"model_share_price":     nullableFloat(position.ModelSharePrice),
			"model_reason":          nullableString(position.ModelReason),
			"sizing_method":         nullableString(position.SizingMethod),
			"raw_kelly":             nullableFloat(position.RawKelly),
			"fractional_kelly":      nullableFloat(position.FractionalKelly),
			"kelly_target_weight":   nullableFloat(position.KellyTargetWeight),
			"final_target_weight":   nullableFloat(position.FinalTargetWeight),
			"binding_constraint":    nullableString(position.BindingConstraint),
			"calibration_status":    nullableString(position.CalibrationStatus),
			"agent_decision":        nullableString(position.AgentDecision),
			"agent_weight":          nullableFloat(position.AgentWeight),
			"agent_reason":          nullableString(position.AgentReason),
			"human_decision":        nullableString(position.HumanDecision),
			"human_weight":          nullableFloat(position.HumanWeight),
			"execution_price":       nullableFloat(position.ExecutionPrice),
			"execution_date":        nullableString(position.ExecutionDate),
			"human_reason":          nullableString(position.HumanReason),
		})
	}
	return map[string]any{
		"review": map[string]any{
			"id":                          review.ID,
			"created_at":                  review.CreatedAt,
			"as_of":                       review.AsOf,
			"strategy_id":                 review.StrategyID,
			"strategy_config_text":        review.StrategyConfigText,
			"portfolio_scope":             nullableString(review.PortfolioScope),
			"universe_tickers_json":       review.UniverseTickersJson,
			"user_requested_tickers_json": nullableString(review.UserRequestedTickersJson),
			"execution_result":            review.ExecutionResult,
			"agent_harness":               nullableString(review.AgentHarness),
			"agent_model":                 nullableString(review.AgentModel),
			"agent_skill":                 nullableString(review.AgentSkill),
			"user_prompt_text":            nullableString(review.UserPromptText),
			"chat_history_text":           nullableString(review.ChatHistoryText),
			"agent_summary":               nullableString(review.AgentSummary),
			"final_decision":              nullableString(review.FinalDecision),
			"human_reasoning_text":        nullableString(review.HumanReasoningText),
			"final_validation_valid":      nullableInt(review.FinalValidationValid),
			"final_validation_summary":    nullableString(review.FinalValidationSummary),
			"report_path":                 nullableString(review.ReportPath),
			"warnings_text":               nullableString(review.WarningsText),
		},
		"positions": outPositions,
	}
}

func ReviewListOutput(reviews []sqlitejournal.TradeReview) map[string]any {
	out := make([]map[string]any, 0, len(reviews))
	for _, review := range reviews {
		out = append(out, ReviewJournalOutput(review, nil)["review"].(map[string]any))
	}
	return map[string]any{"reviews": out}
}

func nullableString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableFloat(value sql.NullFloat64) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
}

func nullableInt(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
