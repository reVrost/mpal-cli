# Security Policy

## Reporting a Vulnerability

Please report security issues privately by emailing `security@marketpal.ai`.
Do not open a public issue that contains API keys, tokens, account identifiers,
or exploit details.

## API Keys

`MPAL_API_KEY` is a bearer credential. Keep it out of source control, logs, and
shared prompts. If a key is exposed, rotate or revoke it in MarketPal before
sharing diagnostics.

The CLI sends the key only in the `Authorization: Bearer <key>` header to the
configured `MPAL_BASE_URL`.

## Trading Boundary

`mpal` does not contain live order placement. Security reports about any path
that appears able to place, route, or execute trades are high priority.
