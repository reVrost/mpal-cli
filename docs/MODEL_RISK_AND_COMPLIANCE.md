# Model Risk And Compliance Boundaries

`mpal-cli` is a research review and record-keeping tool. It is not a broker,
not an order-routing system, not an autonomous trading system, and not a
source of financial product advice, personal advice, investment advice, tax
advice, or legal advice.

## Human Responsibility

Every `TRADE`, `NO_TRADE`, `BUY`, `TRIM`, `REDUCE`, `EXIT_CANDIDATE`,
`watchlist`, `veto`, `resize`, or `delay` label is review evidence. It is not
an instruction to place an order.

A human reviewer remains responsible for:

- suitability and personal circumstances;
- licensing, authorization, disclosure, and record-keeping obligations;
- tax, transaction-cost, liquidity, and market-impact review;
- verifying current prices, order sizes, and market status before any order;
- broker execution, if any, outside this repository.

There is no `mpal` CLI or MCP tool that executes live trades.

## Model Risk

MarketPal scores, strategy output, Markov reads, raw Kelly fields, event
summaries, and agent explanations can be wrong, stale, incomplete, or
mis-scoped. Common model-risk cases include:

- stale portfolio, price, profile, or event data;
- an incorrect universe or sleeve scope;
- a strategy config that does not match the intended review cadence;
- a config edited after seeing output;
- momentum signals that detect price strength without explaining the catalyst;
- source-thin event context;
- Markov or raw Kelly fields inferred from limited samples;
- valuation, credit, financial, or article summaries that require primary
  source review;
- agent summaries that overstate what the JSON actually supports.

The correct response to model risk is not to add hidden judgment inside the
tool. Encode durable rules in explicit configs or private portfolio policy,
validate concrete plans, and journal the final human decision separately from
the first-pass model packet.

## Deterministic Output Is Not A Guarantee

`mpal` uses deterministic local rules for config hashing, planning constraints,
validation, decision-gate assembly, reports, and SQLite journaling. That makes
review packets more auditable, but it does not make the data complete or the
investment conclusion correct.

Deterministic means "reproducible from the same inputs and code path." It does
not mean profitable, suitable, compliant, or current.

## Compliance Boundary

Teams using `mpal-cli` in a professional setting should keep compliance checks
outside the tool unless and until those checks are implemented as explicit
validated product requirements. At minimum, review workflows should define:

- who may run reviews;
- who may approve final decisions;
- what disclosures are required;
- how client suitability is assessed, if relevant;
- where official records are retained;
- what source documents are required before an action is approved;
- how stale or missing data blocks decision support.

`mpal-cli` can help create an audit trail for a review packet, but it does not
replace regulated supervision, investment committee approval, or client-facing
advice controls.

## Chain-Of-Thought And Private Reasoning

The SQLite review journal is an accountable decision ledger, not a transcript
store. It records structured review facts, concise agent/human rationale, final
validation status, and per-ticker decisions. It should not store private
chain-of-thought, hidden reasoning traces, every intermediate prompt, every
tool call, or raw private policy contents.

Use `agent_summary`, `agent_reason`, `human_reasoning_text`, and
`human_reason` for concise decision rationale that can be reviewed later.

## Safe Review Pattern

The preferred professional pattern is:

1. Run an approved, versioned strategy config.
2. Inspect data freshness, event context, sizing constraints, and validation.
3. Treat model output as a first-pass review packet, not an instruction.
4. Validate any final human or agent override.
5. Journal the first-pass packet and the final human decision.
6. Keep private policy and broker execution outside repo-tracked files.

