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

Validate and journal a reviewed packet:

```sh
mpal portfolio validate \
  --plan examples/final_plan.json \
  --portfolio examples/portfolio.json \
  --universe examples/universe.json \
  --config strategies/engine_weekly_swing_v1.yaml \
  --json

mpal journal append --type agent_final_action --input examples/final_action.json --json
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
6. Journal the reviewed artifact with `mpal journal append`.

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
strategy version, config hash, validation result, and journal entry ID.
Signals may include optional `markov` metadata with trend-state transition
probabilities over the strategy rebalance horizon. This metadata is explanatory
by default and does not change scoring, planning, or validation unless a
strategy explicitly enables the experimental `risk.sizing_method:
fractional_kelly` sizing overlay. That one setting uses conservative internal
Kelly defaults; users can tune `risk.kelly_fraction` for 25%, half Kelly, or
another value in `(0,1]`. Even then, Kelly sizing is only an input and proposed
trades remain clamped by the strategy's fixed risk controls, including any
limits supplied by `risk.profile`. Kelly-sized trades include structured sizing
audit fields such as `kelly_target_weight`, `final_target_weight`,
`binding_constraint`, favorable/unfavorable probabilities, and
`calibration_status`.

Agents may summarize review packets or construct bounded alternative packets,
but must:

- use `mpal` JSON as source of truth
- not invent model actions outside `mpal` output
- validate alternative packets with `mpal portfolio validate`
- journal review artifacts with `mpal journal append`
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
