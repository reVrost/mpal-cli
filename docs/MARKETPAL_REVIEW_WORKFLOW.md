# MarketPal Strategy Review Workflow

This guide explains how to use `mpal-cli` to produce structured strategy review
artifacts for research, validation, and journaling. It is written for
agent-assisted workflows where MarketPal produces deterministic JSON, an agent
summarizes the evidence and constraints, and a human remains responsible for
any investment decision.

This guide is not financial, investment, tax, or legal advice. It does not
recommend that any person buy, sell, hold, trim, reduce, or exit any security.
`mpal-cli` is not a broker, does not place live orders, and should not be
treated as an order-routing system.

## Review Boundary

`mpal-cli` is useful because it keeps the machine boundary explicit:

- Strategy configs produce repeatable model output for a specific date,
  universe, portfolio input, and config hash.
- Portfolio constraints decide whether a model signal is executable inside the
  supplied inputs.
- Event and freshness fields add context that a reviewer can inspect.
- Journal entries preserve what the tool returned and what a human or agent
  concluded from that run.

The output is a review packet. It is not a trading instruction.

For field-level audit interpretation, see
[DATA_PROVENANCE_AND_AUDIT.md](DATA_PROVENANCE_AND_AUDIT.md). For model-risk
and compliance boundaries, see
[MODEL_RISK_AND_COMPLIANCE.md](MODEL_RISK_AND_COMPLIANCE.md).

## Example Review Cadences

MarketPal supports several review cadences. These are workflow labels for
organizing research, not advice about how often any person should trade.

- **Weekly review:** evaluate current signals, assigned cash, constraints, and
  existing active-engine holdings.
- **Monthly review:** evaluate whether a broader engine cleanup or rotation
  scenario is worth reviewing.
- **Quarterly review:** inspect sleeve-level drift, contribution routing, and
  portfolio-policy alignment.
- **Event-driven review:** refresh the evidence packet after a material event,
  data warning, or risk flag.

This structure avoids treating every review as a full rebalance. Academic
momentum literature often studies intermediate horizons, while very short
horizons can include reversal effects, liquidity pressure, bid-ask noise, and
profit-taking. The practical use of `mpal-cli` is to make the evidence and
constraints visible, not to force a calendar-based action.

## Strategy Configs

`mpal-cli` includes several swing-oriented strategy configs. They are related,
but they are not interchangeable.

| Strategy | Typical Review Context | Possible Output Shape | Out Of Scope |
| --- | --- | --- | --- |
| `best_weekly_swing_v1` | Default weekly swing review under the current hosted API | Starter candidates, top-up candidates, risk-driven trims/reductions, or `NO_TRADE` using weekly Markov/Kelly sizing evidence | Monthly allocation review or high-turnover rebuild |
| `best_monthly_swing_v1` | Default monthly swing review using a true monthly rebalance horizon | Lower-turnover starter/top-up candidates with stronger profile/QVM support and monthly Markov/Kelly sizing evidence | Weekly timing trades, daily review, or cleanup/rebuild |
| `engine_weekly_swing_v1` | Active-engine review, especially when the input includes cash assigned to that engine | Starter candidates, top-up candidates, risk-driven trims/reductions, or `NO_TRADE` | Forced cleanup or high-turnover rotation |
| `engine_quality_swing_rebuild_v1` | Engine cleanup, transition review, or rebuild scenario | Weak-hold review, oversized-position review, ranked candidates, higher-turnover proposal | Routine weekly monitoring |
| `portfolio_low_churn_swing_v1` | Conservative full-portfolio or well-funded portfolio review | Lower-churn incremental changes with larger minimum trade value | Aggressive rotation or engine rebuild |
| `engine_quality_value_reversion_v1` | Engine-sleeve cash deployment when quality and valuation should dominate momentum | Conservative starter buys or modest top-ups in quality/value names after measured pullbacks | Momentum breakouts, high-churn cleanup, or forced exits |
| `portfolio_quality_value_reversion_v1` | Full-portfolio or well-funded portfolio review for quality/value pullbacks | Low-churn starter buys or top-ups with larger minimum trade value | Engine-only policy reviews or aggressive rotation |

The quality-value reversion configs use the local
`scoring_v2_quality_value_reversion` contract. They are research/local-engine
configs until `strategy list` or `strategy validate` reports
`api_compatible: true` for the hosted MarketPal API.

The weekly engine strategy is buy-biased, not mathematically buy-only. It is
tuned to avoid forced cleanup sells, but a valid run can still produce trims or
reductions if the supplied inputs breach risk constraints. A hard buy-only
workflow should be implemented as a separate explicit config or code path.

## Optional Listing-Region Tilt

Strategy configs may set a one-line soft geography preference:

```yaml
portfolio:
  listing_region_tilt: US
```

Supported values are `US` and `ASX`. When omitted, the planner remains purely
score-led. When set, the tilt applies only to **new starter buys**: if the
preferred listing region is below the built-in exposure threshold and a
preferred-region candidate is close enough to a higher-scoring candidate, the
planner may choose the preferred-region candidate first. It does not force a
trade, does not affect sells/trims/exits, and does not change validation.
The built-in v1 tilt uses a 0.10 score nudge while preferred-region exposure is
below 60%, which makes it strong enough to surface close US candidates without
turning geography into the primary signal.

Ticker classification is intentionally simple in v1: `.AX` tickers are treated
as `ASX`; all other tickers are treated as `US`.

## Action Labels

Strategy output may include action labels such as:

- `BUY`: add a new position or top up an existing one inside the model output.
- `TRIM`: reduce an oversized holding inside the model output.
- `REDUCE`: cut a weak or lower-ranked holding inside the model output.
- `EXIT_CANDIDATE`: mark a position that no longer clears the configured hold
  rules.

These labels describe model output. They are not instructions to place trades.
A downstream reviewer should evaluate validation status, portfolio policy,
event context, freshness warnings, tax considerations, costs, and personal
suitability outside this repository.

## Run A Weekly Review Packet

Use `best_weekly_swing_v1` for the normal weekly swing review. Use
`engine_weekly_swing_v1` only when the portfolio input is intentionally
scoped to the active engine and any cash in the input is meant to be evaluated
inside that sleeve.

Example:

```sh
mpal strategy run \
  --date YYYY-MM-DD \
  --portfolio tmp/mpal-runs/engine-portfolio.json \
  --universe tmp/mpal-runs/engine-universe.json \
  --config strategies/best_weekly_swing_v1.yaml \
  --json > tmp/mpal-runs/weekly-engine-run.json

mpal ticker events \
  --run tmp/mpal-runs/weekly-engine-run.json \
  --portfolio tmp/mpal-runs/engine-portfolio.json \
  --days 14 \
  --json > tmp/mpal-runs/weekly-engine-events.json

mpal decision gate \
  --run tmp/mpal-runs/weekly-engine-run.json \
  --events tmp/mpal-runs/weekly-engine-events.json \
  --config strategies/best_weekly_swing_v1.yaml \
  --alternates 5 \
  --json > tmp/mpal-runs/weekly-engine-gate.json
```

Review the output as separate layers:

- `model_result`: whether the score model found qualifying signals.
- `execution_result`: whether the plan clears cash, turnover, minimum trade,
  and risk constraints.
- `baseline_plan.proposed_trades`: the model's executable baseline, if any.
- `signals`: research ranking, not an order list.
- `ticker events`: source-backed context for proposed actions and alternates.

If `model_result` is `TRADE` but `execution_result` is `NO_TRADE`, the review
packet is saying that the model found signals but the supplied portfolio inputs
and constraints blocked execution.

## Run A Monthly Swing Review

Use `best_monthly_swing_v1` for the slower monthly allocation pass. It uses
`portfolio.rebalance: monthly`, so hosted Markov/raw Kelly evidence is read on
the monthly horizon instead of treating a weekly edge as a monthly signal.

Example:

```sh
mpal strategy run \
  --date YYYY-MM-DD \
  --portfolio tmp/mpal-runs/portfolio.json \
  --universe tmp/mpal-runs/universe.json \
  --config strategies/best_monthly_swing_v1.yaml \
  --json > tmp/mpal-runs/monthly-swing-run.json

mpal ticker events \
  --run tmp/mpal-runs/monthly-swing-run.json \
  --portfolio tmp/mpal-runs/portfolio.json \
  --days 45 \
  --json > tmp/mpal-runs/monthly-swing-events.json

mpal decision gate \
  --run tmp/mpal-runs/monthly-swing-run.json \
  --events tmp/mpal-runs/monthly-swing-events.json \
  --config strategies/best_monthly_swing_v1.yaml \
  --alternates 5 \
  --json > tmp/mpal-runs/monthly-swing-gate.json
```

## Run A Cleanup Or Rotation Scenario

Use `engine_quality_swing_rebuild_v1` for an intentionally broader engine
cleanup, transition, or rebuild scenario. This config can produce sells, trims,
and rotations because that is part of the scenario being modeled.

Example:

```sh
mpal strategy run \
  --date YYYY-MM-DD \
  --portfolio tmp/mpal-runs/engine-portfolio.json \
  --universe tmp/mpal-runs/engine-universe.json \
  --config strategies/engine_quality_swing_rebuild_v1.yaml \
  --json > tmp/mpal-runs/monthly-engine-rebuild.json

mpal ticker events \
  --run tmp/mpal-runs/monthly-engine-rebuild.json \
  --portfolio tmp/mpal-runs/engine-portfolio.json \
  --days 14 \
  --json > tmp/mpal-runs/monthly-engine-events.json
```

Questions for reviewing a broader scenario:

- Did a holding fail the configured hold threshold?
- Is an action caused by position size, weak score, stale data, or event risk?
- Does validation pass under turnover, cash, and trade-size constraints?
- Does the event pack support, contradict, or fail to explain the model output?
- Is the scenario consistent with the user's private portfolio policy?

## Review Sleeve-Level Inputs

Quarterly or sleeve-level reviews are about portfolio construction inputs, not
single-security action labels. If a user has a private
`~/.marketpal/portfolio-policy.md`, use it only as local private context and do
not copy it into repo-tracked files.

Sleeve-level checks can inspect:

- whether the supplied core, engine, and high-conviction sleeves match the
  user's private policy inputs;
- whether active-engine cash is represented in the correct input file;
- whether contribution routing is documented privately;
- whether fixed holdings are excluded from automated cleanup unless the private
  policy explicitly allows review;
- whether any data freshness warning makes the run unsuitable for decision
  support.

## Event-Driven Review Packets

An event-driven review packet can be useful when the information set has
changed. Examples include:

- earnings, guidance, or analyst updates;
- accounting, liquidity, governance, or fraud-risk headlines;
- a material change to the original thesis;
- a position-size or risk-budget breach inside the supplied inputs;
- a large gap move that changes the setup being modeled;
- a data staleness warning that makes model output unreliable.

Event-driven review does not imply immediate action. It means the prior review
packet may no longer reflect the current evidence set.

## Interpreting P&L And Risk Signals

Do not turn red/green P&L color into an implicit product rule. A review packet
should separate:

- current score and rank;
- hold threshold status;
- event context;
- position size and concentration;
- volatility or drawdown fields, when present;
- validation status;
- any private policy constraints supplied by the user.

The disposition-effect literature documents that investors often hold losers
too long and sell winners too early. The product implication is not "sell" or
"hold"; it is that the review packet should expose the evidence separately from
the user's entry price and emotional framing.

Risk rules belong in explicit configs or private user policy, not in hidden
agent behavior. If a workflow needs stops, maximum loss per idea, volatility
sizing, or averaging-down rules, encode those rules explicitly and validate
them before relying on the output.

## How To Read A MarketPal Run

Every strategy review should separate signal quality from execution quality.

### Signal Read

Inspect:

- `final_score`
- `momentum_score`
- `profile_score`
- `markov`, when present: current trend state, favorable/unfavorable transition
  probabilities, confidence, and warnings
- `action_hint`
- event score and event confidence, when present

The current swing configs use momentum as the primary entry signal and
profile-QVM as a quality/holdability check.
Markov metadata is an explanatory transition read by default; it does not
authorize trades and does not replace validation. A strategy may explicitly
enable `risk.sizing_method: fractional_kelly` to use Markov probabilities as a
conservative sizing input, but proposed trades remain clamped by the fixed risk
controls. When structured sizing is present, read the Kelly target, final target,
horizon, binding constraint, probability inputs, warnings, and calibration
status from `proposed_trades[].sizing` rather than inferring them from prose.
Use `mpal decision gate` after `strategy run` and `ticker events` to package
proposed trades, rejections, event context, alternate signal context, validation
state, and sizing evidence for an agent review gate.

### Execution Read

Inspect:

- top-level `result`
- `model_result`
- `execution_result`
- `baseline_plan.proposed_trades`
- `baseline_plan.rejected`
- `validation.valid`

A high-ranked signal is not executable unless it survives the supplied
portfolio constraints.

### Event Read

`mpal ticker events` adds source-backed context before a run is summarized or
journaled. The event pack can surface:

- catalysts;
- adverse updates;
- missing source context;
- stale data;
- alternate candidates.

This is especially important for momentum names, where the model may detect
strength without fully explaining why the move happened.

### Markov Read

When hosted strategy output does not include `signals[].markov`, use the
server profile evidence:

```sh
mpal ticker profile \
  --tickers AAPL,MSFT,NVDA \
  --date YYYY-MM-DD \
  --json
```

The profile response includes `markov` and `raw_kelly` buckets for daily,
weekly, and monthly horizons when the server has enough price history. These
fields are not part of the executable baseline plan and should not be used to
bypass `execution_result` or validation.

## Journal And Report The Review

`mpal strategy run` auto-journals the deterministic first-pass strategy packet
to SQLite and returns `journal_entry_id`. Do not journal ticker event calls,
Markov calls, DD fetches, or smoke tests as separate review rows.

The review journal flow is:

1. `mpal strategy run`: records the strategy config, universe, execution result,
   model packet, proposed/rejected tickers, and Kelly sizing fields.
2. `mpal report <trade_review_id>`: renders the deterministic HTML first pass
   and updates `report_path` on the review.
3. `mpal journal finalize`: after the user decides, records the final human
   decision, final validation, and per-ticker human call.

Example:

```sh
mpal report review_... \
  --notes "Optional agent or human notes." \
  --json

mpal journal finalize \
  --id review_... \
  --input tmp/mpal-runs/trade-review-finalize.json \
  --json
```

Good started review entries include:

- strategy id and full reviewed config text;
- date;
- portfolio scope;
- universe tickers;
- user-requested tickers;
- execution result;
- agent harness/model/skill;
- user prompt and chat history;
- per-ticker model bucket, sizing read, agent decision, and agent reason.

Good finalized review entries include:

- final human decision;
- human reasoning;
- final validation result;
- per-ticker human decision, final weight, execution price/date when known, and
  human reason.

Use a deterministic raw-model fill assumption for outcomes: paper-fill raw
model trades at the next market open after the review timestamp using the
strategy's fee/slippage assumptions. This keeps model-only versus human-overlay
attribution auditable instead of anecdotal.

The journal should capture concise decision rationale, not private
chain-of-thought. Do not use journal fields as a dump for every intermediate
prompt, tool call, source packet, private policy file, or hidden reasoning
trace.

## Common Review Errors

Avoid treating the tool output as stronger than it is. Common review errors
include:

- running a rebuild config when the intended packet was routine monitoring;
- treating `signals` as an order list;
- treating `execution_result: NO_TRADE` as something to work around;
- mixing cash from different sleeves in the same input;
- making red/green P&L color the main decision variable;
- editing strategy parameters after seeing a run's output;
- ignoring data freshness warnings;
- saving private portfolio policy in the repository.

## Review Packet Checklist

Before treating a review packet as complete:

1. Is the packet weekly monitoring, cleanup/rebuild, sleeve review, or
   event-driven review?
2. Is the chosen strategy config aligned with that packet type?
3. Are portfolio and universe inputs scoped correctly?
4. Is `execution_result` clearly understood?
5. Are proposed actions present in `baseline_plan.proposed_trades`, if any?
6. Did `ticker events` show an adverse catalyst or missing context?
7. Does the plan validate under turnover, max trade size, and cash-buffer rules?
8. Has the review artifact been journaled?

If any answer is unclear, the review packet should be treated as incomplete.

## Sources And Further Reading

- Narasimhan Jegadeesh and Sheridan Titman, "Returns to Buying Winners and
  Selling Losers: Implications for Stock Market Efficiency" (1993). Classic
  cross-sectional equity momentum study using 3- to 12-month formation and
  holding horizons.
  https://moneytothemasses.com/wp-content/uploads/2014/08/Jegadeesh_Titman_1993.pdf

- Tobias Moskowitz, Yao Hua Ooi, and Lasse Heje Pedersen, "Time Series
  Momentum" (2012). Documents trend persistence over 1- to 12-month horizons
  across liquid futures markets.
  https://papers.ssrn.com/sol3/papers.cfm?abstract_id=2089463

- Bruce Lehmann, "Fads, Martingales, and Market Efficiency" (1990). Important
  evidence on short-horizon return reversal.
  https://finance.martinsewell.com/stylized-facts/dependence/Lehmann1990.pdf

- Narasimhan Jegadeesh, "Evidence of Predictable Behavior of Security Returns"
  (1990). Early evidence on predictable return behavior and short-horizon
  reversal.
  https://finance.martinsewell.com/stylized-facts/dependence/Jegadeesh1990.pdf

- Louis K. C. Chan, Narasimhan Jegadeesh, and Josef Lakonishok, "Momentum
  Strategies" (1996). Links price momentum and earnings momentum to gradual
  market reaction to information.
  https://www.nber.org/papers/w5375

- Pedro Barroso and Pedro Santa-Clara, "Momentum Has Its Moments" (2015).
  Shows that momentum risk varies over time and that risk management can
  improve crash behavior.
  https://papers.ssrn.com/sol3/papers.cfm?abstract_id=2041429

- Kent Daniel and Tobias Moskowitz, "Momentum Crashes" (2014/2016). Explains
  crash states in momentum strategies and dynamic risk-management implications.
  https://www.nber.org/system/files/working_papers/w20439/w20439.pdf

- Kathryn Kaminski and Andrew Lo, "When Do Stop-Loss Rules Stop Losses?"
  (2014). Provides a framework for when stop-loss rules add or subtract value.
  https://papers.ssrn.com/sol3/papers.cfm?abstract_id=968338

- Yufeng Han, Guofu Zhou, and Yingzi Zhu, "Taming Momentum Crashes: A Simple
  Stop-Loss Strategy" (2016). Studies stop-loss rules for momentum crash risk.
  https://papers.ssrn.com/sol3/papers.cfm?abstract_id=2407199

- Terrance Odean, "Are Investors Reluctant to Realize Their Losses?" (1998).
  Brokerage-account evidence on the disposition effect.
  https://papers.ssrn.com/sol3/papers.cfm?abstract_id=94142

- Eugene Fama and Kenneth French, "Size, Value, and Momentum in International
  Stock Returns" (2012). Finds momentum across major international regions
  except Japan.
  https://ideas.repec.org/a/eee/jfinec/v105y2012i3p457-472.html
