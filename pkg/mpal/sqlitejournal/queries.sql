-- name: CreateTradeReview :exec
INSERT INTO trade_reviews (
  id,
  as_of,
  strategy_id,
  strategy_config_text,
  portfolio_scope,
  universe_tickers_json,
  user_requested_tickers_json,
  execution_result,
  agent_harness,
  agent_model,
  agent_skill,
  user_prompt_text,
  chat_history_text,
  agent_summary,
  final_decision,
  human_reasoning_text,
  final_validation_valid,
  final_validation_summary,
  report_path,
  warnings_text
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
);

-- name: GetTradeReview :one
SELECT *
FROM trade_reviews
WHERE id = ?;

-- name: ListTradeReviews :many
SELECT *
FROM trade_reviews
ORDER BY as_of DESC, created_at DESC
LIMIT ?;

-- name: CreateTradeReviewPosition :exec
INSERT INTO trade_review_positions (
  id,
  trade_review_id,
  ticker,
  model_bucket,
  model_intent,
  model_score,
  model_weight,
  model_delta_weight,
  model_estimated_value,
  model_share_price,
  model_reason,
  sizing_method,
  raw_kelly,
  fractional_kelly,
  kelly_target_weight,
  final_target_weight,
  binding_constraint,
  calibration_status,
  agent_decision,
  agent_weight,
  agent_reason,
  human_decision,
  human_weight,
  execution_price,
  execution_date,
  human_reason
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
);

-- name: SetTradeReviewReportPath :exec
UPDATE trade_reviews
SET report_path = ?
WHERE id = ?;

-- name: FinalizeTradeReview :exec
UPDATE trade_reviews
SET
  final_decision = ?,
  human_reasoning_text = ?,
  final_validation_valid = ?,
  final_validation_summary = ?,
  report_path = ?,
  warnings_text = ?
WHERE id = ?;

-- name: UpsertHumanTradeReviewPosition :exec
INSERT INTO trade_review_positions (
  id,
  trade_review_id,
  ticker,
  human_decision,
  human_weight,
  execution_price,
  execution_date,
  human_reason
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
)
ON CONFLICT(trade_review_id, ticker) DO UPDATE SET
  human_decision = excluded.human_decision,
  human_weight = excluded.human_weight,
  execution_price = excluded.execution_price,
  execution_date = excluded.execution_date,
  human_reason = excluded.human_reason;

-- name: ListTradeReviewPositions :many
SELECT *
FROM trade_review_positions
WHERE trade_review_id = ?
ORDER BY ticker ASC;

-- name: ListTradeReviewPositionsByTicker :many
SELECT p.*
FROM trade_review_positions p
JOIN trade_reviews r ON r.id = p.trade_review_id
WHERE p.ticker = ?
ORDER BY r.as_of DESC, r.created_at DESC;

-- name: DeleteTradeReview :exec
DELETE FROM trade_reviews
WHERE id = ?;
