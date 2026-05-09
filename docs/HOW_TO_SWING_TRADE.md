# How To Swing Trade With `mpal-cli`

This guide explains how to use `mpal-cli` as a disciplined swing-trading and
portfolio-construction tool. It is written for agent-assisted workflows where
MarketPal generates deterministic strategy output, an agent explains the plan,
and a human decides what to do.

`mpal-cli` is not a broker. It does not place live orders. Treat every run as a
planning, validation, and journaling step.

## The Core Idea

The practical MarketPal swing-trading flow is:

- **Weekly:** deploy cash that is explicitly assigned to the active return
  engine.
- **Monthly:** allow deliberate engine cleanup or rotation, including sells.
- **Quarterly:** review sleeve-level allocation and rebalance only if drift is
  material.
- **Event-driven:** act sooner if a thesis-breaking event, stale data warning,
  or risk breach appears.

This is intentionally different from "rebalance everything every week." Most
academic momentum evidence is built around intermediate horizons, not constant
short-horizon churn. Weekly checks are useful for monitoring, cash deployment,
and risk control; monthly reviews are a better default point for allowing
sells, trims, and rotations.

## Why Weekly Monitoring But Monthly Rotation?

The most famous stock momentum evidence comes from Jegadeesh and Titman, who
studied strategies that rank stocks on past 3- to 12-month returns and hold for
3 to 12 months. That research supports intermediate-horizon momentum, not a
daily or weekly forced rebalance.

Time-series momentum research by Moskowitz, Ooi, and Pedersen also finds return
persistence over 1- to 12-month horizons, with partial reversal over longer
horizons. This supports the idea that a monthly cadence can capture much of the
trend signal while avoiding excessive churn.

The warning is short-term reversal. Very short intervals such as one week or
one month can contain reversal effects, liquidity pressure, bid-ask noise, and
profit-taking. A stock can be a valid intermediate-term winner while still
being noisy over a week.

So the MarketPal operating model is:

- Use **weekly reviews** to stay informed and put new cash to work.
- Use **monthly reviews** to make rotation decisions.
- Use **event-driven reviews** when the market gives you new information that
  cannot wait for the next calendar review.

## Strategy Configs And When To Use Them

`mpal-cli` includes three swing-oriented strategy configs. They are related,
but they are not interchangeable.

| Strategy | Use When | What It Is Allowed To Do | What It Is Not For |
| --- | --- | --- | --- |
| `engine_weekly_swing_v1` | Weekly active-engine review, especially when there is cash assigned to the engine | Add starter positions, top up strong existing holdings, return `NO_TRADE` if there is no executable action | Forced weekly cleanup or high-turnover selling |
| `engine_quality_swing_rebuild_v1` | Monthly or quarterly engine cleanup, transition rebalance, or deliberate rebuild | Sell weak holds, trim oversized positions, add ranked candidates, use higher turnover | Default weekly review |
| `portfolio_low_churn_swing_v1` | Conservative full-portfolio or well-funded portfolio review | Low-churn incremental changes with larger minimum trade value | Engine rebuild or aggressive rotation |

The weekly engine strategy is **buy-biased**, not mathematically buy-only. It is
tuned to avoid forced cleanup sells, but a valid run can still produce trims or
reductions if risk rules are breached. If a user wants a hard buy-only mode,
that should be a separate explicit config or code path.

## Does "Rebalance" Mean Selling?

Yes, but not always.

In this workflow, "rebalance" means the plan is allowed to change position
weights. That can include:

- `BUY`: add a new position or top up an existing one.
- `TRIM`: reduce an oversized holding.
- `REDUCE`: cut a weak or lower-ranked holding.
- `EXIT_CANDIDATE`: sell out of a position that no longer clears the hold
  rules.

The important distinction is:

- **Weekly deployment:** mostly buys/top-ups with assigned cash.
- **Monthly engine rebalance:** buys plus possible sells.
- **Quarterly sleeve rebalance:** portfolio-level adjustment across core,
  engine, and high-conviction sleeves.

Monthly does not mean "must sell." It means "sells are allowed if the validated
plan and event context justify them."

## The Recommended Operating Cadence

### Weekly: Engine Cash Deployment

Use `engine_weekly_swing_v1`.

Run it when:

- New cash has been assigned to the active engine.
- You want to know the current ranked swing candidates.
- You want to check whether existing engine holdings still look acceptable.
- You want a `NO_TRADE` decision when there is no valid action.

This is the default weekly flow:

1. Update the engine portfolio input.
2. Include only cash explicitly assigned to the engine.
3. Run the strategy.
4. Fetch ticker events for proposed trades and alternates.
5. Distinguish `model_result` from `execution_result`.
6. Journal the final decision.

Example:

```sh
mpal strategy run \
  --date YYYY-MM-DD \
  --portfolio tmp/mpal-runs/engine-portfolio.json \
  --universe tmp/mpal-runs/engine-universe.json \
  --config strategies/engine_weekly_swing_v1.yaml \
  --json > tmp/mpal-runs/weekly-engine-run.json

mpal ticker events \
  --run tmp/mpal-runs/weekly-engine-run.json \
  --portfolio tmp/mpal-runs/engine-portfolio.json \
  --days 14 \
  --json > tmp/mpal-runs/weekly-engine-events.json
```

Read the output this way:

- `model_result`: whether the score model found attractive signals.
- `execution_result`: whether the plan is actually tradable under cash,
  turnover, minimum trade, and risk constraints.
- `baseline_plan.proposed_trades`: the executable baseline, if any.
- `signals`: useful research ranking, not automatically a trade list.
- `ticker events`: source-backed evidence layer for proposed trades and
  alternates.

If `model_result` is `TRADE` but `execution_result` is `NO_TRADE`, do not force
a trade. It usually means the model likes candidates but the portfolio has no
cash, the minimum trade value is too high, or risk limits block the action.

### Monthly: Engine Cleanup Or Rotation

Use `engine_quality_swing_rebuild_v1`.

Run it when:

- The engine sleeve has stale holdings.
- A group of positions has fallen below hold quality.
- Position weights have drifted materially.
- You deliberately want MarketPal to consider sells and rotations.

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

Monthly cleanup is where sells belong. Still, do not blindly accept them. Check:

- Did the holding fail the strategy hold threshold?
- Is the sell caused by position size, weak score, stale data, or event risk?
- Is there a better validated use of capital?
- Does the event pack support or contradict the model decision?
- Is the proposed turnover appropriate for the sleeve?

### Quarterly: Sleeve Review

Quarterly reviews are about portfolio construction rather than single-name
signals. If the user has a private `~/.marketpal/portfolio-policy.md`, read it
before making any recommendation.

Quarterly checks should answer:

- Is the core sleeve still near target?
- Is the active engine underweight or overweight?
- Is the high-conviction sleeve too large?
- Should new contributions go to core, engine, or cash?
- Do any fixed holdings need an explicit risk review?

Do not store private sleeve targets, holdings, or personal policy details in
repo-tracked files. Keep them in `~/.marketpal/portfolio-policy.md`.

### Event-Driven: Do Not Wait For The Calendar

Run an event-driven review immediately when there is:

- Adverse earnings or guidance.
- Accounting, liquidity, governance, or fraud risk.
- A major downgrade to the original thesis.
- A position-size breach.
- A large gap down that invalidates the setup.
- A data staleness warning that makes the model output unreliable.

Event-driven does not mean panic-selling. It means re-running the process
because the information set changed.

## Sell Rules: Do Not Anchor On Red Or Green

A common mistake is turning P&L color into a rule:

- "Never sell red unless it is down 25%."
- "Always take profit when green."
- "Hold until break-even."

Those rules are psychologically comfortable, but they are not a robust trading
system. The disposition-effect literature documents that investors often hold
losers too long and sell winners too early. A swing strategy should avoid
anchoring on entry price.

Use this decision frame instead:

| Situation | Better Rule |
| --- | --- |
| Position is red but score, event context, and thesis are intact | Hold or review; do not sell just because it is red |
| Position is red and the signal broke | Reduce or exit if the plan validates |
| Position is green but still ranks well | Let it run unless position size or event risk says otherwise |
| Position is green but thesis broke | Trim or exit; profit does not repair a broken setup |
| Position hit a drawdown threshold | Review risk budget, volatility, and thesis; do not wait mechanically for a deeper loss |

For normal swing trades, a fixed 25% stop is often too wide. For
high-conviction or venture-like positions, it may be too tight. A better risk
rule is to define the amount of sleeve capital you are willing to lose per idea,
then size the position around volatility and invalidation level.

Practical guardrails:

- Set maximum risk per idea before entry.
- Use smaller starter positions for volatile names.
- Prefer structure- or volatility-aware stops over arbitrary round numbers.
- Do not average down unless the strategy explicitly supports it.
- Do not sell winners only to feel safe; sell when risk, score, or thesis says
  the expected return has changed.

## How To Read A MarketPal Run

Every strategy review should separate signal quality from execution quality.

### Signal Read

Look at:

- `final_score`
- `momentum_score`
- `profile_score`
- `action_hint`
- event score and event confidence, when present

The current swing configs use momentum as the primary entry signal and
profile-QVM as a quality/holdability check.

### Execution Read

Look at:

- top-level `result`
- `model_result`
- `execution_result`
- `baseline_plan.proposed_trades`
- `baseline_plan.rejected`
- `validation.valid`

A high-ranked signal is not executable unless it survives portfolio constraints.

### Event Read

Always run `mpal ticker events` before final action. The event pack can surface:

- source-backed catalysts
- adverse updates
- missing source context
- stale data
- alternate candidates

This is especially important for momentum names, where the model may correctly
detect strength but not fully explain why the move happened.

## Journal The Decision

Journal final actions so future reviews can see what was decided and why.

Example:

```sh
mpal journal append \
  --type agent_final_action \
  --input tmp/mpal-runs/final-action.json \
  --json
```

Good journal entries include:

- strategy id and config hash
- date
- portfolio input
- universe input
- model result and execution result
- proposed trades
- rejected candidates
- event read
- final action
- what would change the decision

## Common Mistakes

Avoid these:

- Running the monthly rebuild config every week.
- Treating `signals` as an order list.
- Trading when `execution_result` is `NO_TRADE`.
- Adding cash to the wrong sleeve.
- Selling red positions only because they are red.
- Refusing to sell broken positions because they are red.
- Editing strategy parameters after seeing today's output.
- Ignoring data freshness warnings.
- Saving private portfolio policy in the repository.

## A Simple Human Checklist

Before acting on any swing-trading plan:

1. Is this a weekly deployment, monthly cleanup, quarterly sleeve review, or
   event-driven review?
2. Is the correct strategy config being used?
3. Is cash explicitly assigned to the sleeve being traded?
4. Is `execution_result` actually `TRADE`?
5. Are proposed trades present in `baseline_plan.proposed_trades`?
6. Did `ticker events` show any adverse catalyst or missing context?
7. Does the plan validate under turnover, max trade size, and cash-buffer rules?
8. Has the final action been journaled?

If any answer is unclear, the default action is `NO_TRADE`.

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
