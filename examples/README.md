# Examples

These files are safe sample inputs for local validation commands. API-backed
commands still require `MPAL_API_KEY`.

```sh
mpal portfolio validate \
  --plan examples/final_plan.json \
  --portfolio examples/portfolio.json \
  --universe examples/universe.json \
  --config strategies/momentum_profile_v1.yaml \
  --json
```
