# mpal-cli

`mpal` is a deterministic MarketPal CLI and MCP server for research review
workflows. It returns structured JSON for MarketPal data, strategy runs,
validation, backtests, and journals.

It is not a broker, not an autonomous trader, and cannot place live orders.

## Boundary

This repo is for research, validation, and record keeping. It does not provide
financial product advice, personal advice, investment advice, tax advice, or
legal advice.

`mpal` produces review artifacts from user-supplied inputs and MarketPal API
data. Those artifacts are not order instructions and do not recommend that any
person buy, sell, hold, trim, reduce, or exit any financial product.

Use with retail clients only if you are appropriately licensed or authorised
and perform any required suitability, disclosure, and compliance checks outside
this repository.

## Quick Start

Install the CLI and MCP server:

```sh
go install github.com/revrost/mpal-cli/cmd/mpal@latest
go install github.com/revrost/mpal-cli/cmd/mpal-mcp@latest
export MPAL_API_KEY=mpal_...
```

Smoke test:

```sh
mpal doctor --json
mpal capabilities --json
mpal strategy list --json
```

For agent setup, install the plugin and ask:

```text
Run the MarketPal onboarding skill and report the first-run checklist.
```

Codex:

```sh
codex plugin marketplace add revrost/mpal-cli --ref main
```

Then open `/plugins`, choose `MarketPal Plugins`, and install `marketpal`.

Claude Code:

```sh
claude plugin marketplace add revrost/mpal-cli
claude plugin install marketpal@marketpal-plugins
```

## Simplest Human Workflow

Use MarketPal as a review system, not as a trading bot. The normal human loop is:

```text
set up once
-> run a strategy review
-> inspect evidence and DD
-> validate the final human plan
-> journal what the model said vs what you did
-> later journal the outcome
```

### Which skill should I start with?

Start with `marketpal-onboarding` when you are installing MarketPal, checking a
fresh repo, wiring MCP/plugin tools, setting `MPAL_API_KEY`, smoke-testing
`mpal`, or confirming approved strategy configs.

Use `marketpal-trader` for the actual weekly or monthly review. This is the
main human-in-the-loop workflow: it runs the baseline strategy, reads the
decision gate, checks recent events, applies portfolio policy, validates any
override, and journals the final reviewed action.

Use `equity-dd-analyst` when you want deeper public-equity DD on a narrowed set
of names, a thematic comparison, or a source-backed investment memo. In the
normal workflow, run the strategy review first, then deepen DD on the proposed
trades, alternates, or user-requested tickers that still look relevant.

### First-time setup

1. Install `mpal` and `mpal-mcp`.
2. Set `MPAL_API_KEY` in the same shell or app environment that runs the agent.
3. Install the Codex or Claude Code plugin if you want skill-guided reviews.
4. Run:

```sh
mpal doctor --json
```

Use `mpal doctor --skip-api --json` for a local-only check, or
`mpal doctor --strict --json` in CI when missing required setup should return a
non-zero exit code.

5. Ask your agent:

```text
Run the MarketPal onboarding skill and report the first-run checklist.
```

6. Optional but recommended: create a private portfolio policy at:

```text
~/.marketpal/portfolio-policy.md
```

That file is where you describe sleeves, fixed/core holdings, contribution
rules, high-conviction holdings, review cadence, and whether reviews should be
full-portfolio or engine-only. Keep private holdings out of repo-tracked files.

### Normal weekly or monthly review

Ask your agent something like:

```text
Use the MarketPal trader skill to run my weekly engine review with
engine_weekly_swing_v1. Use my private portfolio policy if available. Show the
baseline, decision gate, alternates, validation result, and journal the final
review.
```

For a lower-churn full-portfolio review:

```text
Use the MarketPal trader skill to run a low-churn full-portfolio swing review
with portfolio_low_churn_swing_v1. Separate the raw model plan from my final
human action in the journal.
```

The trader workflow should do this:

1. Load `~/.marketpal/portfolio-policy.md` when present.
2. Choose an approved strategy config with `mpal strategy list/show`.
3. Run the baseline with `mpal strategy run`; the command auto-journals the
   deterministic first pass and returns `journal_entry_id`.
4. Pull recent source-backed context with `mpal ticker events`.
5. Read the deterministic evidence packet with `mpal decision gate`.
6. Explain model result, executable result, sizing, warnings, and alternates.
7. Validate the baseline or any human override with `mpal portfolio validate`.
8. Generate the deterministic HTML first pass with `mpal report <journal_entry_id>`.
9. Finalize the same journal entry with `mpal journal finalize` after the human decision.

### Where DD fits

Use DD after the model has narrowed the field, not before every review. A good
prompt is:

```text
Use the equity DD analyst skill on the proposed trades and top alternates from
the latest MarketPal review. Compare financials, scale, valuation, catalysts,
and risks. Keep the conclusion simple and source-backed.
```

DD can support a human veto, delay, resize, or bounded replacement, but the
final changed plan still needs `mpal portfolio validate` before it is journaled
as the reviewed action.

### What gets journaled?

Journal one accountable review in two phases:

- `mpal journal start`: records the strategy output, agent context, and
  per-ticker model/agent read.
- `mpal journal finalize`: records the final human decision, final validation,
  and final per-ticker human call.

This separation lets you measure whether the human overlay helped or hurt the
model over time without logging every intermediate command.

### Mental model

- `mpal strategy run`: the raw model packet.
- `mpal decision gate`: deterministic evidence, sizing, rejections, and
  alternates from the completed run.
- `marketpal-trader`: the human portfolio-manager review process.
- `mpal portfolio validate`: checks whether the final concrete plan obeys the
  strategy and portfolio rules.
- `mpal journal start/finalize`: records the accountable review decision in the
  SQLite journal.

The durable SQLite review-journal schema is documented in
[docs/REVIEW_JOURNAL_SQLITE.md](docs/REVIEW_JOURNAL_SQLITE.md). It stores
accountable review decisions, not every intermediate command or cache payload.

### Contributing map

- CLI commands live in `internal/cli/`.
- MCP tool wrappers live in `internal/mcpserver/`.
- Core engine, strategy, decision-gate, validation, and journal types live in
  `pkg/mpal/`.
- Agent skills live in `skills/`.
- Longer workflow docs live in `docs/`.
- Run `go test ./...` before opening a PR.

## Configuration

API-backed commands use `MPAL_API_KEY`. Optional environment variables:

```sh
export MPAL_BASE_URL=https://api.marketpal.ai
export MPAL_JOURNAL=~/.marketpal/journal.jsonl
```

Private portfolio policy belongs outside the repo:

```text
~/.marketpal/portfolio-policy.md
```

Agents may use that file to scope a review packet, but should not copy private
holdings or dollar amounts into tracked files.

## Strategies

Built-in configs:

- `portfolio_low_churn_swing_v1`: routine full-portfolio review packet
- `engine_weekly_swing_v1`: weekly return-engine sleeve review packet
- `engine_quality_swing_rebuild_v1`: manual engine cleanup or rebuild scenario
- `engine_quality_value_reversion_v1`: engine quality-value pullback review
- `portfolio_quality_value_reversion_v1`: full-portfolio quality-value pullback review
- `momentum_profile_v1`
- `momentum_only_v1`
- `simple_score_v1`

Custom configs can live in:

```text
~/.marketpal/strategies/
```

Strategy configs are YAML and can be checked against
`schemas/strategy.schema.json`. See
[docs/STRATEGY_CONFIGS.md](docs/STRATEGY_CONFIGS.md).
Quality-value mean-reversion configs use optional `quality_weight`,
`value_weight`, and `reversion_weight` scoring fields; all scoring weights must
sum to `1.0`. These fields use the local `scoring_v2_quality_value_reversion`
contract and are marked `api_compatible: false` until the hosted MarketPal API
supports the same scoring contract.

`config_hash` is calculated from the canonical expanded config that is sent to
the hosted API, not from the raw YAML bytes. Slim configs using `defaults` and
equivalent fully expanded configs therefore share the same hash.

The full review workflow is in
[docs/MARKETPAL_REVIEW_WORKFLOW.md](docs/MARKETPAL_REVIEW_WORKFLOW.md).

## Core Commands

Inspect capabilities and configs:

```sh
mpal doctor --json
mpal capabilities --json
mpal strategy list --json
mpal strategy show --id engine_weekly_swing_v1 --json
mpal strategy validate --config strategies/engine_weekly_swing_v1.yaml --json
```

Read MarketPal data:

```sh
mpal portfolio snapshot --json
mpal watchlist get --json
mpal ticker profile --tickers AAPL,MSFT,NVDA --date 2026-05-10 --json
mpal ticker events --tickers AAPL,MSFT,NVDA --days 14 --json
mpal ticker financials --tickers AAPL,MSFT,NVDA --years 6 --include-ttm --json
mpal ticker fundamentals --tickers AAPL,MSFT,NVDA --json
mpal ticker insiders --tickers AAPL,MSFT,NVDA --days 365 --limit 100 --json
mpal ticker ownership --tickers AAPL,MSFT,NVDA --days 365 --limit 100 --json
mpal ticker markov --tickers AAPL,MSFT,NVDA --date 2026-05-10 --rebalance weekly --json
```

`mpal ticker events` is the curated feed for recent source-backed context. It includes price/volume events, filings, ASX announcements, press releases, insider activity, institutional activity, and enriched article/announcement summaries when available.

`mpal ticker fundamentals` is a compact profile-backed DD packet. It includes valuation fields (`price`, `market_cap`, `enterprise_value`, `pe`, `forward_pe`, `pb`, `ps`, `ev_to_ebit`, `ev_to_fcf`, `fcf_yield`, DCF and target-price payloads), estimate fields (`forward_eps`, `trailing_eps`, `eps_growth`, projections, growth pattern, earnings date), and credit fields (`debt_to_equity`, `solvency_ratio`, Altman Z-score, latest debt/cash/working-capital fields from stored financials).

Run a strategy review packet:

```sh
mpal strategy run \
  --date 2026-05-10 \
  --universe examples/universe.json \
  --portfolio examples/portfolio.json \
  --config strategies/engine_weekly_swing_v1.yaml \
  --json
```

Build an evidence-bound decision gate from a completed strategy run:

```sh
mpal decision gate \
  --run tmp/mpal-runs/strategy-run.json \
  --alternates 5 \
  --json
```

Validate and journal a reviewed packet:

```sh
mpal portfolio validate \
  --plan examples/final_plan.json \
  --portfolio examples/portfolio.json \
  --universe examples/universe.json \
  --config strategies/engine_weekly_swing_v1.yaml \
  --json

mpal journal start --input examples/trade_review_start.json --json
mpal journal finalize --id review_... --input examples/trade_review_finalize.json --json
mpal journal list --limit 20 --json
```

Backtest:

```sh
mpal backtest run \
  --start 2025-01-01 \
  --end 2026-01-01 \
  --universe examples/universe.json \
  --config strategies/engine_weekly_swing_v1.yaml \
  --json
```

## Review Workflow

A normal research packet is:

1. Choose an approved config with `mpal strategy list --json`.
2. Inspect it with `mpal strategy show --id <strategy-id> --json`.
3. Run `mpal strategy run` with an explicit date, portfolio, universe, and
   config.
4. Add event context with `mpal ticker events`.
5. Validate any concrete baseline or alternative packet with
   `mpal portfolio validate`.
6. Start the review journal with `mpal journal start`.
7. Finalize it with `mpal journal finalize` after the human decision.

Scheduled or autonomous agent runs should only use approved configs. Agents
must not silently modify configs or invent one-off strategy parameters.

## MCP

`mpal-mcp` exposes the same boundary through Model Context Protocol tools. It
is a wrapper around the CLI/client logic, not a separate trading system.

Run over stdio:

```sh
mpal-mcp
```

Claude Code:

```sh
claude mcp add mpal --env MPAL_API_KEY="$MPAL_API_KEY" -- mpal-mcp
```

Codex:

```sh
codex mcp add mpal -- mpal-mcp
```

For local source checkout development, use `examples/mcp.local.json`, which
runs `go run ./cmd/mpal-mcp`.

There is no MCP tool for live order placement.

## Agent Rules

`mpal strategy run` is the source of truth for model output: signals, target
weights, proposed model actions, warnings, freshness metadata, strategy ID,
strategy version, config hash, and validation result.
Signals may include optional `markov` metadata with trend-state transition
probabilities over the strategy rebalance horizon. This metadata is explanatory
by default and does not change scoring, planning, or validation unless a
strategy explicitly enables the experimental `risk.sizing_method:
fractional_kelly` sizing overlay. That one setting uses conservative internal
Kelly defaults; users can tune `risk.kelly_fraction` for 25%, half Kelly, or
another value in `(0,1]`. Even then, Kelly sizing is only an input and proposed
trades remain clamped by the strategy's fixed risk controls, including any
limits supplied by `risk.profile`. Kelly-sized trades include structured sizing
audit fields such as `kelly_target_weight`, `final_target_weight`, `horizon`,
`horizon_bars`, `binding_constraint`, favorable/unfavorable probabilities, and
`calibration_status`. `mpal decision gate` packages those fields with rejected
tickers, alternate signal context, validation state, and an evidence hash so an
agent can approve, veto, delay, downsize, or propose a validated override
without inventing hidden model inputs.

Agents may summarize review packets or construct bounded alternative packets,
but must:

- use `mpal` JSON as source of truth
- not invent model actions outside `mpal` output
- validate alternative packets with `mpal portfolio validate`
- start review artifacts with `mpal journal start`
- finalize human decisions with `mpal journal finalize`
- never execute live trades or call broker/order-placement tools

## Development

```sh
go test ./...
go build ./cmd/mpal ./cmd/mpal-mcp
```

Generated ConnectRPC code is checked in. If
`proto/marketpal/v1/mpal.proto` changes, regenerate from the repo root:

```sh
buf generate
```

Plugin and registry distribution notes live in
[PLUGIN_DISTRIBUTION.md](PLUGIN_DISTRIBUTION.md).
