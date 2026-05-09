---
name: marketpal-onboarding
description: Use when a user needs to install, configure, or smoke-test MarketPal in Codex or Claude Code; verify mpal/mpal-mcp setup, MPAL_API_KEY, MCP/plugin installation, approved strategy configs, private ~/.marketpal/portfolio-policy.md, and safe first-run workflows after repo or strategy changes.
---

# MarketPal Onboarding

Use this skill to get a user from a fresh checkout or plugin install to a safe,
working MarketPal setup. It is for setup and smoke tests, not trade approval.
Use `marketpal-trader` for actual strategy reviews, validation, vetoes,
overrides, and journaling.

## Setup Workflow

1. Check the local environment.
   - Run `command -v mpal` and `command -v mpal-mcp`.
   - If either command is missing, recommend the repo-local development command
     (`go run ./cmd/mpal ...`) or the install path documented in `README.md`.
   - Check whether `MPAL_API_KEY` is available. Do not print the key.
   - If `.envrc` exists, note that local shells may need `direnv allow` or an
     explicit `source .envrc` for development runs.

2. Smoke-test the CLI.
   - Run `mpal capabilities --json` when `mpal` is on `PATH`; otherwise use
     `go run ./cmd/mpal capabilities --json` from the repo.
   - Run `mpal strategy list --json` and confirm the expected approved configs
     are visible.
   - Run `mpal strategy show --id <id> --json` for any config the user plans to
     use.

3. Explain the current strategy set.
   - `portfolio_low_churn_swing_v1`: routine full-portfolio swing review with
     conservative turnover and minimum trade rules.
   - `engine_weekly_swing_v1`: weekly MarketPal return-engine sleeve review;
     deploys assigned engine cash into ranked swing candidates without forcing
     cleanup sells.
   - `engine_quality_swing_rebuild_v1`: manual higher-turnover engine cleanup
     or transition rebuild; do not use as the default weekly strategy.

4. Check private portfolio policy.
   - Look for `~/.marketpal/portfolio-policy.md`.
   - If present, summarize only the operational rules needed for setup: sleeve
     mapping, fixed holdings, contribution routing, review cadence, and whether
     engine-only reviews are required.
   - If absent, offer to create a private starter policy under `~/.marketpal/`.
     Do not put private portfolio policy in repo-tracked files.

5. Verify plugin or MCP wiring.
   - For Codex plugin installs, check `.codex-plugin/plugin.json`,
     `.agents/plugins/marketplace.json`, `skills/`, and `.mcp.json`.
   - For Claude Code plugin installs, check `.claude-plugin/plugin.json`,
     `.claude-plugin/marketplace.json`, `skills/`, and `.mcp.json`.
   - Confirm the plugin packages both onboarding and trader skills through the
     root `skills/` directory.
   - Confirm MCP starts `mpal-mcp` and receives `MPAL_API_KEY`.

6. Run a non-trading smoke test.
   - Prefer read-only commands first: capabilities, strategy list/show,
     portfolio snapshot, and watchlist get.
   - If the user wants a strategy smoke test, use example or temporary inputs
     and an approved config.
   - For a real portfolio first run, use `marketpal-trader` next and keep live
     trading disabled. The onboarding skill must not approve trades.

## First-Run Checklist

Report setup status with these fields:

- `mpal`: found/missing, version if available.
- `mpal-mcp`: found/missing.
- `MPAL_API_KEY`: set/missing, without revealing the value.
- `MCP/plugin`: configured/missing/unclear.
- `approved strategies`: found list and validation status.
- `private policy`: found/missing, path only.
- `safe next command`: one concrete command to run next.

## Troubleshooting Rules

- If API calls fail as unauthenticated, first check whether `MPAL_API_KEY` is
  actually present in the shell running Codex or Claude Code.
- If `strategy run` fails on a missing date, pass an explicit `--date
  YYYY-MM-DD`.
- If plugin install works but MCP tools fail, check whether `mpal-mcp` is on
  `PATH` for the app process, not just the terminal.
- If strategy names look stale, run `mpal strategy list --json` and compare
  against the three current configs above.
- If a user asks "what should I trade?", switch to `marketpal-trader` after
  setup is verified.

## Hard Rules

- Never execute live trades or call broker/order-placement tools.
- Never print secrets, API keys, or full private policy contents unless the
  user explicitly asks to inspect that local file.
- Never store private portfolio policy in repo-tracked files.
- Do not tune strategy parameters during onboarding; create or update strategy
  configs only when the user explicitly asks for strategy development.
