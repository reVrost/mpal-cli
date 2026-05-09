# mpal-cli

`mpal` is a deterministic MarketPal capability CLI for agent harnesses and
human operators. It exposes MarketPal data, strategy, portfolio planning,
backtesting, and journal capabilities as structured JSON.

It is not an autonomous trader, not a broker client, and cannot execute live
orders.

## Install

```sh
go install github.com/revrost/mpal-cli/cmd/mpal@latest
```

For local development:

```sh
go test ./...
go build ./cmd/mpal
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

- `simple_score_v1`
- `momentum_only_v1`
- `momentum_profile_v1`

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

## Agent Boundary

`mpal strategy run` produces the deterministic baseline: signals, target
weights, proposed trades, warnings, freshness metadata, strategy ID, strategy
version, config hash, validation result, and journal entry ID.

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
