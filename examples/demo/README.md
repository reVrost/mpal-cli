# mpal demo fixtures

These fixtures power `mpal demo` and are safe to run without `MPAL_API_KEY`.
They are deterministic sample data, not live MarketPal output and not investment
advice.

The demo workflow is:

```sh
mpal demo run --json
mpal demo report
mpal demo journal --json
```

The workflow uses only the files in this directory and writes local artifacts
under `tmp/mpal-demo/` by default.

