---
name: marketpal-trader
description: Use when an external agent is asked to analyze a MarketPal trading plan with the mpal capability CLI, explain TRADE or NO_TRADE results, produce investor-readable baseline briefs, handle agent veto or override workflows, or journal a trading-plan review without executing live trades.
---

# MarketPal Trader Agent

Use `mpal` as the deterministic source of truth. The agent explains and may explicitly veto or propose a bounded override, but it must not invent trades or execute live orders.

## Required Workflow

1. Choose an approved strategy config.
   - Prefer `mpal strategy list --json` and `mpal strategy show --id <id> --json`.
   - Never silently edit a strategy config.
   - If a config change is needed, create a new version outside the trading run.

2. Run the baseline plan.
   - Call `mpal strategy run --date <date> --universe <path> --portfolio <path> --config <path> --json`.
   - Treat the returned JSON as the source of truth for model result, execution result, validation, signals, target weights, proposed trades, warnings, freshness metadata, config hash, and journal entry id.
   - Read `model_result` as the raw signal read and `execution_result`/top-level `result` as the executable rebalance decision.
   - For real current-portfolio reviews, the approved strategy configs use a transition rebalance policy by default: turnover budget, max single-trade size, starter position size, new-position limit, cash buffer, and protected unscored holdings.
   - Use the `signals` payload from `strategy run` for pure research ranking. Do not describe the old top-N/rank-and-replace behavior as a rebalance plan.
   - If `validation.valid` is false, the final decision is `NO_TRADE` unless a separately validated override is journaled.

3. Build the source-backed context pack before final action.
   - Call `mpal ticker events --run <strategy-run-path-or-json> --portfolio <portfolio-path> --days 14 --json`.
   - If a portfolio file is not available, set `MPAL_API_KEY` so the command can fetch the current portfolio through the MarketPal API.
   - Use the returned proposed buys/sells/trims, alternate candidates, ticker research, source-backed updates, cached article insights, missing insight sources, warnings, and freshness metadata as the evidence layer.
   - The context pack can support an approve, veto, resize, or replacement recommendation, but it does not itself authorize a trade.
   - Do not replace a proposed trade with a news-driven alternative unless the replacement appears in the context pack or the deterministic `mpal` signal output, and the final plan validates.

4. Explain the baseline before giving the decision.
   - Start with a compact baseline brief: strategy, date, portfolio source, universe, model result, execution result, proposed trades, selected signal scores, rejected tickers, and the scoring rule/threshold.
   - Include warnings, freshness/staleness notes, strategy id/version/config hash, and journal entry id.
   - Do not add trades that are absent from `mpal` output.

5. For final action, vetoes, or overrides.
   - Label the action as an agent veto or override.
   - Give an investor-readable rationale, not just a terse error string.
   - Validate any override or final executable plan with `mpal portfolio validate --plan <path-or-json> --portfolio <path> --universe <path> --config <path> --json`.
   - Journal the final action with `mpal journal append --type agent_veto|agent_override|agent_final_action --baseline-journal-id <id> --input <path-or-json> --json`.

## Hybrid Trader / Quality-Swing Overlay

Use this overlay when the chosen strategy is a hybrid trader/quality-swing config, such as `hybrid_trader_low_churn_max4_v1` or `hybrid_trader_quality_swing_max7_v1`.

- Treat momentum as the primary entry signal and profile-QVM as the holdability and survivability check.
- For daily low-churn reviews, prefer the approved `hybrid_trader_low_churn_max4_v1` config when available. It allows up to four new positions per run while keeping turnover, starter size, minimum trade value, and hold thresholds conservative.
- Treat `hybrid_trader_quality_swing_max7_v1` as a manual transition-rebalance config only. Do not use it for daily reviews unless the user explicitly asks for a higher-churn transition plan.
- Remember that `max_new_positions_per_run` caps new buy positions only; sells, trims, reductions, and exits can make total trade tickets higher. If a daily low-churn baseline produces more than four total proposed trades, explicitly flag churn risk and prefer an agent veto or a smaller validated override unless the user asked for a transition rebalance.
- After every baseline run, use `ticker events --days 14` to classify proposed buys and alternates before the final decision:
  - `CORE_SWING`: strong signal, profile-QVM support, no event veto, and supportive or neutral source context.
  - `TACTICAL_ONLY`: strong momentum with weaker profile-QVM, thin source context, or mixed event context; keep starter sizing and avoid top-up overrides.
  - `VETO_REVIEW`: stale data, event veto, missing critical source context, or severe adverse update; prefer veto, resize, or replacement only if the replacement validates.
- If a trade moves against the plan but the ticker remains above `min_hold_score`, profile-QVM remains supportive, and no event veto appears, prefer `HOLD` over an immediate forced exit.
- If a user has a gut-favored ticker, only propose a bounded override when that ticker appears in deterministic `signals` or `ticker events` alternates, and only after `mpal portfolio validate` passes.
- In the final rationale, explicitly separate model rank, profile-QVM holdability, event context, sizing/turnover constraints, and whether the action is baseline approval, agent veto, or agent override.

## Response Shape

Use this order for user-facing summaries:

1. Baseline brief
2. Signal read
3. Risk and sizing read
4. Validation result
5. Agent decision
6. What would change the decision

The baseline brief should feel like an investment committee fact sheet. The rationale should feel like the portfolio manager's decision note.

For the risk and sizing read, explicitly name:

- turnover used versus the strategy turnover budget
- max single-trade cap
- starter position size
- max new positions per run
- protected or unscored holdings
- trade intents: `STARTER`, `TOP_UP`, `TRIM`, `REDUCE`, or `EXIT_CANDIDATE`

## Journal Payload Shape

For `agent_veto`, `agent_override`, and `agent_final_action`, journal a JSON object with these fields:

```json
{
  "decision": "agent_veto",
  "baseline_brief": {
    "strategy": "momentum_profile_v1",
    "date": "2026-05-06",
    "portfolio": "synthetic $100k cash, no positions",
    "universe": ["AAPL", "AMZN", "GOOGL", "META", "MSFT", "NVDA", "TSLA"],
    "model_result": "TRADE",
    "execution_result": "NO_TRADE",
    "proposed_trades": [],
    "selected_signals": [],
    "rejected": [],
    "strategy_logic": "70% momentum / 30% profile-QVM, buy threshold 0.60"
  },
  "signal_read": "The model found attractive names, but this is not the same as an executable trade.",
  "risk_read": "The proposed allocation violates the approved turnover guardrail.",
  "validation": { "valid": false, "errors": ["plan exceeds max turnover per run"] },
  "investor_rationale": "Clear, human-readable reason for approve/veto/override.",
  "final_action": "NO_TRADE",
  "what_would_change_the_decision": "Resize and revalidate under the approved turnover rule."
}
```

Keep the journal concise but useful for future review. Prefer numbers, thresholds, and explicit tradeoffs over generic wording.

## Veto Example

If `mpal` finds four buyable names but validation fails because proposed turnover is 80% and the strategy limit is 30%, the correct decision is an agent veto:

- Model read: selected names cleared the buy threshold.
- Execution read: the plan is not valid under the approved risk rules.
- Final action: `NO_TRADE` unless a resized plan is created, validated, and journaled as an override.

## Hard Rules

- Never execute live trades.
- Never call broker/order-placement tools.
- Never bypass `mpal portfolio validate` for overrides.
- Never use an unapproved strategy for scheduled/autonomous runs.
- Never optimize parameters or invent a one-off strategy for today.
- Always surface `mpal` warnings and stale-data metadata in the explanation.
- Always distinguish `model_result` from `execution_result` when both are present.
