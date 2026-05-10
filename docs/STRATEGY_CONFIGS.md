# Strategy Configs

MarketPal strategy configs are authored as YAML. The repo also ships a JSON
Schema at `schemas/strategy.schema.json`; YAML editors can use that schema for
validation, descriptions, and autocomplete.

## Defaults Profiles

Built-in strategy YAML should stay readable. It uses one top-level `defaults`
field instead of repeating advanced knobs in every file:

- `swing_v1`: used by the three swing strategies. It enables the standard
  14-day event guardrail, long-only planning, protected unscored holdings, and
  standard backtest assumptions.
- `basic_v1`: used by the simple/momentum baseline strategies. It keeps event
  guardrails disabled and applies the same long-only, protection, and backtest
  assumptions.

The defaults are expanded by the strategy loader before validation, strategy
runs, portfolio validation, and backtests.

## Visible Knobs

These are the normal fields users should read and tune:

- `scoring.momentum_weight`
- `scoring.profile_weight`
- `scoring.min_buy_score`
- `scoring.min_hold_score`
- `portfolio.max_positions`
- `portfolio.max_position_pct`
- `portfolio.min_trade_value`
- `portfolio.rebalance`
- `portfolio.listing_region_tilt`
- `risk.turnover_budget_pct`
- `risk.max_single_trade_pct`
- `risk.starter_position_pct`
- `risk.max_new_positions_per_run`
- `risk.cash_buffer_pct`

## Hidden Advanced Knobs

These remain supported for full custom configs, but they are intentionally not
shown in built-in strategy YAML:

- `event_guardrail.*`
- `portfolio.long_only`
- `risk.protect_unscored_holdings`
- `backtest.*`

If a custom config needs to override those fields, omit `defaults` and provide a
full explicit YAML config.
