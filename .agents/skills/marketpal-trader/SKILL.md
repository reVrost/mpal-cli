---
name: marketpal-trader
description: Use when an external agent is asked to analyze a MarketPal trading plan with the mpal capability CLI, apply portfolio construction policy, explain TRADE or NO_TRADE results, produce investor-readable baseline briefs, handle agent veto or override workflows, or journal a trading-plan review without executing live trades.
---

# MarketPal Trader Agent

Use `mpal` as the deterministic source of truth. The agent explains and may explicitly veto or propose a bounded override, but it must not invent trades or execute live orders.

## Required Workflow

1. Load private portfolio policy when available.
   - Before real current-portfolio reviews, check `~/.marketpal/portfolio-policy.md`.
   - If present, read it and apply its sleeve rules, fixed holdings, contribution policy, review cadence, and high-conviction guardrails.
   - Treat the policy as private user context; do not copy personal dollar amounts into repo-tracked files or public outputs unless directly relevant to the requested review.
   - If the policy marks holdings as fixed/core/high-conviction, do not let the normal engine plan trade them unless the user explicitly asks for a full-portfolio review or a review of that sleeve.
   - If no policy file exists, continue with the normal MarketPal workflow and state that no private portfolio policy was found.

2. Choose an approved strategy config.
   - Prefer `mpal strategy list --json` and `mpal strategy show --id <id> --json`.
   - For API-backed runs, require `api_compatible: true`; if a locally valid config is not API-compatible, treat it as research-only until the hosted API contract is updated.
   - Never silently edit a strategy config.
   - If a config change is needed, create a new version outside the trading run.

3. Run the baseline plan.
   - Call `mpal strategy run --date <date> --universe <path> --portfolio <path> --config <path> --json`.
   - If MarketPal is installed as an MCP server, call `mpal_strategy_run` with the same date, universe, portfolio, and explicit config inputs.
   - Treat the returned JSON as the source of truth for model result, execution result, validation, signals, target weights, proposed trades, warnings, freshness metadata, config hash, and journal entry id.
   - Read `model_result` as the raw signal read and `execution_result`/top-level `result` as the executable rebalance decision.
   - For real current-portfolio reviews, the approved strategy configs use a transition rebalance policy by default: turnover budget, max single-trade size, starter position size, new-position limit, cash buffer, and protected unscored holdings.
   - Use the `signals` payload from `strategy run` for pure research ranking. Do not describe the old top-N/rank-and-replace behavior as a rebalance plan.
   - If `validation.valid` is false, the final decision is `NO_TRADE` unless a separately validated override is journaled.

4. Build the source-backed context pack before final action.
   - Call `mpal ticker events --run <strategy-run-path-or-json> --portfolio <portfolio-path> --days 14 --json`.
   - If using MCP, call `mpal_ticker_events` with the previous strategy run and the portfolio input.
   - If a portfolio file is not available, set `MPAL_API_KEY` so the command can fetch the current portfolio through the MarketPal API.
   - Use the returned proposed buys/sells/trims, alternate candidates, ticker research, source-backed updates, cached article insights, missing insight sources, warnings, and freshness metadata as the evidence layer.
   - Extract up to five alternate buy candidates from `ticker events` alternates first, then from deterministic `signals` if the context pack has fewer than five. Exclude tickers already in proposed buy trades unless the user specifically asks for substitutes.
   - Always show those alternate buy candidates to the user with ticker, rank, score, action hint, event context, and why they were not in the executable baseline. Label them as non-executable candidates until a replacement or override plan validates.
   - The context pack can support an approve, veto, resize, or replacement recommendation, but it does not itself authorize a trade.
   - Do not replace a proposed trade with a news-driven alternative unless the replacement appears in the context pack or the deterministic `mpal` signal output, and the final plan validates.
   - For user-requested tickers, first check whether they already appear in the strategy run signals, context pack tickers, context pack alternates, or proposed trades. Only call `mpal ticker events --tickers ...` for requested tickers missing from the context pack. Batch missing requested tickers into one call.
   - Keep context calls compact by default: use the strategy defaults unless the user asks for deeper research; when adding extra requested tickers, prefer `--days 14 --limit 80` and avoid raising `--insights-per-ticker` unless the decision depends on deeper article detail.

5. Build a dual-horizon Markov/Kelly decision gate before final action.
   - Always produce a Kelly decision gate for every proposed trade and up to five alternate buy candidates, even when the baseline strategy uses fixed starter or max-trade caps.
   - Always compute both weekly and daily Markov reads for the same ticker set. Weekly Markov is the primary Kelly sizing and trade-decision horizon for weekly swing strategies; daily Markov is an execution-timing overlay.
   - First inspect `proposed_trades[].sizing` and `signals[].markov` from `strategy run`. If `proposed_trades[].sizing.method == "fractional_kelly"`, treat that sizing object as the executable weekly Kelly sizing source of truth, but still compute daily Markov for timing.
   - If the strategy output omits `signals[].markov`, call `mpal ticker markov --tickers <proposed-and-alternate-tickers> --date <run-date> --rebalance weekly --json` or MCP `mpal_ticker_markov` for weekly Markov.
   - Separately call `mpal ticker markov --tickers <proposed-and-alternate-tickers> --date <run-date> --rebalance daily --json` or MCP `mpal_ticker_markov` for daily Markov timing.
   - If the installed `mpal` binary does not expose `ticker markov` but the local repo source does, use `go run ./cmd/mpal ticker markov ...` from the repo root and clearly label the result as a local Markov/Kelly decision gate. If neither path is available, state that Kelly could not be calculated and why.
   - Use the MarketPal fractional Kelly formula unless `proposed_trades[].sizing` already provides executable values: `raw_kelly = (payoff_ratio * favorable_probability - unfavorable_probability) / (payoff_ratio * (favorable_probability + unfavorable_probability))`; `confidence_shrunk = max(0, raw_kelly * confidence)`; `fractional_kelly = min(confidence_shrunk * kelly_fraction, kelly_max_fraction)`.
   - Use strategy-specified Kelly settings when present; otherwise use MarketPal defaults: `payoff_ratio=1.0`, `kelly_fraction=0.25`, `kelly_max_fraction=0.05`, `kelly_min_confidence=0.25`, `kelly_min_sample_count=30`, `kelly_missing_edge_policy=fixed`.
   - For each audited ticker, report weekly favorable probability, weekly unfavorable probability, weekly confidence, weekly sample count, weekly raw Kelly, weekly fractional Kelly target, daily raw Kelly, daily gate action, final Kelly gate action, final executable target weight, and any clamp or fallback reason.
   - Kelly must affect the trade decision:
     - Weekly `raw_kelly <= 0`, missing weekly Markov data, or weekly confidence/sample thresholds below the configured minimum makes a proposed buy `KELLY_REJECT`. Do not approve that buy unless a separate written override rationale is journaled and the override validates.
     - Positive weekly Kelly below the strategy starter size makes the proposed buy `KELLY_DOWNSIZE`. Prefer resizing to the weekly Kelly target, subject to minimum trade value and validation; if the resized trade fails minimum value, veto that buy rather than silently using fixed starter size.
     - Positive weekly Kelly at or above the starter size but below the proposed top-up/add size makes the trade `KELLY_CAP`. Cap the proposed trade to the weekly Kelly target and validate the resulting plan.
     - Positive weekly Kelly above the proposed size makes the trade `KELLY_PASS`. The trade may keep baseline sizing, still subject to event and portfolio risk gates.
     - Daily `raw_kelly <= 0` does not replace weekly Kelly as the sizing horizon, but it triggers `DAILY_TIMING_REJECT` or `DAILY_TIMING_DOWNSIZE`: prefer delaying the trade or halving the starter/top-up if the reduced plan clears minimum trade value and validates.
     - Daily positive Kelly cannot promote a trade when weekly Kelly rejects it. If daily is positive but weekly is negative or unavailable, keep the trade rejected or validate a separate written override.
     - For alternates, positive Kelly can make an alternate replacement-eligible even when the baseline candidate has a slightly higher deterministic score, provided the alternate appears in `ticker events.alternates` or deterministic `signals`, has no event veto, and the replacement validates.
   - If the baseline strategy uses fixed caps instead of `risk.sizing_method: fractional_kelly`, Kelly still controls the agent decision gate. The fixed-cap baseline is only the starting proposal; the final approved plan must reflect Kelly vetoes, caps, or validated replacements.
   - Do not let an agent-computed Kelly gate authorize an unvalidated trade by itself. `mpal strategy run` supplies the candidate set, Kelly changes the agent approve/veto/resize/replace decision, and `mpal portfolio validate` remains required before the final plan is executable.

### Runtime Efficiency

- Do not rerun `mpal strategy run` in the same review unless the portfolio, universe, date, or config changes.
- After the baseline run completes, run independent evidence calls in parallel where the environment allows it: `ticker events --run`, weekly `ticker markov`, daily `ticker markov`, and any missing requested-ticker `ticker events --tickers ...` can run concurrently.
- Prefer one batched command per evidence type. Use comma-separated tickers for `ticker markov`, `ticker profile`, `ticker events`, `ticker financials`, `ticker insiders`, and `ticker ownership`; do not loop over one ticker at a time from the skill.
- Reuse local run artifacts under `tmp/mpal-runs/` during the same review. If a JSON file already exists for the same date, strategy, universe, portfolio, and ticker set, read it instead of refetching unless the user asks for a fresh run.
- Keep chat-visible output compact. Save wide JSON and HTML reports to files, then summarize the decision; avoid pasting large payloads back into chat.
- If a user asks for broad extra research, fetch the deterministic strategy/context first, then only deepen source-backed research for tickers that are plausible candidates after the model, Kelly gate, and event veto checks.

6. Run the risk/reward override gate.
   - Before approving a valid baseline, compare each proposed buy with the top alternate buy candidates from `ticker events`.
   - Do not treat thin or missing source-backed updates as a standalone reason to reject or replace a proposed starter. For a momentum-led swing strategy, unexplained price/volume strength can still be a valid starter signal when deterministic score, profile-QVM, sizing, and validation are strong.
   - Treat a proposed starter as replacement-eligible when it has stale or missing critical data, mixed/adverse event context, materially weaker profile-QVM support, an event veto, a validation/risk issue, a severe concentration/sleeve concern, or a `KELLY_REJECT` / `KELLY_DOWNSIZE` gate result that makes the baseline trade unattractive or impractical.
   - Treat an alternate as a better risk/reward candidate only when it appears in `ticker events.alternates` or deterministic `signals`, has no event veto, passes the Kelly gate, and either has equal-or-better deterministic score/profile support, stronger Kelly-adjusted sizing support, or is within 0.03 `final_score` of a replacement-eligible proposed starter. Do not use news context alone to replace a higher-scoring validated baseline starter.
   - Prefer one bounded replacement at a time: swap the weakest replacement-eligible proposed starter for the strongest qualifying alternate, use the lower of the baseline sizing and Kelly target unless executable Kelly sizing is already present, preserve turnover, cash buffer, and max-new-position count, then validate the concrete override plan with `mpal portfolio validate`.
   - If the override validates, present it as `agent_override` for user review and journal it. If it fails validation, keep the baseline or veto based on the validation errors and event risk.
   - If no qualifying replacement exists, explicitly say why the baseline remains the best validated plan.

7. Explain the baseline before giving the decision.
   - Start with a compact baseline brief: strategy, date, portfolio source, universe, model result, execution result, proposed trades, selected signal scores, rejected tickers, and the scoring rule/threshold.
   - Include an "alternate buy candidates" section with up to five candidates. If the risk/reward gate already found and validated a superior bounded override, state the override directly instead of asking a follow-up question.
   - Include a "Kelly decision gate" section for proposed trades and alternates. Show both daily and weekly Markov raw Kelly. State whether weekly Kelly passed, rejected, downsized, capped, or promoted each candidate; state whether daily Markov caused an execution-timing delay or downsize; and state whether the resulting final plan validated.
   - Include warnings, freshness/staleness notes, strategy id/version/config hash, and journal entry id.
   - Do not add trades that are absent from `mpal` output.

8. For final action, vetoes, or overrides.
   - Label the action as an agent veto or override.
   - Give an investor-readable rationale, not just a terse error string.
   - Validate any override or final executable plan with `mpal portfolio validate --plan <path-or-json> --portfolio <path> --universe <path> --config <path> --json`.
   - If using MCP, validate with `mpal_portfolio_validate`.
   - Journal the final action with `mpal journal append --type agent_veto|agent_override|agent_final_action --baseline-journal-id <id> --input <path-or-json> --json`.
   - If using MCP, journal with `mpal_journal_append`.

## Hybrid Trader / Quality-Swing Overlay

Use this overlay when the chosen strategy is a MarketPal swing config, such as `portfolio_low_churn_swing_v1`, `engine_weekly_swing_v1`, `engine_quality_swing_rebuild_v1`, `engine_quality_value_reversion_v1`, or `portfolio_quality_value_reversion_v1`.

- Treat momentum as the primary entry signal and profile-QVM as the holdability and survivability check.
- Treat weekly Markov transition metadata as the primary input to the Kelly decision gate. A strong favorable weekly transition probability can support a validated candidate or replacement, while a weak, negative, missing, or low-confidence weekly read can force a veto, downsize, cap, or replacement attempt.
- Treat daily Markov transition metadata as an execution-timing overlay. A negative daily read can delay or reduce a weekly-approved trade; a positive daily read can support entry timing; daily Markov cannot override a weekly Kelly rejection by itself.
- For routine daily or weekly full-portfolio reviews, prefer `portfolio_low_churn_swing_v1` when available. It allows up to four new positions per run while keeping turnover, starter size, minimum trade value, and hold thresholds conservative.
- For weekly policy engine-sleeve reviews, prefer `engine_weekly_swing_v1` when available. It is sized for the MarketPal return-engine sleeve and should be the default strategy for engine swing-trade proposals.
- Treat `engine_quality_swing_rebuild_v1` as a manual engine-sleeve rebuild config only. Do not use it for daily reviews unless the user explicitly asks for a higher-churn transition or cleanup plan.
- Use `engine_quality_value_reversion_v1` or `portfolio_quality_value_reversion_v1` only when the user explicitly asks for quality/value mean reversion, underpriced quality, or pullback buying. These configs are deliberately less momentum-led and should not replace the default weekly swing run.
- Remember that `max_new_positions_per_run` caps new buy positions only; sells, trims, reductions, and exits can make total trade tickets higher. If a daily low-churn baseline produces more than four total proposed trades, explicitly flag churn risk and prefer an agent veto or a smaller validated override unless the user asked for a transition rebalance.
- After every baseline run, use `ticker events --days 14` to classify proposed buys and alternates before the final decision:
  - `CORE_SWING`: strong signal, profile-QVM support, no event veto, and supportive or neutral source context.
  - `TACTICAL_ONLY`: strong momentum with weaker profile-QVM, thin source context, or mixed event context; keep starter sizing and avoid top-up overrides, but do not treat thin source context alone as a veto.
  - `VETO_REVIEW`: stale data, event veto, missing critical source context, or severe adverse update; prefer veto, resize, or replacement only if the replacement validates.
- For a weekly cash-deployment review, do not replace a higher-scoring validated `TACTICAL_ONLY` starter with a lower-scoring `CORE_SWING` alternate solely because the alternate has better source-backed news. Present the alternate as optional unless the proposed starter is replacement-eligible under the risk/reward override gate.
- If a trade moves against the plan but the ticker remains above `min_hold_score`, profile-QVM remains supportive, and no event veto appears, prefer `HOLD` over an immediate forced exit.
- If a user has a gut-favored ticker, only propose a bounded override when that ticker appears in deterministic `signals` or `ticker events` alternates, and only after `mpal portfolio validate` passes.
- In the final rationale, explicitly separate model rank, profile-QVM holdability, event context, sizing/turnover constraints, and whether the action is baseline approval, agent veto, or agent override.

## Portfolio Construction Overlay

Use this overlay when `~/.marketpal/portfolio-policy.md` exists or the user asks for portfolio construction, sleeve allocation, stock picking, contribution allocation, or engine-only review.

- Treat the private policy file as the user's standing portfolio mandate.
- Keep core ETF/cash holdings fixed when the policy says they are core, unless the user explicitly asks to rebalance core.
- Keep high-conviction holdings fixed when the policy says they are outside the MarketPal engine, unless the user asks for that high-conviction sleeve review.
- For engine-only reviews, construct or use a portfolio input that represents only the MarketPal return-engine sleeve plus any cash allocated to that sleeve; do not blindly let core or high-conviction positions drive engine trades.
- When a policy sleeve is materially off target, surface the drift in the risk read before approving trades.
- If `mpal` baseline trades conflict with the private policy, prefer an agent veto or a policy-aware override that validates.
- Journal whether the run was full-portfolio, engine-only, core review, high-conviction review, or what-if simulation.

## Response Shape

Use this order for user-facing summaries:

1. Baseline brief
2. Signal read
3. Risk and sizing read
4. Alternate buy candidates
5. Validation result
6. Agent decision
7. What would change the decision

The baseline brief should feel like an investment committee fact sheet. The rationale should feel like the portfolio manager's decision note.

For alternate buy candidates:

- Show exactly five when available; otherwise state how many were available.
- Prefer `ticker events.alternates` ordering, then top buy-like `signals` by `final_score`.
- Include ticker, score, rank, `action_hint`, daily and weekly Markov transition reads when available, event read, source gaps, and the baseline rejection reason such as insufficient funding, min trade value, or buy threshold.
- Ask one direct preference question after the list, e.g. "Which, if any, should I validate as an override candidate?"
- Do not imply these are approved trades unless `mpal portfolio validate` passes on a concrete override plan.

### Trade Table / HTML Report Format

When the user asks for a table, asks "what would you trade today?", asks to compare proposed trades and alternates, asks about extra tickers, or the output would have more than eight table columns, prefer a local HTML report plus a compact chat summary. Use a Markdown table only for quick, narrow responses.

Default columns:

| Ticker | isTrade | Score | Role | Daily Raw Kelly | Weekly Raw Kelly | Frac Kelly | Accepted Sizing % | Accepted Sizing Price | Read |
| --- | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | --- |
| `AAPL` | true | 0.912 | Starter | 4.00% | 12.00% | 3.00% pass | 1.50% | USD 1,500 | One-sentence investment read. |

Report rules:

- Include all baseline proposed trades, up to five alternates, and any user-requested tickers in the same table.
- Use `Role` values such as `Starter`, `Top-up`, `Alternate`, `Trim`, `Exit`, or `user-request`. If the user explicitly asks for a custom role label such as `user-request`, use that exact label.
- `isTrade` is `true` only for the final validated executable plan or validated override being recommended. If a valid baseline is narrowed by an agent override, baseline trades excluded from the override must show `isTrade=false`.
- `Accepted Sizing %` and `Accepted Sizing Price` must reflect the final validated plan only. Non-selected candidates show `0.00%` and `0`, even when they had positive Kelly but were not selected or did not validate.
- Show `Daily Raw Kelly` and `Weekly Raw Kelly` separately from `Frac Kelly`. Raw Kelly may be negative. Label fractional Kelly with the weekly gate result and daily timing overlay where relevant, such as `pass`, `reject`, `downsize`, `cap`, `replacement`, `daily-delay`, `daily-downsize`, `unavailable`, or `low-confidence`.
- The `Read` column should be a concise portfolio-manager note: model rank/score, profile-QVM support, event context, sizing/concentration issue, source-data issue, or why the ticker is not selected.
- For requested tickers not in the baseline context pack, fetch `ticker events --tickers ...` and `ticker markov --tickers ...`, then join those reads with the deterministic `signals` from the latest strategy run when available.
- If a requested ticker has source collision, stale profile, missing insight, or ticker-mapping concerns, state that in `Read` and do not validate it as a trade until the data issue is resolved.
- If the user asks whether to add one or more candidates, validate concrete bounded override combinations with `mpal portfolio validate` before saying they are executable. Prefer one extra starter at a time unless the user explicitly wants a more aggressive deployment.

HTML report rules:

- Write reports under `tmp/mpal-runs/` or the run directory already used for the review. Name them predictably, e.g. `trade-review-YYYY-MM-DD.html`.
- Generate a complete standalone HTML file with inline CSS. Use semantic table markup, sticky header, compact numeric columns, a wider `Read` column, and restrained status coloring for `isTrade`, validation, and warnings.
- Before styling an HTML report, inspect `../marketpal/docs/design-system/DESIGN.md` and `../marketpal/docs/design-system/variables.css` when available. Follow the MarketPal design-system tokens and visual language rather than inventing a new report theme.
- Style reports as a dark-first MarketPal financial workspace with a sleek Cursor/Vercel-like premium finish: dense, quiet, high-contrast, polished, and product-grade. Use dark canvas `#18191f`, dark panels `#25262d`, raised controls `#383a42`, thin borders `#383a42`, primary text `#fcfcfd`, secondary text `#bfc0c4`, Amethyst premium accent `#f1ccff` with black text, teal for positive/buy, red for negative/risk, and amber for warnings.
- Use Geist-style system typography: `Geist, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`; use `Geist Mono`, `ui-monospace`, `SFMono-Regular`, `Menlo`, monospace for tickers, scores, Kelly, sizing, prices, dates, and config hashes. Use tabular numbers throughout the table.
- Keep the layout compact and operational: 14px body text, 13px secondary text, 11px captions, 24px max page title, 6-8px radii, tight paddings, no hero sections, no decorative blobs, no heavy gradients, no oversized cards, and no nested bordered cards.
- Use a premium but restrained visual system: sticky top metadata bar, compact KPI chips for result/validation/turnover/cash, subtle hover rows, selected trade rows with a muted amethyst/teal left accent, and warning rows with amber accents. Dark mode is the default; optional light mode should use the warm MarketPal canvas/panel colors from the design-system docs.
- Include a concise header above the table: strategy id/version, run date, portfolio scope, model result, execution result, validation status, config hash, journal id when available, and final decision.
- Include a short notes section below the table for assumptions, freshness/staleness warnings, source-data issues, and "what would change the decision".
- Keep the HTML static and local. Do not load remote JavaScript, fonts, CSS, trackers, or broker/order-placement links.
- In the final chat response, link to the generated local HTML file and summarize the final trade decision in one or two sentences. Do not paste the whole wide table into chat when the HTML report exists.
- If the user explicitly asks for "HTML", "dashboard", "report", or "easier to read", create the HTML artifact rather than only describing that HTML would be better.

For the risk and sizing read, explicitly name:

- turnover used versus the strategy turnover budget
- max single-trade cap
- starter position size
- max new positions per run
- protected or unscored holdings
- trade intents: `STARTER`, `TOP_UP`, `TRIM`, `REDUCE`, or `EXIT_CANDIDATE`
- Kelly decision gate status: executable Kelly, weekly Kelly pass, weekly Kelly reject, weekly Kelly downsize, weekly Kelly cap, Kelly replacement candidate, daily timing pass, daily timing delay, daily timing downsize, unavailable Markov, low-confidence/low-sample rejection, or fixed fallback
- per-trade Kelly fields when available: weekly favorable probability, weekly unfavorable probability, weekly confidence, weekly sample count, weekly raw Kelly, weekly fractional Kelly target, daily raw Kelly, daily confidence/sample count, Kelly gate action, daily timing action, final target weight, and clamp/fallback warnings

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
    "alternate_buy_candidates": [],
    "selected_signals": [],
    "rejected": [],
    "strategy_logic": "70% momentum / 30% profile-QVM, buy threshold 0.60"
  },
  "kelly_sizing_audit": [
    {
      "ticker": "AAPL",
      "status": "kelly_pass_fixed_cap_baseline",
      "weekly_favorable_probability": 0.42,
      "weekly_unfavorable_probability": 0.33,
      "weekly_confidence": 0.55,
      "weekly_sample_count": 45,
      "weekly_raw_kelly": 0.12,
      "weekly_fractional_kelly_target": 0.0165,
      "daily_raw_kelly": 0.04,
      "daily_confidence": 0.62,
      "daily_sample_count": 188,
      "final_target_weight": 0.015,
      "kelly_gate_action": "KELLY_PASS",
      "daily_timing_action": "DAILY_TIMING_PASS",
      "warnings": ["Kelly gate passed; final target still requires portfolio validation"]
    }
  ],
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
