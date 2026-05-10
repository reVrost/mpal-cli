# MarketPal Data Gaps For Equity DD

Track reusable DD data needs that are not currently covered by `mpal-cli`. Use current `go run ./cmd/mpal capabilities --json` output, not an older installed binary, when reassessing this file. Use web or primary filings for reports until remaining gaps are added.

## Covered By Current mpal

- `mpal ticker fundamentals --tickers <csv> --json` covers compact profile-backed DD fields, including price, market cap, enterprise value, valuation multiples, forward/trailing EPS, selected estimates, DCF/target-price payloads where available, and selected credit fields.
- `mpal ticker financials --tickers <csv> --years <n> --include-ttm --json` covers historical financial statements and TTM context where MarketPal has stored data.
- `mpal ticker events --tickers <csv> --days <n> --json` covers recent source-backed events, including filings, ASX announcements, press releases, insider activity, institutional activity, and enriched article or announcement summaries where available.
- `mpal ticker insiders --tickers <csv> --days <n> --limit <n> --json` covers insider transaction feeds where available.
- `mpal ticker ownership --tickers <csv> --days <n> --limit <n> --json` covers institutional ownership feeds where available.
- `mpal ticker markov --tickers <csv> --date <date> --rebalance weekly --json` covers local Markov transition reads using hosted MarketPal price bars.

## Remaining Known Gaps

- Full primary-source filing and exchange-announcement document retrieval, including exact URLs/PDFs and document text extraction.
- Segment financials when they are not mapped into stored MarketPal financials.
- Consensus estimates and analyst forecast ranges.
- Debt maturity schedules, interest cost, covenant data, and credit metrics.
- Data-center-specific capacity metrics: MW, utilization, contracted backlog, power availability, development pipeline, stabilized yield, and tenant concentration.
- Transcript and investor-day retrieval.
- Capital-raising history and security issuance history.
- Peer-comparison datasets that normalize valuation and leverage metrics across a custom ticker set.

## Desired Future mpal Commands Or Enhancements

- `mpal company filings --ticker <ticker> --type annual|interim|presentation|announcement --json`
- `mpal company filing-text --ticker <ticker> --document-id <id> --json`
- `mpal company segments --ticker <ticker> --json`
- `mpal company debt --ticker <ticker> --json`
- `mpal sector comps --tickers <csv> --metrics ev_ebitda,pe,fcf_yield,net_debt_ebitda --json`
- `mpal company transcripts --ticker <ticker> --json`
- `mpal company capital-history --ticker <ticker> --json`
