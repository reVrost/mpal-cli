package demo

import _ "embed"

// These fixtures are embedded so `mpal demo` works from an installed binary,
// while the same files remain visible as examples in the repository.

//go:embed portfolio.json
var PortfolioJSON []byte

//go:embed universe.json
var UniverseJSON []byte

//go:embed strategy.yaml
var StrategyYAML []byte

//go:embed strategy_run.json
var StrategyRunJSON []byte

//go:embed ticker_events.json
var TickerEventsJSON []byte

//go:embed final_plan.json
var FinalPlanJSON []byte

//go:embed final_journal.json
var FinalJournalJSON []byte
