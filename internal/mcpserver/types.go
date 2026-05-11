package mcpserver

type noInput struct{}

type strategyShowInput struct {
	ID string `json:"id" jsonschema:"Strategy ID from mpal_strategy_list, for example momentum_profile_v1."`
}

type strategyValidateInput struct {
	ConfigPath string `json:"config_path,omitempty" jsonschema:"Path to a strategy YAML/JSON config."`
	ConfigJSON string `json:"config_json,omitempty" jsonschema:"Inline strategy config as YAML or JSON. Use this only for explicit user-provided configs."`
}

type strategyRunInput struct {
	Date          string `json:"date" jsonschema:"As-of date in YYYY-MM-DD format."`
	UniversePath  string `json:"universe_path,omitempty" jsonschema:"Path to universe JSON, either {\"tickers\":[...]} or a ticker array."`
	UniverseJSON  string `json:"universe_json,omitempty" jsonschema:"Inline universe JSON, either {\"tickers\":[...]} or a ticker array."`
	PortfolioPath string `json:"portfolio_path,omitempty" jsonschema:"Path to portfolio JSON."`
	PortfolioJSON string `json:"portfolio_json,omitempty" jsonschema:"Inline portfolio JSON."`
	ConfigPath    string `json:"config_path,omitempty" jsonschema:"Path to explicit versioned strategy YAML/JSON config."`
	ConfigJSON    string `json:"config_json,omitempty" jsonschema:"Inline explicit versioned strategy config. Do not invent or silently modify configs."`
}

type tickerBarsInput struct {
	Ticker string `json:"ticker" jsonschema:"Ticker symbol."`
	Start  string `json:"start" jsonschema:"Start date in YYYY-MM-DD format."`
	End    string `json:"end" jsonschema:"End date in YYYY-MM-DD format."`
}

type tickerProfileInput struct {
	Ticker  string   `json:"ticker,omitempty" jsonschema:"Single ticker symbol."`
	Tickers []string `json:"tickers,omitempty" jsonschema:"Ticker symbols for one batched profile request."`
	Date    string   `json:"date" jsonschema:"As-of date in YYYY-MM-DD format."`
}

type tickerEventsInput struct {
	Tickers           []string `json:"tickers,omitempty" jsonschema:"Explicit tickers to research."`
	RunPath           string   `json:"run_path,omitempty" jsonschema:"Path to a previous mpal strategy run JSON output."`
	RunJSON           string   `json:"run_json,omitempty" jsonschema:"Inline previous mpal strategy run JSON output."`
	PortfolioPath     string   `json:"portfolio_path,omitempty" jsonschema:"Optional portfolio JSON path for run-scoped events."`
	PortfolioJSON     string   `json:"portfolio_json,omitempty" jsonschema:"Optional inline portfolio JSON for run-scoped events."`
	Scope             string   `json:"scope,omitempty" jsonschema:"Tracked ticker scope: portfolio or watchlist."`
	Days              int32    `json:"days,omitempty" jsonschema:"Lookback days. Defaults to 14."`
	Limit             int32    `json:"limit,omitempty" jsonschema:"Maximum source-backed updates. Defaults to 80."`
	Alternates        int32    `json:"alternates,omitempty" jsonschema:"Maximum alternate candidates for run-scoped events. Defaults to 5."`
	InsightsPerTicker int32    `json:"insights_per_ticker,omitempty" jsonschema:"Maximum cached article insights per ticker. Defaults to 2."`
}

type portfolioValidateInput struct {
	PlanPath      string `json:"plan_path,omitempty" jsonschema:"Path to portfolio plan JSON."`
	PlanJSON      string `json:"plan_json,omitempty" jsonschema:"Inline portfolio plan JSON."`
	PortfolioPath string `json:"portfolio_path,omitempty" jsonschema:"Path to portfolio JSON."`
	PortfolioJSON string `json:"portfolio_json,omitempty" jsonschema:"Inline portfolio JSON."`
	UniversePath  string `json:"universe_path,omitempty" jsonschema:"Optional universe JSON path. Defaults to tickers in plan."`
	UniverseJSON  string `json:"universe_json,omitempty" jsonschema:"Optional inline universe JSON. Defaults to tickers in plan."`
	ConfigPath    string `json:"config_path,omitempty" jsonschema:"Path to strategy YAML/JSON config."`
	ConfigJSON    string `json:"config_json,omitempty" jsonschema:"Inline strategy YAML/JSON config."`
}

type portfolioTransactionsInput struct {
	Page  int32 `json:"page,omitempty" jsonschema:"Transaction page. Defaults server-side."`
	Limit int32 `json:"limit,omitempty" jsonschema:"Maximum transactions to return. Defaults server-side and is capped by MarketPal."`
}

type backtestRunInput struct {
	Start          string `json:"start" jsonschema:"Start date in YYYY-MM-DD format."`
	End            string `json:"end" jsonschema:"End date in YYYY-MM-DD format."`
	UniversePath   string `json:"universe_path,omitempty" jsonschema:"Path to universe JSON."`
	UniverseJSON   string `json:"universe_json,omitempty" jsonschema:"Inline universe JSON."`
	ConfigPath     string `json:"config_path,omitempty" jsonschema:"Path to explicit versioned strategy YAML/JSON config."`
	ConfigJSON     string `json:"config_json,omitempty" jsonschema:"Inline explicit versioned strategy config."`
	Benchmark      string `json:"benchmark,omitempty" jsonschema:"Optional benchmark ticker."`
	AllowUntrusted bool   `json:"allow_untrusted,omitempty" jsonschema:"Return diagnostics even when trust checks fail."`
}

type decisionGateInput struct {
	RunPath              string `json:"run_path,omitempty" jsonschema:"Path to a previous mpal strategy run JSON output."`
	RunJSON              string `json:"run_json,omitempty" jsonschema:"Inline previous mpal strategy run JSON output."`
	ConfigPath           string `json:"config_path,omitempty" jsonschema:"Path to strategy YAML/JSON config. Required when include_markov_context is set."`
	ConfigJSON           string `json:"config_json,omitempty" jsonschema:"Inline strategy YAML/JSON config. Required when include_markov_context is set."`
	EventsPath           string `json:"events_path,omitempty" jsonschema:"Path to ticker events JSON context."`
	EventsJSON           string `json:"events_json,omitempty" jsonschema:"Inline ticker events JSON context."`
	Alternates           *int   `json:"alternates,omitempty" jsonschema:"Maximum alternate signal candidates. Defaults to 5. Use 0 to suppress alternates."`
	IncludeMarkovContext string `json:"include_markov_context,omitempty" jsonschema:"Comma-separated Markov context horizons: daily, weekly, monthly."`
}

type journalStartInput struct {
	InputPath string `json:"input_path,omitempty" jsonschema:"Path to trade review start JSON."`
	InputJSON string `json:"input_json,omitempty" jsonschema:"Inline trade review start JSON."`
}

type journalFinalizeInput struct {
	ID        string `json:"id" jsonschema:"Trade review ID to finalize."`
	InputPath string `json:"input_path,omitempty" jsonschema:"Path to trade review finalize JSON."`
	InputJSON string `json:"input_json,omitempty" jsonschema:"Inline trade review finalize JSON."`
}

type journalListInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Maximum entries to return. Defaults to 20."`
}

type journalGetInput struct {
	ID string `json:"id" jsonschema:"Journal entry ID."`
}
