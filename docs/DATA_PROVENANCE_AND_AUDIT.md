# Data Provenance And Audit

`mpal-cli` is designed to produce review packets that a human can inspect,
re-run, validate, and journal. This document explains where data comes from,
how timestamps should be read, which fields are deterministic, and what is
stored locally.

## Source Layers

`mpal` review output combines three layers:

| Layer | Examples | Provenance |
| --- | --- | --- |
| User-supplied inputs | portfolio JSON, universe JSON, strategy config, final action JSON | Provided on the CLI or through MCP tool input |
| Marketpal API data | portfolio snapshots, transactions, watchlists, ticker profile, bars, events, financials, fundamentals, insiders, ownership, hosted strategy runs | Fetched with `MPAL_API_KEY` from Marketpal API-backed commands |
| Local deterministic logic | config expansion, config hash, local planning for supported configs, validation, decision gate assembly, reports, SQLite journaling | Computed by this repo from explicit inputs and API payloads |

The review packet is only as good as those inputs. If the portfolio file,
universe, strategy config, or API payload is stale or incorrectly scoped, the
output should be treated as incomplete review evidence.

## Timestamp Fields

Common timestamp fields have different meanings:

| Field | Meaning |
| --- | --- |
| `as_of` | The review date or data-effective date requested for a run or record. It is not always the fetch time. |
| `created_at` | Local journal insertion time for a durable review record. |
| `fetched_at` | Time a payload was fetched from the API or data provider. |
| `updated_at` | Time Marketpal says the persisted upstream record was last updated. |
| `quote_time` | Timestamp attached to a quote or profile price. |
| `event_date` | Time attached to a source-backed event, filing, release, or market action. |
| `execution_date` | Human-entered execution date for a finalized review position, when known. |

Reviewers should not collapse these into one concept. A strategy run may be
`as_of` a market date while the data was fetched later, and a journal row may
be created after the actual reviewed event.

## Freshness And Stale Data

Marketpal payloads can include a `freshness` object with:

- `source`: the dataset or loader name.
- `provider`: upstream provider, when available.
- `storage`: storage/backend label, when available.
- `as_of`, `fetched_at`, `updated_at`: provenance timestamps.
- `stale`: explicit stale-data flag.
- `warning` or `description`: human-readable context.

Review packets should treat `stale: true`, missing price/profile data, stale
portfolio snapshots, and source-thin event context as review warnings. They do
not automatically mean `TRADE` or `NO_TRADE`; they mean the evidence set may
not be strong enough for decision support.

Backtests are stricter than normal research packets: the local backtest code
blocks stale or untrusted price sources instead of treating them as mere
annotation.

## Config Hashes

Strategy configs are expanded before execution. Built-in defaults such as
`defaults: swing_v1` are applied first, then `config_hash` is computed from
canonical expanded strategy JSON.

The hash algorithm is:

```text
sha256:canonical-expanded-strategy-json-v1
```

Important consequences:

- equivalent slim and fully expanded configs hash the same;
- raw YAML formatting, comments, and key order are not part of the hash;
- the hash identifies the reviewed strategy contract, not the whole review
  packet;
- `strategy_id`, `version`, `approved`, `config_hash`, and
  `config_hash_algorithm` should be kept with review artifacts.

Agents must not silently edit configs after inspecting output. Any changed
parameter creates a different reviewed strategy and should be validated and
journaled as a separate review.

## Deterministic, Model-Derived, API-Derived, And Agent-Derived Fields

Use this distinction when reading JSON:

| Field Type | Examples | How To Treat It |
| --- | --- | --- |
| Deterministic local fields | config hash, validation result, risk clamps, `binding_constraint`, local report path, SQLite IDs, journal schema checks | Reproducible from the same code version and same inputs |
| Marketpal/API-derived fields | prices, portfolio snapshots, watchlist, bars, profile/QVM scores, Markov reads, raw Kelly evidence, events, financials, fundamentals, hosted strategy output | Source-backed or provider-derived; inspect `freshness`, timestamps, and warnings |
| Strategy model-derived fields | `model_result`, `signals`, `final_score`, `action_hint`, `baseline_plan`, `proposed_trades`, `rejected` | Model/scoring output for the supplied run; not trade instructions |
| Agent or human fields | `agent_summary`, `agent_decision`, `agent_reason`, `final_decision`, `human_reasoning_text`, `human_decision` | Review overlay and accountable decision record; not deterministic model output |

`mpal strategy run` is the source of truth for model output. Agent prose can
explain, veto, delay, or propose a validated override, but it should not invent
model actions that are absent from `mpal` JSON.

## Audit Trail

A normal auditable review has these artifacts:

1. Strategy config used for the run, including `strategy_id`, `version`, and
   `config_hash`.
2. Portfolio and universe inputs used for the run.
3. `mpal strategy run` output and returned `journal_entry_id`.
4. `mpal ticker events` or other source-backed context used during review.
5. `mpal decision gate` output when an agent is reviewing the packet.
6. `mpal portfolio validate` output for any final concrete plan or override.
7. `mpal report <journal_entry_id>` deterministic local HTML report path.
8. `mpal journal finalize` record with the final human decision.

The SQLite journal is the durable decision ledger. Temporary JSON files,
source packs, ticker-event calls, and exploratory DD can support a review, but
they are not each journaled as separate accountable decisions.

## Journal Schema

The durable journal schema is documented in
[REVIEW_JOURNAL_SQLITE.md](REVIEW_JOURNAL_SQLITE.md) and implemented in:

```text
pkg/mpal/sqlitejournal/schema.sql
```

The schema intentionally separates:

- `trade_reviews`: one row per reviewed packet, including `as_of`,
  `created_at`, strategy metadata, portfolio scope, universe, execution result,
  agent summary fields, final human decision, validation summary, report path,
  and warnings.
- `trade_review_positions`: one row per ticker inside a reviewed packet,
  including model bucket/intent/score/weight, deterministic sizing fields,
  agent decision fields, and final human decision/execution fields.

The schema is a decision ledger, not a cache schema. Raw API payloads, source
packs, private portfolio-policy contents, and exploratory commands should stay
outside the durable review rows unless they are summarized into the accountable
review decision.

## Local Storage Versus API Fetches

Typical local paths:

| Path | Purpose |
| --- | --- |
| `~/.marketpal/mpal.db` | Durable SQLite review journal. |
| `~/.marketpal/journal.jsonl` | Legacy/general JSONL journal; review decisions use SQLite. |
| `~/.marketpal/portfolio-policy.md` | Private user policy context, outside repo tracking. |
| `~/.marketpal/reports/` | Deterministic local HTML report output, when generated. |
| `tmp/` or `tmp/mpal-runs/` | Temporary local artifacts created by a review workflow. |

Typical API-backed commands:

- `mpal portfolio snapshot`
- `mpal portfolio transactions`
- `mpal watchlist get`
- `mpal ticker profile`
- `mpal ticker events`
- `mpal ticker financials`
- `mpal ticker fundamentals`
- `mpal ticker insiders`
- `mpal ticker ownership`
- `mpal strategy run` for hosted-compatible configs
- `mpal backtest run` for hosted-compatible backtests

Typical local deterministic commands:

- `mpal strategy list`
- `mpal strategy show`
- `mpal strategy validate`
- local fallback strategy runs for local-only scoring contracts
- `mpal portfolio validate`
- `mpal decision gate`
- `mpal report`
- `mpal journal start`
- `mpal journal finalize`
- `mpal journal list`
- `mpal journal get`

Some commands combine layers. For example, `mpal strategy run` may fetch
hosted strategy output and then auto-journal the deterministic first-pass
review locally.

## Journal Scope

The SQLite journal records accountable review decisions. It is not a full
trace log and is not intended to store every intermediate command, prompt,
source payload, private portfolio-policy contents, or private chain-of-thought.

The journal may store concise prompt/chat context, agent summary, agent reason,
and human reasoning when supplied. Those fields should be decision rationale,
not hidden model reasoning transcripts.
