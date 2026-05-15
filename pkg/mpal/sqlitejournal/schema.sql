PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS trade_reviews (
  id TEXT PRIMARY KEY,

  -- System-populated review metadata.
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  as_of TEXT NOT NULL,

  -- Populated from the reviewed mpal strategy run.
  strategy_id TEXT NOT NULL,
  strategy_config_text TEXT NOT NULL,

  portfolio_scope TEXT CHECK (
    portfolio_scope IS NULL OR portfolio_scope IN ('full', 'engine', 'watchlist', 'custom')
  ),

  universe_tickers_json TEXT NOT NULL CHECK (json_valid(universe_tickers_json)),
  user_requested_tickers_json TEXT CHECK (
    user_requested_tickers_json IS NULL OR json_valid(user_requested_tickers_json)
  ),

  execution_result TEXT NOT NULL,

  -- Populated by the agent review layer.
  agent_harness TEXT,
  agent_model TEXT,
  agent_skill TEXT,
  user_prompt_text TEXT,
  chat_history_text TEXT,
  agent_summary TEXT,

  -- Populated by the final human decision.
  final_decision TEXT CHECK (
    final_decision IS NULL OR final_decision IN (
      'trade',
      'skip',
      'watchlist',
      'veto',
      'resize',
      'delay',
      'no_trade'
    )
  ),
  human_reasoning_text TEXT,

  -- Final validation of the actual human/agent plan.
  final_validation_valid INTEGER,
  final_validation_summary TEXT,

  -- Deterministic local report metadata.
  report_path TEXT,

  warnings_text TEXT
) STRICT;

CREATE TABLE IF NOT EXISTS trade_review_positions (
  id TEXT PRIMARY KEY,

  trade_review_id TEXT NOT NULL
    REFERENCES trade_reviews(id) ON DELETE CASCADE,

  ticker TEXT NOT NULL,

  -- Populated from mpal strategy run / decision gate.
  model_bucket TEXT CHECK (
    model_bucket IS NULL OR model_bucket IN (
      'proposed',
      'alternate',
      'rejected',
      'holding',
      'user_requested'
    )
  ),
  model_intent TEXT,
  model_score REAL,
  model_weight REAL,
  model_delta_weight REAL,
  model_estimated_value REAL,
  model_share_price REAL,
  model_reason TEXT,

  -- Populated from deterministic sizing fields when exposed by mpal.
  sizing_method TEXT,
  raw_kelly REAL,
  fractional_kelly REAL,
  kelly_target_weight REAL,
  final_target_weight REAL,
  binding_constraint TEXT,
  calibration_status TEXT,

  -- Populated by the agent review layer.
  agent_decision TEXT CHECK (
    agent_decision IS NULL OR agent_decision IN (
      'trade',
      'skip',
      'watchlist',
      'veto',
      'resize',
      'delay'
    )
  ),
  agent_weight REAL,
  agent_reason TEXT,

  -- Populated by the final human decision.
  human_decision TEXT CHECK (
    human_decision IS NULL OR human_decision IN (
      'trade',
      'skip',
      'watchlist',
      'veto',
      'resize',
      'delay'
    )
  ),
  human_weight REAL,
  execution_price REAL,
  execution_date TEXT,
  human_reason TEXT,

  UNIQUE (trade_review_id, ticker)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_trade_reviews_as_of
  ON trade_reviews(as_of);

CREATE INDEX IF NOT EXISTS idx_trade_review_positions_review
  ON trade_review_positions(trade_review_id);

CREATE INDEX IF NOT EXISTS idx_trade_review_positions_ticker
  ON trade_review_positions(ticker);
