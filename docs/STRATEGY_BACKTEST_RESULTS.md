# Strategy Backtest Results

This page records published backtest evidence for built-in Marketpal strategy
configs. Backtests are research artifacts, not investment advice or trading
instructions. Results depend on the supplied universe, date range, price
history, fee/slippage assumptions, and strategy version.

## May 2026 Watchlist Momentum Search

This run searched weekly pure-momentum configs on a 95-ticker high-conviction
watchlist universe.

- Primary window: 2025-12-03 to 2026-05-12
- Initial capital: 100,000
- Rebalance cadence: weekly
- Price mode: close
- Backtest costs: 5 bps fee and 10 bps slippage
- Strategy family: pure momentum, with quality/value/reversion weights set to
  zero

The full-window CAGR figures are annualized from a short period. They should
not be read as a sustainable annual return estimate.

| Strategy | Role | Full Return | CAGR | Sharpe | Max DD | Trades | Final Equity | Late Validation | Recent 1-Week |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `aggressive_momentum_weekly_v1` | Top raw full-window result | 116.67% | 352.88% | 3.81 | -11.25% | 149 | 216,671.04 | 46.97% | -0.27% |
| `active_momentum_weekly_v1` | Selected robust config | 112.31% | 335.25% | 3.80 | -12.02% | 157 | 212,310.96 | 49.24% | -0.27% |
| `momentum_only_v1` | Built-in benchmark | 44.74% | 105.91% | 2.82 | -10.75% | 168 | 144,735.95 | 22.27% | -0.04% |

Late validation covers the later slice of the same research period. The recent
one-week check is intentionally narrow; the benchmark was slightly better there
because it stayed mostly in cash while the active configs deployed more capital.

## Configs

`aggressive_momentum_weekly_v1` maps to the top raw-search config:

```text
mom2_mb0p55_mh0p1_mp10_mw0p18_mn4_st0p02_tr0p05_to0p45
```

Key settings:

- `max_positions: 10`
- `max_position_pct: 0.18`
- `min_buy_score: 0.55`
- `min_hold_score: 0.10`
- `max_new_positions_per_run: 4`
- `starter_position_pct: 0.02`
- `max_single_trade_pct: 0.05`
- `turnover_budget_pct: 0.45`
- `cash_buffer_pct: 0.02`

`active_momentum_weekly_v1` maps to the selected robust config:

```text
mom2_mb0p55_mh0p1_mp14_mw0p18_mn4_st0p02_tr0p04_to0p45
```

Key settings:

- `max_positions: 14`
- `max_position_pct: 0.18`
- `min_buy_score: 0.55`
- `min_hold_score: 0.10`
- `max_new_positions_per_run: 4`
- `starter_position_pct: 0.02`
- `max_single_trade_pct: 0.04`
- `turnover_budget_pct: 0.45`
- `cash_buffer_pct: 0.02`

`aggressive_momentum_weekly_v1` produced the highest full-window return. The
slightly more diversified `active_momentum_weekly_v1` was kept because it had a
stronger late-validation result and a lower single-trade cap.

## Final Holdings

Final holdings at 2026-05-12, sorted by ending portfolio weight:

| Strategy | Final Holdings |
| --- | --- |
| `aggressive_momentum_weekly_v1` | DOCN 18.4%, INTC 17.7%, LITE 16.2%, BE 16.1%, AMD 16.0%, APLD 6.8%, CRWV 6.4%, HZN.AX 0.4%, DRO.AX 0.0%, ISRG 0.0%, GOOGL 0.0%, cash about 2.0% |
| `active_momentum_weekly_v1` | DOCN 18.8%, BE 15.0%, INTC 14.7%, LITE 14.0%, MU 13.8%, AMD 8.3%, AMZN 5.3%, GNP.AX 3.0%, KAR.AX 2.2%, ARU.AX 1.5%, HZN.AX 1.0%, PLS.AX 0.2%, GOOGL 0.0%, ISRG 0.0%, cash about 2.0% |
| `momentum_only_v1` | DOCN 12.2%, INTC 11.7%, LITE 11.4%, BE 11.3%, KAR.AX 10.4%, MU 8.9%, AMD 7.2%, SXE.AX 5.7%, GOOGL 4.5%, LYC.AX 4.4%, SNDK 3.3%, NHC.AX 3.1%, AMZN 2.0%, WDS.AX 1.2%, MIN.AX 0.6%, cash about 2.0% |

Small near-zero residual positions can appear from price movement, minimum
trade thresholds, and cash-buffer constraints.

## Built-In Strategy Coverage

The table below is intentionally conservative. A strategy is marked "documented"
only when this repo records a comparable result set for that strategy.

| Strategy | Documented Result Status |
| --- | --- |
| `aggressive_momentum_weekly_v1` | Documented in the May 2026 watchlist momentum search |
| `active_momentum_weekly_v1` | Documented in the May 2026 watchlist momentum search |
| `momentum_only_v1` | Documented as the benchmark in the May 2026 watchlist momentum search |
| `momentum_profile_v1` | No comparable published result in this page |
| `simple_score_v1` | No comparable published result in this page |
| `best_weekly_swing_v1` | No comparable published result in this page |
| `best_monthly_swing_v1` | No comparable published result in this page |
| `engine_weekly_swing_v1` | No comparable published result in this page |
| `engine_quality_swing_rebuild_v1` | No comparable published result in this page |
| `portfolio_low_churn_swing_v1` | No comparable published result in this page |
| `engine_quality_value_reversion_v1` | No comparable published result in this page |
| `portfolio_quality_value_reversion_v1` | No comparable published result in this page |

Before presenting a strategy as having historical evidence, run a fresh
backtest on the target universe and record the same assumptions, date range,
metrics, and final holdings.
