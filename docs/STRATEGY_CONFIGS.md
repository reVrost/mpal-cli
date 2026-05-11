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
- `risk.profile`
- `risk.turnover_budget_pct`
- `risk.max_single_trade_pct`
- `risk.starter_position_pct`
- `risk.max_new_positions_per_run`
- `risk.cash_buffer_pct`
- `risk.sizing_method`
- `risk.kelly_fraction`

`quality_weight`, `value_weight`, and `reversion_weight` are optional. They
exist for quality-value mean-reversion strategies where the aggregate
`profile_weight` is too blunt. All scoring weights must sum to `1.0`.

The v1 reversion score favors a measured pullback from the one-year high. It
does not reward a stock sitting at highs, and it also fades very deep drawdowns
that may indicate a broken setup rather than a normal mean-reversion candidate.

`risk.profile` is optional shorthand for the three core fixed risk controls:
`risk.turnover_budget_pct`, `risk.max_single_trade_pct`, and
`risk.starter_position_pct`. Explicit values still override the profile. These
controls are always caps or fixed-size fallbacks; they do not get loosened by
Kelly sizing. The built-in profiles are:

- `basic`: 20% turnover, 5% max trade, 2% starter
- `low_churn`: 6% turnover, 2.5% max trade, 1.2% starter
- `weekly_swing`: 12% turnover, 3.5% max trade, 1.5% starter
- `rebuild`: 30% turnover, 4% max trade, 1.5% starter

Strategy runs may also include optional `signals[].markov` metadata. By
default this is a metadata-only transition read from recent price bars: it
classifies the current trend bucket, estimates next-state probabilities over
the strategy's `portfolio.rebalance` horizon, and reports confidence and
sample warnings. It does not change `final_score`, candidate ordering, proposed
trades, or validation unless the strategy explicitly sets
`risk.sizing_method: fractional_kelly`.

Fractional Kelly sizing is experimental and conservative. The normal config can
be as small as one line:

```yaml
risk:
  sizing_method: fractional_kelly
```

That uses internal defaults: 25% Kelly fraction, 5% maximum Kelly target, 1.0
payoff ratio, 0.25 minimum confidence, 30 minimum Markov samples, and fixed
sizing fallback when Markov data is missing. Users who want a different
fraction can set one extra field:

```yaml
risk:
  sizing_method: fractional_kelly
  kelly_fraction: 0.50
```

`kelly_fraction` must be in `(0,1]`; common conservative values are `0.25` and
`0.50`. The planner estimates a long-only target from Markov
favorable/unfavorable probabilities, shrinks the raw Kelly estimate by Markov
confidence, applies `risk.kelly_fraction`, and caps the Kelly target. That
target is only an input: final trades are still clamped by
`portfolio.max_position_pct`, `risk.max_single_trade_pct`, remaining
`risk.turnover_budget_pct`, available cash after `risk.cash_buffer_pct`, and
`portfolio.min_trade_value`. Low-confidence, low-sample, or non-positive Kelly
reads do not get promoted into larger fixed sizes.

When both are configured, priority is:

1. `risk.profile` expands fixed caps and the fixed starter fallback.
2. `risk.sizing_method: fractional_kelly` computes a desired Kelly target.
3. The final proposed trade is clamped by the fixed caps from the profile or
   explicit overrides.

In JSON output, `proposed_trades[].sizing.target_weight` and
`proposed_trades[].sizing.kelly_target_weight` are the Kelly target before the
final risk clamp and after `risk.kelly_max_fraction`, while
`proposed_trades[].sizing.fractional_kelly` is the confidence-shrunk fractional
Kelly estimate before that Kelly max cap. `proposed_trades[].target_weight`
and `proposed_trades[].sizing.final_target_weight` are the actual proposed
final target. `proposed_trades[].sizing.horizon` and `horizon_bars` identify
the Markov horizon that produced the executable Kelly target.
`proposed_trades[].sizing.binding_constraint` names the active sizing limit
when available, such as `kelly_target`, `kelly_max_fraction`,
`max_single_trade_pct`, `max_position_pct`, `turnover_budget_pct`,
`cash_buffer_pct`, or `fixed_fallback`. Markov-derived Kelly decisions also
carry the favorable/unfavorable probabilities used for sizing and
`calibration_status: heuristic_markov` until a calibrated probability model is
available. If the Kelly target is reduced by caps, the `sizing.warnings` field
records that clamp.

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
- `risk.kelly_min_edge`
- `risk.kelly_max_fraction`
- `risk.kelly_default_payoff_ratio`
- `risk.kelly_min_confidence`
- `risk.kelly_min_sample_count`
- `risk.kelly_missing_edge_policy`
- `risk.protect_unscored_holdings`
- `backtest.*`

If a custom config needs to override those fields, omit `defaults` and provide a
full explicit YAML config.
