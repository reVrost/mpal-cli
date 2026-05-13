# Examples

These demo artifacts are safe sample inputs for local commands. They let a new
user try the review flow without using private portfolio files.

- `portfolio.json`: small sample portfolio
- `universe.json`: sample stock list
- `final_plan.json`: sample final plan for validation
- `final_action.json`: sample payload for saving a reviewed decision
- `mcp.local.json`: local MCP development config

API-backed commands still require `MPAL_API_KEY`.

```sh
mpal portfolio validate \
  --plan examples/final_plan.json \
  --portfolio examples/portfolio.json \
  --universe examples/universe.json \
  --config strategies/momentum_profile_v1.yaml \
  --json
```

For local MCP development from this checkout:

```sh
cp examples/mcp.local.json .mcp.json
export MPAL_API_KEY=mpal_...
```

Then point your MCP-compatible client at the project `.mcp.json`.
