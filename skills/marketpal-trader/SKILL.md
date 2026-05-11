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
   - For real current-portfolio reviews, the approved strategy configs use a transition rebalance policy by default: sizing method, turnover budget, max single-trade size, starter position size or Kelly fallback, new-position limit, cash buffer, and protected unscored holdings.
   - Inspect `strategy.config.risk.sizing_method`, `proposed_trades[].sizing`, `proposed_trades[].reason`, `proposed_trades[].target_weight`, and `proposed_trades[].delta_weight`. If `mpal` returns a structured sizing decision, explain it as part of the baseline; do not fall back to old fixed-starter language.
   - Use the `signals` payload from `strategy run` for pure research ranking. Do not describe the old top-N/rank-and-replace behavior as a rebalance plan.
   - If `validation.valid` is false, the final decision is `NO_TRADE` unless a separately validated override is journaled.

4. Build the source-backed context pack before final action.
   - Call `mpal ticker events --run <strategy-run-path-or-json> --portfolio <portfolio-path> --days 14 --json`.
   - If using MCP, call `mpal_ticker_events` with the previous strategy run and the portfolio input.
   - If a portfolio file is not available, set `MPAL_API_KEY` so the command can fetch the current portfolio through the MarketPal API.
   - Use the returned proposed buys/sells/trims, alternate candidates, ticker research, source-backed updates, cached article insights, missing insight sources, warnings, and freshness metadata as the evidence layer.
   - Extract up to five alternate buy candidates from `ticker events` alternates first, then from deterministic `signals` if the context pack has fewer than five. Exclude tickers already in proposed buy trades unless the user specifically asks for substitutes.
   - Always show those alternate buy candidates to the user with ticker, rank, score, action hint, event context, and why they were not in the executable baseline. Label them as non-executable candidates until a replacement or override plan validates.
   - When `signals[].markov` is present, use it as a probabilistic transition read: current trend state, favorable/unfavorable next-state probability, confidence, sample count, and warnings. Raw Markov metadata by itself must not authorize a buy, replacement, or resize.
   - If `mpal strategy run` or `proposed_trades[].sizing` used Markov-derived fractional Kelly, explain the resulting `mpal` sizing decision. If `mpal` did not expose a structured sizing decision, do not compute Kelly manually; describe Markov as context only and say the executable Kelly decision was not exposed.
   - The context pack can support an approve, veto, resize, or replacement recommendation, but it does not itself authorize a trade.
   - Do not replace a proposed trade with a news-driven alternative unless the replacement appears in the context pack or the deterministic `mpal` signal output, and the final plan validates.
   - For user-requested tickers, first check whether they already appear in the strategy run signals, context pack tickers, context pack alternates, or proposed trades. Only call `mpal ticker events --tickers ...` for requested tickers missing from the context pack. Batch missing requested tickers into one call.
   - Keep context calls compact by default: use the strategy defaults unless the user asks for deeper research; when adding extra requested tickers, prefer `--days 14 --limit 80` and avoid raising `--insights-per-ticker` unless the decision depends on deeper article detail.

5. Read deterministic sizing and optional Markov context before final action.
   - When available, call `mpal decision gate --run <strategy-run-path-or-json> --alternates 5 --json` or MCP `mpal_decision_gate` after the baseline run. Treat its evidence hash, item statuses, sizing fields, rejected tickers, alternate context, and validation read as the decision-gate evidence packet.
   - Treat `proposed_trades[].sizing`, `baseline_plan.rejected[]`, and `mpal decision gate` output as the only executable Kelly sizing/gating sources.
   - Do not independently compute raw Kelly, fractional Kelly, gate statuses, vetoes, caps, downsizes, delays, or replacement eligibility from `mpal ticker markov` output.
   - Identify the executable sizing horizon from `proposed_trades[].sizing.horizon` when present, otherwise from `signals[].markov.horizon`, and otherwise from `strategy.config.portfolio.rebalance`. Do not call sizing weekly unless the exposed horizon is weekly.
   - For configs with `portfolio.rebalance: daily`, such as low-churn daily configs, treat executable Kelly sizing as daily-horizon sizing. Weekly Markov, if fetched, is secondary swing context only.
   - Use `mpal ticker markov` only for explanatory context when the user asks for extra timing/horizon color, when `signals[].markov` is missing and the review needs a Markov read, or when `mpal decision gate --include-markov-context ...` explicitly requires it.
   - Daily Markov is a timing-risk flag, not a hard veto, unless `mpal` exposes a deterministic timing gate or backtest evidence supports that rule. A negative daily read may justify caution, smaller validated sizing, delay/watchlist language, or a follow-up, but it must not mechanically halve or reject a strategy-approved weekly/monthly trade.
   - If `mpal` does not expose structured sizing, explain fixed sizing and risk caps from the strategy config. Markov may inform caution, but it must not replace the deterministic planner.

### Runtime Efficiency

- Do not rerun `mpal strategy run` in the same review unless the portfolio, universe, date, or config changes.
- After the baseline run completes, run independent evidence calls in parallel where the environment allows it: `ticker events --run`, optional `ticker markov` context requested by the user or needed for missing Markov reads, and any missing requested-ticker `ticker events --tickers ...` can run concurrently.
- Prefer one batched command per evidence type. Use comma-separated tickers for `ticker markov`, `ticker profile`, `ticker events`, `ticker financials`, `ticker insiders`, and `ticker ownership`; do not loop over one ticker at a time from the skill.
- Reuse local run artifacts under `tmp/mpal-runs/` during the same review. If a JSON file already exists for the same date, strategy, universe, portfolio, and ticker set, read it instead of refetching unless the user asks for a fresh run.
- Keep chat-visible output compact. Save wide JSON and HTML reports to files, then summarize the decision; avoid pasting large payloads back into chat.
- If a user asks for broad extra research, fetch the deterministic strategy/context first, then only deepen source-backed research for tickers that are plausible candidates after the model, sizing/risk read, and event veto checks.

6. Run the risk/reward override gate.
   - Before approving a valid baseline, compare each proposed buy with the top alternate buy candidates from `ticker events`.
   - Do not treat thin or missing source-backed updates as a standalone reason to reject or replace a proposed starter. For a momentum-led swing strategy, unexplained price/volume strength can still be a valid starter signal when deterministic score, profile-QVM, sizing, and validation are strong.
   - Treat a proposed starter as replacement-eligible when it has stale or missing critical data, mixed/adverse event context, materially weaker profile-QVM support, an event veto, a validation/risk issue, a severe concentration/sleeve concern, or adverse structured `mpal` sizing/rejection output that makes the baseline trade unattractive or impractical.
   - Treat an alternate as a better risk/reward candidate only when it appears in `ticker events.alternates` or deterministic `signals`, has no event veto, has comparable-or-better deterministic score/profile support or structured `mpal` sizing support when exposed, and is within 0.03 `final_score` of a replacement-eligible proposed starter unless the baseline has a validation or event veto issue. Do not use news context or agent-computed Kelly alone to replace a higher-scoring validated baseline starter.
   - Prefer one bounded replacement at a time: swap the weakest replacement-eligible proposed starter for the strongest qualifying alternate, preserve or reduce the validated baseline sizing unless `mpal` exposes a lower deterministic sizing target, preserve turnover, cash buffer, and max-new-position count, then validate the concrete override plan with `mpal portfolio validate`.
   - If the override validates, present it as `agent_override` for user review and journal it. If it fails validation, keep the baseline or veto based on the validation errors and event risk.
   - If no qualifying replacement exists, explicitly say why the baseline remains the best validated plan.

7. Explain the baseline before giving the decision.
   - Start with a compact baseline brief: strategy, date, portfolio source, universe, model result, execution result, proposed trades, selected signal scores, rejected tickers, and the scoring rule/threshold.
   - Include an "alternate buy candidates" section with up to five candidates. If the risk/reward gate already found and validated a superior bounded override, state the override directly instead of asking a follow-up question.
   - Include a sizing and Markov context section for proposed trades and alternates. Report structured `mpal` sizing fields when present, including horizon, binding constraint, final target, warnings, and calibration status. Show daily/weekly Markov only when available or requested, and label it as context unless `mpal` exposes a deterministic gate.
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
- Treat Markov transition metadata as context unless `mpal` has already consumed it into structured sizing output. The strategy's configured rebalance horizon is the primary horizon for executable sizing.
- Treat daily Markov as an execution-timing risk flag only. A negative daily read can support caution or watchlist language, but it must not mechanically delay, reduce, or reject a validated trade unless `mpal` exposes that deterministic gate.
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

## Weekly Swing Mode

Use this mode for weekly engine-sleeve or low-churn swing reviews.

- Use the selected strategy's configured rebalance horizon as the source of truth for the trade packet.
- Treat Kelly as short-horizon sizing support after the candidate already qualifies through signal threshold, profile-QVM, event context, freshness, policy, and validation.
- Use recent event context, normally `ticker events --days 14` unless the strategy or user specifies another window.
- Prefer smaller starter trades when `mpal` reports low Kelly confidence, missing Markov edge, fixed sizing fallback, or thin source context.
- Do not top up aggressively unless the ticker remains above the buy threshold, profile-QVM remains supportive, no event veto appears, and any exposed Kelly target remains above current weight.
- If several proposed buys are from the same sector/theme, flag correlation or concentration risk before approving the full packet. Do not treat individually positive Kelly-sized trades as independent bets.

## Monthly Swing Mode

Use this mode when the user asks for a monthly swing review, monthly cleanup, transition rebalance, or a monthly/quarterly engine rebuild.

- Use a longer event/context window, normally `ticker events --days 30` to `--days 45`, unless the strategy explicitly uses a shorter catalyst window.
- Require stronger persistence than a weekly entry: prefer candidates whose score remains above the strategy threshold and whose profile-QVM/event context support the holding period.
- Prefer lower turnover than weekly reviews. Be slower to replace existing holdings that remain above `min_hold_score` and have no event veto.
- Be cautious with Kelly if probability inputs are generated from a shorter horizon than the monthly review. If the Markov or Kelly horizon is weekly but the review is monthly, label it as short-horizon support only and do not let it justify aggressive monthly sizing.
- If the strategy output does not expose the Kelly/probability horizon, state that the sizing horizon is not exposed rather than assuming it matches the monthly holding period.
- For monthly reviews, prefer watchlist or smaller validated entries when source freshness is stale, event context is thin, or sizing depends on default payoff assumptions.

## Fractional Kelly Swing-Sizing Overlay

Use this overlay when the selected strategy config has `risk.sizing_method: fractional_kelly` or when `strategy run` returns `proposed_trades[].sizing`.

- Treat `mpal` executable sizing output as the source of truth when present. Do not invent Kelly inputs, Kelly fractions, gate statuses, payoff ratios, or confidence shrinkage.
- Agent-computed Kelly must not veto, cap, downsize, delay, promote, or make an alternate replacement-eligible. Only structured `mpal` sizing/rejection output or `mpal decision gate` evidence can support that.
- Kelly sizing is a sizing overlay, not an independent trade authorization layer. A ticker must first qualify through the selected strategy's signal threshold, portfolio policy, event guardrails, freshness checks, and validation rules.
- Markov transition metadata may be one input to deterministic `mpal` sizing. Raw Markov metadata remains explanatory and must not authorize, reject, replace, or resize a trade by itself.
- Always distinguish signal eligibility, Kelly sizing, risk caps, and validation:
  - signal eligibility: whether the ticker cleared the strategy threshold
  - Kelly sizing: how much `mpal` wanted to allocate before final clamps, if exposed
  - risk caps: why the final size may be smaller than the Kelly target
  - validation: whether the final concrete plan is executable
- For each Kelly-sized trade, report fields that are present in `mpal` output: sizing method, raw Kelly, fractional Kelly, `kelly_target_weight`, `final_target_weight`, `binding_constraint`, payoff ratio or default payoff assumption, current weight, delta weight, warnings, confidence, sample count, favorable/unfavorable probabilities, and calibration status. If the current CLI output does not expose a field, say that rather than estimating it.
- Report the sizing horizon when exposed by `proposed_trades[].sizing.horizon`; otherwise infer the strategy-run horizon from `signals[].markov.horizon` or `strategy.config.portfolio.rebalance` and label it as inferred. Do not call executable sizing weekly when the strategy rebalance is daily.
- Prefer `proposed_trades[].sizing.binding_constraint` when present. If it is absent, explain the binding constraint when inferable: Kelly target, max position, max single trade, turnover budget, cash buffer, min trade value, Kelly max cap, max new positions, protected holdings, or validation failure.
- If the binding constraint is not explicit in `mpal` output, say "binding constraint not exposed" rather than guessing.
- Never approve a larger trade merely because Kelly is high if event context, data quality, policy sleeve, liquidity, concentration, correlation, or validation is adverse.
- If Kelly input data is missing, stale, low-confidence, or below the strategy sample threshold, explain whether `mpal` fell back to fixed sizing, skipped the candidate, rejected it, or capped the size. Use `proposed_trades[].reason`, `proposed_trades[].sizing.warnings`, and `baseline_plan.rejected[].reason` as evidence.
- Do not interpret per-ticker Kelly fractions as portfolio-optimal Kelly allocations. Unless `mpal` explicitly provides a portfolio-level Kelly or covariance-aware optimizer, treat Kelly sizing as single-candidate edge sizing that is then clamped by portfolio construction rules.
- Describe calibration status honestly. If `mpal` reports `calibration_status: heuristic_markov`, call the Kelly sizing heuristic and confidence-shrunk. If `mpal` does not expose calibration evidence for the probability inputs, call the Kelly sizing experimental or confidence-shrunk, not proven.
- Prefer an agent veto, resize, or watchlist-only decision when structured `mpal` output exposes low-confidence transition data, sample count below the configured threshold, missing favorable/unfavorable probabilities, defaulted payoff assumptions, adverse event context despite a high Kelly target, poor liquidity/concentration/correlation context, or multiple highly correlated proposed trades that independently receive positive Kelly sizes.
- For weekly swing reviews, treat exposed Kelly as short-horizon sizing support. For monthly swing reviews, first check whether the exposed probability/Kelly horizon matches the monthly holding period; if not exposed or shorter horizon, label it as short-horizon support and keep sizing conservative.
- Never let high Kelly sizing override an event veto, stale critical data, validation failure, sleeve-policy conflict, severe concentration risk, or obvious correlation clustering across proposed trades.

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
- Include ticker, score, rank, `action_hint`, strategy-horizon Markov transition read when available, optional daily/weekly context when requested, sizing read when exposed, event read, source gaps, and the baseline rejection reason such as insufficient funding, min trade value, or buy threshold.
- If a clearly superior alternate passes the risk/reward gate and a concrete override validates, present the override directly. Otherwise ask one direct preference question, e.g. "Which, if any, should I validate as an override candidate?"
- Do not imply these are approved trades unless `mpal portfolio validate` passes on a concrete override plan.

### Trade Table / HTML Report Format

When the user asks for a table, asks "what would you trade today?", asks to compare proposed trades and alternates, asks about extra tickers, or the output would have more than eight table columns, prefer a local HTML report plus a compact chat summary. Use a Markdown table only for quick, narrow responses.

Default columns:

| Ticker | isTrade | Score | Role | Sizing Horizon | Raw Kelly | Frac Kelly | Accepted Sizing % | Accepted Sizing Price | Read |
| --- | ---: | ---: | --- | --- | ---: | ---: | ---: | ---: | --- |
| `AAPL` | true | 0.912 | Starter | weekly | 12.00% | 3.00% capped | 1.50% | USD 1,500 | One-sentence investment read. |

Report rules:

- Include all baseline proposed trades, up to five alternates, and any user-requested tickers in the same table.
- Use `Role` values such as `Starter`, `Top-up`, `Alternate`, `Trim`, `Exit`, or `user-request`. If the user explicitly asks for a custom role label such as `user-request`, use that exact label.
- `isTrade` is `true` only for the final validated executable plan or validated override being recommended. If a valid baseline is narrowed by an agent override, baseline trades excluded from the override must show `isTrade=false`.
- `Accepted Sizing %` and `Accepted Sizing Price` must reflect the final validated plan only. Non-selected candidates show `0.00%` and `0`, even when they had positive Kelly but were not selected or did not validate.
- Show raw Kelly and fractional Kelly only when `mpal` exposes them. Raw Kelly may be negative. Label fractional Kelly with the structured `binding_constraint`, warning, or validation state from `mpal`, such as `kelly_target`, `kelly_max_fraction`, `max_single_trade_pct`, `fixed_fallback`, `unavailable`, or `low-confidence`.
- The `Read` column should be a concise portfolio-manager note: model rank/score, profile-QVM support, event context, sizing/concentration issue, source-data issue, or why the ticker is not selected.
- For requested tickers not in the baseline context pack, fetch `ticker events --tickers ...`; fetch `ticker markov --tickers ...` only when the user asks for Markov context or the latest strategy run lacks a needed Markov read. Join those reads with deterministic `signals` from the latest strategy run when available.
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

- sizing method: fixed, fractional Kelly, or other configured method
- fixed starter size, if fixed sizing was used or used as fallback
- sizing horizon, Kelly target weight, raw Kelly, fractional Kelly, final clamped target weight, favorable/unfavorable probabilities, and calibration status when available
- binding constraint for each proposed trade from `mpal` output when present, otherwise when inferable
- turnover used versus the strategy turnover budget
- max single-trade cap
- max position cap
- cash buffer and available cash after buffer when exposed or calculable from the portfolio input
- starter position size
- max new positions per run
- protected or unscored holdings
- trade intents: `STARTER`, `TOP_UP`, `TRIM`, `REDUCE`, or `EXIT_CANDIDATE`
- structured sizing status from `mpal`: executable Kelly, fixed fallback, unavailable Markov, low-confidence/low-sample rejection, binding cap, or validation failure
- per-trade Kelly fields when available from `mpal`: favorable probability, unfavorable probability, confidence, sample count, raw Kelly, fractional Kelly, Kelly target weight, final target weight, binding constraint, horizon, calibration status, and clamp/fallback warnings

## Kelly Evidence Standard

When evaluating whether a Kelly-enabled strategy version improves the system, require evidence rather than assuming improvement:

- Compare fixed sizing versus Kelly sizing over the same backtest window when the current CLI/configs make that comparison available.
- Report available metrics such as CAGR/return, Sharpe, Sortino, max drawdown, turnover, average trade size, hit rate, payoff ratio, and tail-loss behavior when exposed by `mpal backtest run`.
- Check whether probability inputs are calibrated, defaulted, or heuristic if `mpal` exposes that status.
- Do not claim Kelly improves returns unless the backtest and risk metrics support it.
- If the CLI does not expose calibration or fixed-versus-Kelly comparison artifacts, state the gap and avoid promotional language.

## Journal Payload Shape

For `agent_veto`, `agent_override`, and `agent_final_action`, journal a JSON object with these fields:

```json
{
  "decision": "agent_veto",
  "decision_gate_evidence_hash": "sha256:example",
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
  "sizing_audit": [
    {
      "ticker": "AAPL",
      "source": "mpal_strategy_run",
      "method": "fractional_kelly",
      "horizon": "weekly",
      "horizon_bars": 5,
      "favorable_probability": 0.42,
      "unfavorable_probability": 0.33,
      "confidence": 0.55,
      "sample_count": 45,
      "raw_kelly": 0.12,
      "fractional_kelly": 0.0165,
      "kelly_target_weight": 0.0165,
      "final_target_weight": 0.015,
      "binding_constraint": "max_single_trade_pct",
      "calibration_status": "heuristic_markov",
      "warnings": ["Kelly target clamped by risk controls"]
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
