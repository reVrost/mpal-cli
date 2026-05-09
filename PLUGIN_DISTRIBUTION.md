# MarketPal Agent Distribution

MarketPal should be distributed as an MCP server first, then wrapped for each
agent client. MCP is the shared capability layer for Claude Code, Claude
Desktop, Codex, Cursor, and other clients.

## Current Repo Status

- `mpal` and `mpal-mcp` build from Go.
- `.mcp.json` starts the stdio MCP server with `MPAL_API_KEY`.
- `.codex-plugin/plugin.json` packages the Codex plugin metadata, bundled
  skills, and MCP config.
- `.agents/plugins/marketplace.json` exposes the Codex plugin through a repo
  marketplace.
- `.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json` expose the
  same skill and MCP config through Claude Code's plugin marketplace format.
- `registry/server.json` is ready for the MCP Registry once a matching OCI
  image is public.

## Install Paths

### Universal MCP Install

Use this when a user only wants the tools, without a plugin wrapper.

```sh
go install github.com/revrost/mpal-cli/cmd/mpal@latest
go install github.com/revrost/mpal-cli/cmd/mpal-mcp@latest
export MPAL_API_KEY=mpal_...
```

Claude Code:

```sh
claude mcp add --scope user --transport stdio \
  --env MPAL_API_KEY="$MPAL_API_KEY" \
  --env MPAL_BASE_URL="${MPAL_BASE_URL:-https://api.marketpal.ai}" \
  mpal -- mpal-mcp
```

Codex:

```sh
codex mcp add mpal -- mpal-mcp
```

Then set `MPAL_API_KEY` in the shell that launches Codex, or in the user's
Codex MCP configuration.

### Codex Plugin Install

The repo marketplace lets Codex users install the plugin from this repository:

```sh
codex plugin marketplace add revrost/mpal-cli --ref main
```

Then open Codex, run `/plugins`, switch to `MarketPal Plugins`, and install
`marketpal`.

The plugin includes:

- `skills/marketpal-onboarding/SKILL.md`
- `skills/marketpal-trader/SKILL.md`
- `.mcp.json`
- `.codex-plugin/plugin.json`

Users still need `mpal-mcp` on `PATH` and `MPAL_API_KEY` available.

### Claude Code Plugin Install

Claude Code users can add the marketplace and install the plugin:

```sh
claude plugin marketplace add revrost/mpal-cli
claude plugin install marketpal@marketpal-plugins
```

The plugin reuses the same root-level `skills/` and `.mcp.json`. Users still
need `mpal-mcp` on `PATH` and `MPAL_API_KEY` available.

For local plugin development:

```sh
claude --plugin-dir .
```

### Claude Desktop Extension

Claude Desktop's friendliest local install path is an `.mcpb` bundle. Add this
after the CLI/MCP server is stable:

1. Build release binaries for macOS, Linux, and Windows.
2. Create an MCPB `manifest.json` with an `MPAL_API_KEY` user setting.
3. Bundle the platform binary and manifest into `.mcpb`.
4. Attach the `.mcpb` to GitHub Releases.
5. Optionally add an MCP Registry `mcpb` package entry with `fileSha256`.

This is separate from the Claude Code plugin marketplace.

## Release Checklist

1. Add a real Git remote and tags such as `v0.1.0`.
2. Push a public GHCR image matching `registry/server.json`, or change the
   registry package to a published release artifact.
3. Decide whether plugin manifests should pin `version` or rely on Git commits.
   If version is pinned, bump it on every public plugin release.
4. Add release automation with GoReleaser or GitHub Actions for `mpal`,
   `mpal-mcp`, checksums, and container images.
5. Add install smoke tests for:
   - `mpal capabilities --json`
   - `mpal-mcp` MCP initialize/list-tools
   - `claude plugin validate .`
6. Keep the plugin skill, README, and embedded strategy list aligned whenever
   public strategy configs are added or removed.

## Normal-User Product Notes

- Reduce setup friction by supporting Homebrew, npm, or signed release binaries.
  `go install` is fine for developers, but not for normal users.
- First Homebrew target: create a separate `revrost/homebrew-tap` repo with an
  `mpal` formula, so the user command is `brew install revrost/tap/mpal`.
  The formula should install both `mpal` and `mpal-mcp`.
- Prefer hosted HTTP MCP plus OAuth/API-key auth for the least local setup.
- Keep every tool explicitly non-order-placement. The current no-live-trading
  boundary is a strength and should stay visible in tool descriptions,
  manifests, docs, and examples.
- Add screenshots or short terminal recordings for plugin install flows before
  submitting to official directories.
