---
name: portfolio-review
description: Use when a user asks for a portfolio review, risk review, sleeve allocation review, concentration analysis, return-target feasibility check, trim/exit rules, portfolio cleanup plan, or next-action roadmap for a stock portfolio. Use current Marketpal/mpal portfolio data where available, apply private portfolio policy when present, score holdings and themes with a professional risk/reward rubric, and do not provide personal financial advice or execute live trades.
---

# Portfolio Review

Run a portfolio-manager style review that turns current holdings, policy, thesis, and Marketpal signals into a concise risk map and next-action plan. This skill is for whole-portfolio and engine-sleeve risk reviews, not single-name DD or routine executable trade packets.

## Hard Rules

- Use current data. For live holdings, prices, transactions, events, laws, tax, and market context, fetch current data with `mpal` and/or browsing as needed.
- Load `~/.marketpal/portfolio-policy.md` before real portfolio reviews when it exists. Apply sleeve targets, fixed/core holdings, high-conviction guardrails, contribution rules, and exit/trim policy.
- Treat Marketpal output as the deterministic trading source of truth. Do not invent executable buys, sells, trims, targets, Kelly sizing, validation status, or journal entries.
- Do not execute live trades. Validate concrete trade packets with `mpal portfolio validate` before describing them as executable.
- Separate facts, inference, and opinion. Frame conclusions as a research and risk-management view based on stated assumptions, not personal financial advice.
- Protect private details. Treat holdings, weights, transactions, policy text, tax lots, risk reports, and generated review artifacts as private. Do not write them to repo-tracked paths unless the user explicitly asks for that exact artifact.
- Use `marketpal-trader` for executable weekly/monthly trade packet reviews, validated overrides, reports, and journal finalization. Use `equity-dd-analyst` for deep single-name or sector DD.

## Review Modes

Pick the narrowest mode that satisfies the request:

- **Quick risk check**: current weights, top risks, immediate watch items, and one next action.
- **Full portfolio review**: sleeves, clusters, return target feasibility, holding scorecard, trim/exit candidates, and action roadmap.
- **Engine cleanup review**: active sleeve only; identify weak, duplicated, sub-scale, or off-thesis holdings and validate any concrete trims/exits.
- **Exit/trim policy build**: draft durable standing rules; update `~/.marketpal/portfolio-policy.md` only after explicit user approval, and preserve a dated backup before editing.
- **Quarterly allocation review**: compare core/engine/high-conviction sleeves with target bands and decide where new cash or trims should go.

## Workflow

1. Define scope.
   - Infer today's date unless the user provides another date.
   - Infer the review universe from the current portfolio unless the user names a sleeve or ticker set.
   - If the user states a return target, convert it into required returns for each sleeve and call out realism, volatility, and drawdown implications.

2. Build the current-state pack.
   - If Marketpal capabilities are uncertain, run `mpal capabilities --json` first.
   - Create a private run directory such as `~/.marketpal/reviews/YYYY-MM-DD/` or `tmp/mpal-private/portfolio-review-YYYY-MM-DD/`. Use `~/.marketpal/reviews/` for durable private artifacts; use ignored temp paths for short-lived local work.
   - Run `mpal portfolio snapshot --json` for holdings, values, weights, and freshness, and save it to a private artifact path before using it as `--portfolio <portfolio.json>`.
   - Run `mpal portfolio transactions --limit <n> --json` when fills, tax lots, recent deployment, or journaling matter, and save it privately.
   - Load `~/.marketpal/portfolio-policy.md` when present.
   - For key holdings and risky clusters, batch `mpal ticker profile --tickers ... --json`, `mpal ticker fundamentals --tickers ... --json`, `mpal ticker financials --tickers ... --json`, and `mpal ticker events --tickers ... --days 14 --json` when valuation, balance sheet, cash flow, dilution, or event risk affects the conclusion.
   - Add `mpal ticker insiders --tickers ... --json`, `mpal ticker ownership --tickers ... --json`, or bars/Markov context when insider flow, ownership flow, liquidity, or timing is part of the risk call.
   - For engine reviews, choose the approved strategy from the private policy or `mpal strategy list --json` / `mpal strategy show --id <id> --json`; save the universe and portfolio JSON artifacts, then run `mpal strategy run --date <date> --universe <universe.json> --portfolio <portfolio.json> --config <strategy.yaml> --json` when executable trade or cleanup evidence is needed.
   - After a strategy run, use `mpal ticker events --run <run.json> --portfolio <portfolio.json> --days <window> --json` and, when useful, `mpal decision gate --run <run.json> --events <events.json> --config <strategy.yaml> --alternates 5 --json`.

3. Map the portfolio before judging it.
   - Split holdings into policy sleeves: core, Marketpal engine, high-conviction, cash/unassigned.
   - Derive thesis buckets from the holdings, policy, and user thesis. Examples include AI/big tech, AI infrastructure/electrification, lithium/decarbonization, fintech/crypto, consumer, resources, defensives, and cash.
   - Compute top position weight, top 5 weight, top 10 weight, sleeve drift, theme concentration, and recent net deployment.
   - Identify hidden correlation: names that look different but depend on the same macro factor, funding cycle, commodity, multiple expansion, AI capex cycle, or risk-on liquidity.
   - Reconcile weights to roughly 100%. If cash, stale prices, currency conversion, or missing fields prevent reconciliation, state the gap before drawing conclusions.

4. Test return-target feasibility.
   - Use the portfolio identity: `target_return = sum(sleeve_weight * required_sleeve_return) + contribution_effect`.
   - If one sleeve is assumed to return a known rate, solve for the required return of the active/risk sleeve: `required_active_return = (target_return - other_sleeve_contributions - contribution_effect) / active_sleeve_weight`.
   - Show at least base, upside, and stress cases when the user gives an ambitious annual target.
   - Include what must go right, likely drawdown range, max drawdown tolerance, and what would force de-risking.

5. Score with the professional rubric.
   - Load `references/portfolio-review-rubric.md` for the 100-point portfolio score, holding score, and action labels when doing anything more than a quick check.
   - Score the whole portfolio first, then the main holdings or engine holdings.
   - Use the rubric to decide whether the next action is add, hold, trim, exit, rotate, journal, or wait.
   - Do not let a high-conviction story override position sizing, liquidity, financing risk, validation failure, or correlation clustering.

6. Build exit and trim logic.
   - Apply the user's private exit/trim rules first when present.
   - If no durable rules exist, propose conservative defaults:
     - Review any single active holding above 8% of the total portfolio.
     - Default trim review above 10% unless policy says it is core/high-conviction.
     - Review any off-policy theme cluster above its working cap.
     - Down 15% from cost: review; no averaging down without refreshed thesis and validation.
     - Down 25% from cost: require event-driven or monthly rebuild review.
     - Down 35% from cost: default major review; exit or resize only after thesis, event, liquidity, and validation checks unless the private policy explicitly mandates a forced action.
   - For a concrete trade packet, write a plan artifact and validate it with:
     `mpal portfolio validate --plan <plan.json> --portfolio <portfolio.json> --universe <universe.json> --config <strategy.yaml> --json`.
   - If MCP Marketpal tools are available, use the equivalent `mpal_portfolio_validate` call with the same explicit inputs.

7. Journal accountable decisions only.
   - Do not journal snapshots, ticker events, profile/fundamental fetches, or broad exploration as separate reviews.
   - For normal strategy reviews, use the `journal_entry_id` returned by `mpal strategy run`; that first row already exists.
   - Use `mpal journal start --input <path-or-json> --json` only for manually assembled or imported reviews that did not come from `strategy run`.
   - After the human chooses a final action, finalize the same review with `mpal journal finalize --id <review-id> --input <finalize.json> --json`.
   - If no trade is accepted, journal only when the no-trade or watchlist decision is an accountable review outcome the user wants recorded.

8. Handle durable policy edits safely.
   - For policy updates, first produce a proposed change summary and, when practical, a diff.
   - Ask for explicit user approval before changing `~/.marketpal/portfolio-policy.md` unless the user has already directly requested that exact edit in the current turn.
   - Before editing, copy the existing policy to `~/.marketpal/portfolio-policy.backup-YYYYMMDD-HHMMSS.md`.
   - After editing, re-read the changed section and summarize what changed without exposing unnecessary private details.

9. Produce a decision-useful memo.
   - Start with the bottom line: portfolio state, main risk, and next focus.
   - Include a freshness/data block: snapshot timestamp, source, stale flags, missing cash/currency/price fields, and any data gaps that affect confidence.
   - Include a compact table of largest holdings and theme exposures.
   - Explain whether the stated return target is mathematically plausible given sleeve weights and required active-sleeve returns.
   - List immediate actions, monthly cleanup candidates, and what would change the view.
   - Include the next review date or trigger for every major risk item.
   - Keep action language bounded: "review", "validate", "trim candidate", "watchlist", or "accepted validated packet" as appropriate.

## Output Shape

Prefer this order for full reviews:

1. Bottom line
2. Current portfolio facts
3. Sleeve and theme exposure
4. Return-target feasibility
5. Scorecard and key risks
6. Trim/exit/watchlist candidates
7. Next actions by time horizon
8. Validation/journal status when relevant
9. Next review date/trigger

Use a local HTML or Markdown report only when the review is too wide for chat or the user asks for an artifact. Reports containing holdings, transactions, weights, policy details, or risk analysis must go under a private path such as `~/.marketpal/reviews/YYYY-MM-DD/` unless the user explicitly requests a repo-tracked artifact. Keep chat output compact.

## Quality Gate

Before finalizing:

- Check that all weights add up or explain missing cash/data.
- Check that the recommendation follows the user's private sleeve policy.
- Check that high-return ambition is translated into required active-sleeve returns.
- Check that every proposed executable trade is validated; otherwise label it as a candidate only.
- Check that private portfolio artifacts and reports are not written to repo-tracked paths without explicit user request.
- Check that durable policy edits had explicit approval or a direct same-turn request, and that a backup was created.
- Check the conclusion against `references/portfolio-review-rubric.md`. Improve the review if evidence quality, position-sizing discipline, or actionability is weak.
- For skill evaluation or regression testing, use `references/eval-prompts.md` and verify the answer includes weight reconciliation, validation status, journal status, and stale-data warnings where relevant.
