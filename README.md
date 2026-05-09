# mpal-cli

`mpal` is a deterministic MarketPal capability CLI for agent harnesses and
human operators. It exposes MarketPal data, strategy, portfolio planning,
backtesting, and journal capabilities as structured JSON.

It is not an autonomous trader, not a broker client, and cannot execute live
orders.

## Install

```sh
go install github.com/revrost/mpal-cli/cmd/mpal@latest
go install github.com/revrost/mpal-cli/cmd/mpal-mcp@latest
```

For local development:

```sh
go test ./...
go build ./cmd/mpal ./cmd/mpal-mcp
```

## Authentication

MarketPal API-backed commands use an API key:

```sh
export MPAL_API_KEY=mpal_...
```

The server maps this key to the MarketPal user ID. The CLI does not accept a
user ID flag for current portfolio or watchlist data.

Optional environment variables:

```sh
export MPAL_BASE_URL=https://api.marketpal.ai
export MPAL_JOURNAL=~/.marketpal/journal.jsonl
```

## Built-in Strategies

The CLI embeds official strategy configs so installed binaries can list and
show them without needing the source tree:

- `portfolio_low_churn_swing_v1`
- `engine_weekly_swing_v1`
- `engine_quality_swing_rebuild_v1`
- `momentum_profile_v1`
- `momentum_only_v1`
- `simple_score_v1`

For agent-assisted routine full-portfolio reviews, prefer
`portfolio_low_churn_swing_v1`. For weekly MarketPal return-engine sleeve
reviews, prefer `engine_weekly_swing_v1`. Treat
`engine_quality_swing_rebuild_v1` as a manual, higher-churn MarketPal engine
cleanup/rebuild config unless the user explicitly asks for it.

For the recommended weekly/monthly/quarterly swing-trading workflow, see
[docs/HOW_TO_SWING_TRADE.md](docs/HOW_TO_SWING_TRADE.md).

Users can add custom configs under:

```text
~/.marketpal/strategies/
```

Scheduled or autonomous agent runs should only use approved strategy configs.
Agents must not silently modify configs or invent one-off strategy parameters.

## Commands

```sh
mpal capabilities --json
mpal strategy list --json
mpal strategy show --id momentum_profile_v1 --json
mpal strategy validate --config strategies/momentum_profile_v1.yaml --json

mpal portfolio snapshot --json
mpal watchlist get --json

mpal strategy run \
  --date 2026-05-09 \
  --universe examples/universe.json \
  --portfolio examples/portfolio.json \
  --config strategies/momentum_profile_v1.yaml \
  --json

mpal ticker bars --ticker AAPL --start 2026-01-01 --end 2026-05-09 --json
mpal ticker profile --ticker AAPL --date 2026-05-09 --json
mpal ticker events --tickers AAPL,MSFT,NVDA --days 14 --json

mpal portfolio validate \
  --plan examples/final_plan.json \
  --portfolio examples/portfolio.json \
  --universe examples/universe.json \
  --config strategies/momentum_profile_v1.yaml \
  --json

mpal backtest run \
  --start 2025-01-01 \
  --end 2026-01-01 \
  --universe examples/universe.json \
  --config strategies/momentum_profile_v1.yaml \
  --json

mpal journal append --type agent_final_action --input examples/final_action.json --json
mpal journal list --limit 20 --json
mpal journal get --id <journal-entry-id> --json
```

## MCP Server

`mpal-mcp` exposes the same capability boundary through Model Context Protocol
tools. It is a thin wrapper around the CLI/client logic, not a separate trading
brain.

Install:

```sh
go install github.com/revrost/mpal-cli/cmd/mpal-mcp@latest
export MPAL_API_KEY=mpal_...
```

Run over stdio:

```sh
mpal-mcp
```

Claude Code local install:

```sh
claude mcp add mpal --env MPAL_API_KEY="$MPAL_API_KEY" -- mpal-mcp
```

Codex local install:

```sh
codex mcp add mpal -- mpal-mcp
```

Claude Code project config can also use this repo's `.mcp.json`. For local
source checkout development, use `examples/mcp.local.json`, which runs
`go run ./cmd/mpal-mcp`.

The MCP server exposes these tools:

```text
mpal_capabilities
mpal_strategy_list
mpal_strategy_show
mpal_strategy_validate
mpal_strategy_run
mpal_portfolio_snapshot
mpal_watchlist_get
mpal_ticker_bars
mpal_ticker_profile
mpal_ticker_events
mpal_portfolio_validate
mpal_backtest_run
mpal_journal_append
mpal_journal_list
mpal_journal_get
```

There is no MCP tool for live order placement.

## Codex Plugin

This repo is also structured as a Codex plugin:

```text
.codex-plugin/plugin.json
.agents/plugins/marketplace.json
.mcp.json
skills/marketpal-trader/SKILL.md
skills/marketpal-onboarding/SKILL.md
```

The plugin packages `marketpal-onboarding`, `marketpal-trader`, and the `mpal`
MCP server configuration. Use onboarding for install checks and safe smoke
tests; use trader for strategy reviews, validation, and journaling. Codex users
should install `mpal-mcp`, set `MPAL_API_KEY`, then add this repo as a plugin
marketplace:

```sh
codex plugin marketplace add revrost/mpal-cli --ref main
```

Then open `/plugins`, choose `MarketPal Plugins`, and install `marketpal`.

## Claude Code Plugin

This repo also includes a Claude Code plugin marketplace:

```text
.claude-plugin/plugin.json
.claude-plugin/marketplace.json
.mcp.json
skills/marketpal-trader/SKILL.md
skills/marketpal-onboarding/SKILL.md
```

Claude Code users should install `mpal-mcp`, set `MPAL_API_KEY`, then run:

```sh
claude plugin marketplace add revrost/mpal-cli
claude plugin install marketpal@marketpal-plugins
```

For local development, load the plugin directly:

```sh
claude --plugin-dir .
```

See `PLUGIN_DISTRIBUTION.md` for the current distribution checklist and the
Claude Desktop `.mcpb` path.

## MCP Registry

The MCP Registry metadata lives at:

```text
registry/server.json
```

The official registry currently discovers installable servers through supported
package types such as npm, PyPI, NuGet, OCI, MCPB, or remote MCP URLs. This repo
uses the OCI path:

```text
ghcr.io/revrost/mpal-cli:0.1.0
```

Before publishing `registry/server.json`, push a matching container image. The
Dockerfile includes the required MCP server-name label:

```text
io.modelcontextprotocol.server.name=io.github.revrost/mpal
```

## Agent Boundary

`mpal strategy run` produces the deterministic baseline: signals, target
weights, proposed trades, warnings, freshness metadata, strategy ID, strategy
version, config hash, validation result, and journal entry ID.

When using MCP, `mpal_strategy_run` is the equivalent source-of-truth tool.

An external agent may explain, veto, or propose a bounded override, but it must:

- use `mpal` JSON as source of truth
- never invent trades outside `mpal` output
- validate overrides with `mpal portfolio validate`
- journal final actions with `mpal journal append`
- never execute live trades

## Data Freshness

MarketPal responses surface freshness and warnings when data could be stale.
Examples include historical price storage, live provider fetches, profile/QVM
snapshot age, missing profile fallback, and backtest trust diagnostics. Agent
harnesses should include these fields in investor-facing explanations.

## Regenerating Protobuf Code

Generated ConnectRPC code is checked in. If `proto/marketpal/v1/mpal.proto`
changes, regenerate from the repo root:

```sh
buf generate
```

## Disclaimer

This software is for research and portfolio-planning workflows. It is not
financial advice and does not place trades.
