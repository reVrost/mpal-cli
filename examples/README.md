# Examples

These demo artifacts are safe sample inputs for local commands. They let a new
user try the review flow without using private portfolio files.

- `portfolio.json`: small sample portfolio
- `universe.json`: sample stock list
- `final_plan.json`: sample final plan for validation
- `final_action.json`: sample payload for saving a reviewed decision
- `demo/`: committed fixture workflow for the no-key demo commands
- `mcp.local.json`: local MCP development config

Run the deterministic no-key demo first. It uses only `examples/demo/` fixtures,
labels output as demo data, and never calls the live MarketPal API.

```sh
mpal demo run --json
mpal demo report
mpal demo journal --json
```

API-backed commands still require `MPAL_API_KEY`. Local validation can use the
top-level sample files without an API key:

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
