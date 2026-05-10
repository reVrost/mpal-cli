# Strategy Configs

MarketPal strategy configs are authored as YAML. The repo also ships a JSON
Schema at `schemas/strategy.schema.json`; YAML editors can use that schema for
validation, descriptions, and autocomplete.

## Defaults Profiles

Built-in strategy YAML should stay readable. It uses one top-level `defaults`
field instead of repeating advanced knobs in every file:

- `swing_v1`: used by the swing strategies. It enables the standard
  14-day event guardrail, long-only planning, protected unscored holdings, and
  standard backtest assumptions.
- `basic_v1`: used by the simple/momentum baseline strategies. It keeps event
  guardrails disabled and applies the same long-only, protection, and backtest
  assumptions.

The defaults are expanded by the strategy loader before validation, strategy
runs, portfolio validation, and backtests.

`config_hash` is a SHA-256 hash of the canonical expanded strategy JSON. The
raw YAML layout is not part of the hash, so a slim built-in config using
`defaults: swing_v1` hashes the same as an equivalent fully expanded config.

## Visible Knobs

These are the normal fields users should read and tune:

- `scoring.momentum_weight`
- `scoring.profile_weight`
- `scoring.quality_weight`
- `scoring.value_weight`
- `scoring.reversion_weight`
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

`quality_weight`, `value_weight`, and `reversion_weight` are optional. They
exist for quality-value mean-reversion strategies where the aggregate
`profile_weight` is too blunt. All scoring weights must sum to `1.0`.

The v1 reversion score favors a measured pullback from the one-year high. It
does not reward a stock sitting at highs, and it also fades very deep drawdowns
that may indicate a broken setup rather than a normal mean-reversion candidate.

Strategy runs may also include optional `signals[].markov` metadata. This is a
metadata-only transition read from recent price bars: it classifies the current
trend bucket, estimates next-state probabilities over the strategy's
`portfolio.rebalance` horizon, and reports confidence and sample warnings. It
does not change `final_score`, candidate ordering, proposed trades, or
validation. If the hosted strategy-run API omits this field, use
`mpal ticker markov` as a separate local evidence packet.

The hosted MarketPal API currently uses the `hosted_strategy_api_v1` execution
contract, which supports `momentum_weight` and `profile_weight`. Configs with
non-zero `quality_weight`, `value_weight`, or `reversion_weight` are valid local
configs, but `mpal strategy run` and `mpal backtest run` fail fast until the
hosted API supports `scoring_v2_quality_value_reversion`. Use
`mpal strategy validate --json` or `mpal strategy list --json` to inspect
`api_compatibility`.

## Hidden Advanced Knobs

These remain supported for full custom configs, but they are intentionally not
shown in built-in strategy YAML:

- `event_guardrail.*`
- `portfolio.long_only`
- `risk.protect_unscored_holdings`
- `backtest.*`

If a custom config needs to override those fields, omit `defaults` and provide a
full explicit YAML config.
