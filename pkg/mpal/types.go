package mpal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ResultTrade   = "TRADE"
	ResultNoTrade = "NO_TRADE"

	SideBuy  = "BUY"
	SideSell = "SELL"
	SideHold = "HOLD"

	TradeIntentStarter       = "STARTER"
	TradeIntentTopUp         = "TOP_UP"
	TradeIntentTrim          = "TRIM"
	TradeIntentReduce        = "REDUCE"
	TradeIntentExitCandidate = "EXIT_CANDIDATE"
)

type Freshness struct {
	Source      string     `json:"source"`
	Provider    string     `json:"provider,omitempty"`
	Storage     string     `json:"storage,omitempty"`
	AsOf        *time.Time `json:"as_of,omitempty"`
	FetchedAt   *time.Time `json:"fetched_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	Stale       bool       `json:"stale"`
	Warning     string     `json:"warning,omitempty"`
	Description string     `json:"description,omitempty"`
}

type Bar struct {
	Date          time.Time `json:"date"`
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Close         float64   `json:"close"`
	AdjustedClose *float64  `json:"adjusted_close,omitempty"`
	Volume        float64   `json:"volume"`
}

type Position struct {
	Ticker       string  `json:"ticker"`
	Shares       float64 `json:"shares,omitempty"`
	MarketValue  float64 `json:"market_value"`
	Weight       float64 `json:"weight"`
	CurrentPrice float64 `json:"current_price,omitempty"`
}

type Portfolio struct {
	UserID    string     `json:"user_id,omitempty"`
	Cash      float64    `json:"cash"`
	Equity    float64    `json:"equity"`
	Positions []Position `json:"positions"`
	Freshness *Freshness `json:"freshness,omitempty"`
	Warnings  []string   `json:"warnings,omitempty"`
}

type Universe struct {
	Tickers []string `json:"tickers" yaml:"tickers"`
}

type StrategyConfig struct {
	Schema      string               `json:"-" yaml:"$schema,omitempty"`
	ID          string               `json:"id" yaml:"id"`
	Version     string               `json:"version" yaml:"version"`
	Defaults    string               `json:"defaults,omitempty" yaml:"defaults"`
	Name        string               `json:"name,omitempty" yaml:"name"`
	Description string               `json:"description,omitempty" yaml:"description"`
	WhenToUse   string               `json:"when_to_use,omitempty" yaml:"when_to_use"`
	Cadence     string               `json:"cadence,omitempty" yaml:"cadence"`
	Approved    bool                 `json:"approved" yaml:"approved"`
	Scoring     ScoringConfig        `json:"scoring" yaml:"scoring"`
	Events      EventGuardrailConfig `json:"event_guardrail" yaml:"event_guardrail"`
	Portfolio   PortfolioConfig      `json:"portfolio" yaml:"portfolio"`
	Risk        RiskConfig           `json:"risk" yaml:"risk"`
	Backtest    BacktestConfig       `json:"backtest" yaml:"backtest"`
}

type ScoringConfig struct {
	MomentumWeight  float64 `json:"momentum_weight" yaml:"momentum_weight"`
	ProfileWeight   float64 `json:"profile_weight" yaml:"profile_weight"`
	QualityWeight   float64 `json:"quality_weight,omitempty" yaml:"quality_weight,omitempty"`
	ValueWeight     float64 `json:"value_weight,omitempty" yaml:"value_weight,omitempty"`
	ReversionWeight float64 `json:"reversion_weight,omitempty" yaml:"reversion_weight,omitempty"`
	MinBuyScore     float64 `json:"min_buy_score" yaml:"min_buy_score"`
	MinHoldScore    float64 `json:"min_hold_score" yaml:"min_hold_score"`
}

type EventGuardrailConfig struct {
	Enabled      bool    `json:"enabled" yaml:"enabled"`
	LookbackDays int     `json:"lookback_days" yaml:"lookback_days"`
	VetoScore    float64 `json:"event_veto_score" yaml:"event_veto_score"`
	BoostScore   float64 `json:"event_boost_score" yaml:"event_boost_score"`
	BoostAmount  float64 `json:"event_boost_amount" yaml:"event_boost_amount"`
}

type PortfolioConfig struct {
	LongOnly          bool    `json:"long_only" yaml:"long_only"`
	MaxPositions      int     `json:"max_positions" yaml:"max_positions"`
	MaxPositionPct    float64 `json:"max_position_pct" yaml:"max_position_pct"`
	MinTradeValue     float64 `json:"min_trade_value" yaml:"min_trade_value"`
	Rebalance         string  `json:"rebalance" yaml:"rebalance"`
	ListingRegionTilt string  `json:"listing_region_tilt,omitempty" yaml:"listing_region_tilt"`
}

type RiskConfig struct {
	Profile                 string   `json:"profile,omitempty" yaml:"profile,omitempty"`
	TurnoverBudgetPct       float64  `json:"turnover_budget_pct" yaml:"turnover_budget_pct"`
	MaxSingleTradePct       float64  `json:"max_single_trade_pct" yaml:"max_single_trade_pct"`
	StarterPositionPct      float64  `json:"starter_position_pct" yaml:"starter_position_pct"`
	MaxNewPositionsPerRun   int      `json:"max_new_positions_per_run" yaml:"max_new_positions_per_run"`
	CashBufferPct           float64  `json:"cash_buffer_pct" yaml:"cash_buffer_pct"`
	ProtectUnscoredHoldings bool     `json:"protect_unscored_holdings" yaml:"protect_unscored_holdings"`
	SizingMethod            string   `json:"sizing_method,omitempty" yaml:"sizing_method,omitempty"`
	KellyFraction           *float64 `json:"kelly_fraction,omitempty" yaml:"kelly_fraction,omitempty"`
	KellyMinEdge            *float64 `json:"kelly_min_edge,omitempty" yaml:"kelly_min_edge,omitempty"`
	KellyMaxFraction        *float64 `json:"kelly_max_fraction,omitempty" yaml:"kelly_max_fraction,omitempty"`
	KellyDefaultPayoffRatio *float64 `json:"kelly_default_payoff_ratio,omitempty" yaml:"kelly_default_payoff_ratio,omitempty"`
	KellyMinConfidence      *float64 `json:"kelly_min_confidence,omitempty" yaml:"kelly_min_confidence,omitempty"`
	KellyMinSampleCount     *int     `json:"kelly_min_sample_count,omitempty" yaml:"kelly_min_sample_count,omitempty"`
	KellyMissingEdgePolicy  string   `json:"kelly_missing_edge_policy,omitempty" yaml:"kelly_missing_edge_policy,omitempty"`
}

type BacktestConfig struct {
	InitialCash float64 `json:"initial_cash" yaml:"initial_cash"`
	FeeBps      float64 `json:"fee_bps" yaml:"fee_bps"`
	SlippageBps float64 `json:"slippage_bps" yaml:"slippage_bps"`
}

const (
	StrategyDefaultsSwingV1 = "swing_v1"
	StrategyDefaultsBasicV1 = "basic_v1"

	StrategyConfigHashAlgorithm = "sha256:canonical-expanded-strategy-json-v1"
	HostedStrategyAPIContract   = "hosted_strategy_api_v1"
	ScoringContractV1           = "scoring_v1_momentum_profile"
	ScoringContractV2           = "scoring_v2_quality_value_reversion"
)

type StrategyRef struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	ConfigHash string `json:"config_hash"`
	Approved   bool   `json:"approved"`
	Source     string `json:"source,omitempty"`
	Path       string `json:"path,omitempty"`
}

type ProfileScore struct {
	Ticker        string     `json:"ticker"`
	AsOf          time.Time  `json:"as_of"`
	ProfileScore  float64    `json:"profile_score"`
	MomentumScore *float64   `json:"momentum_score,omitempty"`
	QualityScore  *float64   `json:"quality_score,omitempty"`
	ValueScore    *float64   `json:"value_score,omitempty"`
	ScoreSource   string     `json:"score_source"`
	Reasons       []string   `json:"reasons,omitempty"`
	Warnings      []string   `json:"warnings,omitempty"`
	Freshness     *Freshness `json:"freshness,omitempty"`
}

type BarsResult struct {
	Ticker    string     `json:"ticker"`
	Start     time.Time  `json:"start"`
	End       time.Time  `json:"end"`
	Bars      []Bar      `json:"bars"`
	Warnings  []string   `json:"warnings,omitempty"`
	Freshness *Freshness `json:"freshness,omitempty"`
}

type FactorSnapshot struct {
	Ticker                 string     `json:"ticker"`
	YahooTicker            string     `json:"yahoo_ticker"`
	Market                 string     `json:"market"`
	Exchange               string     `json:"exchange"`
	SnapshotDate           time.Time  `json:"snapshot_date"`
	ProfileUpdatedAt       *time.Time `json:"profile_updated_at,omitempty"`
	ProfileVersion         string     `json:"profile_version"`
	SourceHash             string     `json:"source_hash"`
	Sector                 string     `json:"sector,omitempty"`
	Industry               string     `json:"industry,omitempty"`
	Currency               string     `json:"currency,omitempty"`
	QVMScore               *float64   `json:"qvm_score,omitempty"`
	QVMQualityScore        *float64   `json:"qvm_quality_score,omitempty"`
	QVMValueScore          *float64   `json:"qvm_value_score,omitempty"`
	QVMMomentumScore       *float64   `json:"qvm_momentum_score,omitempty"`
	QVMClassification      string     `json:"qvm_classification,omitempty"`
	QVMIsEligible          bool       `json:"qvm_is_eligible"`
	Price                  *float64   `json:"price,omitempty"`
	MarketCap              *float64   `json:"market_cap,omitempty"`
	PE                     *float64   `json:"pe,omitempty"`
	PS                     *float64   `json:"ps,omitempty"`
	EVToFCF                *float64   `json:"ev_to_fcf,omitempty"`
	EVToEBIT               *float64   `json:"ev_to_ebit,omitempty"`
	FCFYield               *float64   `json:"fcf_yield,omitempty"`
	ROIC                   *float64   `json:"roic,omitempty"`
	ROE                    *float64   `json:"roe,omitempty"`
	QVMMetrics             []byte     `json:"qvm_metrics,omitempty"`
	QVMComponentScores     []byte     `json:"qvm_component_scores,omitempty"`
	QVMFlags               []byte     `json:"qvm_flags,omitempty"`
	PriceTechnicalSnapshot []byte     `json:"price_technical_snapshot,omitempty"`
}

type FactorSnapshotCoverage struct {
	YahooTicker        string    `json:"yahoo_ticker"`
	FirstSnapshotDate  time.Time `json:"first_snapshot_date"`
	LatestSnapshotDate time.Time `json:"latest_snapshot_date"`
	SnapshotCount      int64     `json:"snapshot_count"`
}

type EventScore struct {
	Ticker      string    `json:"ticker"`
	SourceURL   string    `json:"source_url,omitempty"`
	Title       string    `json:"title,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	Score       float64   `json:"score"`
	Confidence  *float64  `json:"confidence,omitempty"`
	Version     string    `json:"version,omitempty"`
	Model       string    `json:"model,omitempty"`
	ScoredAt    time.Time `json:"scored_at"`
}

type SignalResult struct {
	Ticker               string      `json:"ticker"`
	AsOf                 time.Time   `json:"as_of"`
	MomentumScore        float64     `json:"momentum_score"`
	ProfileScore         float64     `json:"profile_score"`
	QualityScore         *float64    `json:"quality_score,omitempty"`
	ValueScore           *float64    `json:"value_score,omitempty"`
	ReversionScore       *float64    `json:"reversion_score,omitempty"`
	EventScore           *float64    `json:"event_score,omitempty"`
	EventScoreConfidence *float64    `json:"event_score_confidence,omitempty"`
	Markov               *MarkovRead `json:"markov,omitempty"`
	FinalScore           float64     `json:"final_score"`
	ActionHint           string      `json:"action_hint"`
	EventVeto            bool        `json:"event_veto,omitempty"`
	Reasons              []string    `json:"reasons,omitempty"`
	Warnings             []string    `json:"warnings,omitempty"`
	Freshness            []Freshness `json:"freshness,omitempty"`
}

type MarkovRead struct {
	Model                   string             `json:"model"`
	Horizon                 string             `json:"horizon"`
	HorizonBars             int                `json:"horizon_bars"`
	CurrentState            string             `json:"current_state"`
	CurrentReturn           float64            `json:"current_return"`
	TransitionProbabilities map[string]float64 `json:"transition_probabilities"`
	FavorableProbability    float64            `json:"favorable_probability"`
	UnfavorableProbability  float64            `json:"unfavorable_probability"`
	ExpectedStateScore      float64            `json:"expected_state_score"`
	SampleCount             int                `json:"sample_count"`
	TotalTransitionCount    int                `json:"total_transition_count"`
	Confidence              float64            `json:"confidence"`
	Warnings                []string           `json:"warnings,omitempty"`
}

type TickerMarkovResult struct {
	RunID        string             `json:"run_id"`
	Mode         string             `json:"mode"`
	AsOf         time.Time          `json:"as_of"`
	Rebalance    string             `json:"rebalance"`
	Horizon      string             `json:"horizon"`
	HorizonBars  int                `json:"horizon_bars"`
	LookbackDays int                `json:"lookback_days"`
	Results      []TickerMarkovItem `json:"results"`
	Warnings     []string           `json:"warnings,omitempty"`
}

type TickerMarkovItem struct {
	Ticker    string      `json:"ticker"`
	BarCount  int         `json:"bar_count"`
	Markov    *MarkovRead `json:"markov,omitempty"`
	Freshness *Freshness  `json:"freshness,omitempty"`
	Warnings  []string    `json:"warnings,omitempty"`
}

type TargetPosition struct {
	Ticker       string  `json:"ticker"`
	TargetWeight float64 `json:"target_weight"`
	Reason       string  `json:"reason"`
}

type ProposedTrade struct {
	Ticker         string          `json:"ticker"`
	Side           string          `json:"side"`
	Intent         string          `json:"intent,omitempty"`
	CurrentWeight  float64         `json:"current_weight"`
	TargetWeight   float64         `json:"target_weight"`
	DeltaWeight    float64         `json:"delta_weight"`
	EstimatedValue float64         `json:"estimated_value"`
	Reason         string          `json:"reason"`
	Sizing         *SizingDecision `json:"sizing,omitempty"`
}

type SizingDecision struct {
	Method          string   `json:"method"`
	Source          string   `json:"source,omitempty"`
	RawKelly        float64  `json:"raw_kelly,omitempty"`
	FractionalKelly float64  `json:"fractional_kelly,omitempty"`
	TargetWeight    float64  `json:"target_weight,omitempty"`
	PayoffRatio     float64  `json:"payoff_ratio,omitempty"`
	Confidence      float64  `json:"confidence,omitempty"`
	SampleCount     int      `json:"sample_count,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

type RejectedTicker struct {
	Ticker string `json:"ticker"`
	Reason string `json:"reason"`
}

type PortfolioPlanResult struct {
	AsOf           time.Time        `json:"as_of"`
	Result         string           `json:"result"`
	Targets        []TargetPosition `json:"targets"`
	ProposedTrades []ProposedTrade  `json:"proposed_trades"`
	Rejected       []RejectedTicker `json:"rejected,omitempty"`
	Summary        string           `json:"summary"`
	Warnings       []string         `json:"warnings,omitempty"`
}

type StrategyRunResult struct {
	RunID           string              `json:"run_id"`
	Mode            string              `json:"mode"`
	AsOf            time.Time           `json:"as_of"`
	Strategy        StrategyRef         `json:"strategy"`
	Result          string              `json:"result"`
	ModelResult     string              `json:"model_result"`
	ExecutionResult string              `json:"execution_result"`
	Summary         string              `json:"summary"`
	Signals         []SignalResult      `json:"signals"`
	BaselinePlan    PortfolioPlanResult `json:"baseline_plan"`
	Validation      ValidationResult    `json:"validation"`
	JournalEntryID  string              `json:"journal_entry_id,omitempty"`
	Warnings        []string            `json:"warnings,omitempty"`
}

type BacktestOptions struct {
	TrustedOnly           bool   `json:"trusted_only"`
	AllowUntrusted        bool   `json:"allow_untrusted"`
	Benchmark             string `json:"benchmark,omitempty"`
	ProfileVersion        string `json:"profile_version,omitempty"`
	SnapshotFreshnessDays int    `json:"snapshot_freshness_days,omitempty"`
}

type BacktestResult struct {
	RunID          string              `json:"run_id"`
	Mode           string              `json:"mode"`
	Start          time.Time           `json:"start"`
	End            time.Time           `json:"end"`
	Strategy       StrategyRef         `json:"strategy"`
	Trusted        bool                `json:"trusted"`
	TrustStatus    string              `json:"trust_status"`
	TrustReasons   []string            `json:"trust_reasons,omitempty"`
	Metrics        BacktestMetrics     `json:"metrics"`
	EquityCurve    []EquityPoint       `json:"equity_curve"`
	Trades         []BacktestTrade     `json:"trades"`
	Rebalances     []BacktestRebalance `json:"rebalances"`
	FinalPortfolio Portfolio           `json:"final_portfolio"`
	DataQuality    DataQualityReport   `json:"data_quality"`
	Benchmark      *BenchmarkResult    `json:"benchmark,omitempty"`
	JournalEntryID string              `json:"journal_entry_id,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
}

type BacktestMetrics struct {
	InitialEquity        float64 `json:"initial_equity"`
	FinalEquity          float64 `json:"final_equity"`
	TotalReturn          float64 `json:"total_return"`
	CAGR                 float64 `json:"cagr"`
	AnnualizedVolatility float64 `json:"annualized_volatility"`
	Sharpe               float64 `json:"sharpe"`
	Sortino              float64 `json:"sortino"`
	MaxDrawdown          float64 `json:"max_drawdown"`
	Calmar               float64 `json:"calmar"`
	CashDrag             float64 `json:"cash_drag"`
	TradeCount           int     `json:"trade_count"`
	RebalanceCount       int     `json:"rebalance_count"`
	AverageTurnover      float64 `json:"average_turnover"`
}

type EquityPoint struct {
	Date      time.Time `json:"date"`
	Equity    float64   `json:"equity"`
	Cash      float64   `json:"cash"`
	Drawdown  float64   `json:"drawdown"`
	Exposure  float64   `json:"exposure"`
	Positions int       `json:"positions"`
}

type BacktestTrade struct {
	Date           time.Time `json:"date"`
	SignalDate     time.Time `json:"signal_date"`
	Ticker         string    `json:"ticker"`
	Side           string    `json:"side"`
	Shares         float64   `json:"shares"`
	Price          float64   `json:"price"`
	GrossValue     float64   `json:"gross_value"`
	Fee            float64   `json:"fee"`
	SlippageBps    float64   `json:"slippage_bps"`
	CashAfterTrade float64   `json:"cash_after_trade"`
	Reason         string    `json:"reason"`
}

type BacktestRebalance struct {
	Date       time.Time        `json:"date"`
	FillDate   time.Time        `json:"fill_date"`
	Result     string           `json:"result"`
	Targets    []TargetPosition `json:"targets,omitempty"`
	Trades     []BacktestTrade  `json:"trades,omitempty"`
	Rejected   []RejectedTicker `json:"rejected,omitempty"`
	Turnover   float64          `json:"turnover"`
	Warnings   []string         `json:"warnings,omitempty"`
	DataSource string           `json:"data_source,omitempty"`
}

type DataQualityReport struct {
	Trusted       bool                     `json:"trusted"`
	BarSource     string                   `json:"bar_source,omitempty"`
	ProfileSource string                   `json:"profile_source,omitempty"`
	EventSource   string                   `json:"event_source,omitempty"`
	Tickers       []TickerDataQuality      `json:"tickers,omitempty"`
	Blockers      []string                 `json:"blockers,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	Coverage      []FactorSnapshotCoverage `json:"coverage,omitempty"`
}

type TickerDataQuality struct {
	Ticker       string     `json:"ticker"`
	BarCount     int        `json:"bar_count"`
	FirstBarDate *time.Time `json:"first_bar_date,omitempty"`
	LastBarDate  *time.Time `json:"last_bar_date,omitempty"`
	Warnings     []string   `json:"warnings,omitempty"`
	Blockers     []string   `json:"blockers,omitempty"`
}

type BenchmarkResult struct {
	Ticker       string  `json:"ticker"`
	TotalReturn  float64 `json:"total_return"`
	ExcessReturn float64 `json:"excess_return"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func LoadStrategyFile(path string) (StrategyConfig, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return StrategyConfig{}, "", err
	}
	return LoadStrategyBytes(raw)
}

func LoadStrategyBytes(raw []byte) (StrategyConfig, string, error) {
	var cfg StrategyConfig
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return StrategyConfig{}, "", err
	}
	cfg = ApplyStrategyDefaults(cfg)
	return cfg, HashStrategyConfig(cfg), nil
}

func CanonicalStrategyConfig(cfg StrategyConfig) StrategyConfig {
	cfg = ApplyStrategyDefaults(cfg)
	cfg.Schema = ""
	cfg.Defaults = ""
	cfg.Risk.Profile = ""
	return cfg
}

func HashStrategyConfig(cfg StrategyConfig) string {
	raw, err := json.Marshal(CanonicalStrategyConfig(cfg))
	if err != nil {
		return ""
	}
	return HashBytes(raw)
}

func ApplyStrategyDefaults(cfg StrategyConfig) StrategyConfig {
	switch strings.TrimSpace(cfg.Defaults) {
	case StrategyDefaultsSwingV1:
		applySharedStrategyDefaults(&cfg)
		cfg.Events = EventGuardrailConfig{
			Enabled:      true,
			LookbackDays: 14,
			VetoScore:    -0.55,
			BoostScore:   0.70,
			BoostAmount:  0.03,
		}
	case StrategyDefaultsBasicV1:
		applySharedStrategyDefaults(&cfg)
		cfg.Events = EventGuardrailConfig{}
	}
	applyRiskProfileDefaults(&cfg)
	return cfg
}

func applySharedStrategyDefaults(cfg *StrategyConfig) {
	cfg.Portfolio.LongOnly = true
	cfg.Risk.ProtectUnscoredHoldings = true
	cfg.Backtest = BacktestConfig{
		InitialCash: 100000,
		FeeBps:      5,
		SlippageBps: 10,
	}
}

func ValidateStrategyConfig(cfg StrategyConfig) ValidationResult {
	var errs []string
	cfg = applyRiskProfileDefaultsCopy(cfg)
	if strings.TrimSpace(cfg.ID) == "" {
		errs = append(errs, "id is required")
	}
	if strings.TrimSpace(cfg.Version) == "" {
		errs = append(errs, "version is required")
	}
	switch strings.TrimSpace(cfg.Defaults) {
	case "", StrategyDefaultsSwingV1, StrategyDefaultsBasicV1:
	default:
		errs = append(errs, "defaults must be empty, swing_v1, or basic_v1")
	}
	if cfg.Scoring.MomentumWeight < 0 ||
		cfg.Scoring.ProfileWeight < 0 ||
		cfg.Scoring.QualityWeight < 0 ||
		cfg.Scoring.ValueWeight < 0 ||
		cfg.Scoring.ReversionWeight < 0 {
		errs = append(errs, "scoring weights must be non-negative")
	}
	if math.Abs(scoringWeightTotal(cfg.Scoring)-1) > 0.000001 {
		errs = append(errs, "scoring weights must sum to 1")
	}
	if cfg.Scoring.MinBuyScore < cfg.Scoring.MinHoldScore {
		errs = append(errs, "min_buy_score must be >= min_hold_score")
	}
	if cfg.Events.Enabled {
		events := normalizedEventGuardrails(cfg)
		if events.LookbackDays <= 0 {
			errs = append(errs, "event_guardrail.lookback_days must be > 0")
		}
		if events.VetoScore < -1 || events.VetoScore > 1 {
			errs = append(errs, "event_guardrail.event_veto_score must be in [-1,1]")
		}
		if events.BoostScore < -1 || events.BoostScore > 1 {
			errs = append(errs, "event_guardrail.event_boost_score must be in [-1,1]")
		}
		if events.BoostAmount < 0 || events.BoostAmount > 1 {
			errs = append(errs, "event_guardrail.event_boost_amount must be in [0,1]")
		}
	}
	if cfg.Portfolio.MaxPositions <= 0 {
		errs = append(errs, "portfolio.max_positions must be > 0")
	}
	if cfg.Portfolio.MaxPositionPct <= 0 || cfg.Portfolio.MaxPositionPct > 1 {
		errs = append(errs, "portfolio.max_position_pct must be in (0,1]")
	}
	if cfg.Portfolio.MinTradeValue < 0 {
		errs = append(errs, "portfolio.min_trade_value must be >= 0")
	}
	if region := normalizeListingRegion(cfg.Portfolio.ListingRegionTilt); region != "" && region != listingRegionUS && region != listingRegionASX {
		errs = append(errs, "portfolio.listing_region_tilt must be empty, US, or ASX")
	}
	if cfg.Risk.TurnoverBudgetPct < 0 || cfg.Risk.TurnoverBudgetPct > 1 {
		errs = append(errs, "risk.turnover_budget_pct must be in [0,1]")
	}
	if cfg.Risk.MaxSingleTradePct <= 0 || cfg.Risk.MaxSingleTradePct > 1 {
		errs = append(errs, "risk.max_single_trade_pct must be in (0,1]")
	}
	if cfg.Risk.StarterPositionPct <= 0 || cfg.Risk.StarterPositionPct > 1 {
		errs = append(errs, "risk.starter_position_pct must be in (0,1]")
	}
	if cfg.Risk.StarterPositionPct > cfg.Risk.MaxSingleTradePct {
		errs = append(errs, "risk.starter_position_pct must be <= risk.max_single_trade_pct")
	}
	if cfg.Risk.MaxNewPositionsPerRun < 0 {
		errs = append(errs, "risk.max_new_positions_per_run must be >= 0")
	}
	if cfg.Risk.CashBufferPct < 0 || cfg.Risk.CashBufferPct >= 1 {
		errs = append(errs, "risk.cash_buffer_pct must be in [0,1)")
	}
	switch profile := normalizeRiskProfile(cfg.Risk.Profile); profile {
	case "", RiskProfileBasic, RiskProfileLowChurn, RiskProfileWeeklySwing, RiskProfileRebuild:
	default:
		errs = append(errs, "risk.profile must be empty, basic, low_churn, weekly_swing, or rebuild")
	}
	sizing := normalizeSizingConfig(cfg.Risk)
	switch method := normalizeSizingMethod(cfg.Risk.SizingMethod); method {
	case "", SizingMethodFixed, SizingMethodFractionalKelly:
	default:
		errs = append(errs, "risk.sizing_method must be empty, fixed, or fractional_kelly")
	}
	switch policy := normalizeKellyMissingEdgePolicy(cfg.Risk.KellyMissingEdgePolicy); policy {
	case "", KellyMissingEdgePolicyFixed, KellyMissingEdgePolicySkip:
	default:
		errs = append(errs, "risk.kelly_missing_edge_policy must be fixed or skip")
	}
	if cfg.Risk.KellyFraction != nil || sizing.Method == SizingMethodFractionalKelly {
		if sizing.KellyFraction <= 0 || sizing.KellyFraction > 1 {
			errs = append(errs, "risk.kelly_fraction must be in (0,1]")
		}
	}
	if cfg.Risk.KellyMinEdge != nil && sizing.KellyMinEdge < 0 {
		errs = append(errs, "risk.kelly_min_edge must be >= 0")
	}
	if cfg.Risk.KellyMaxFraction != nil || sizing.Method == SizingMethodFractionalKelly {
		if sizing.KellyMaxFraction <= 0 || sizing.KellyMaxFraction > 1 {
			errs = append(errs, "risk.kelly_max_fraction must be in (0,1]")
		}
	}
	if cfg.Risk.KellyDefaultPayoffRatio != nil || sizing.Method == SizingMethodFractionalKelly {
		if sizing.KellyDefaultPayoffRatio <= 0 {
			errs = append(errs, "risk.kelly_default_payoff_ratio must be > 0")
		}
	}
	if cfg.Risk.KellyMinConfidence != nil || sizing.Method == SizingMethodFractionalKelly {
		if sizing.KellyMinConfidence < 0 || sizing.KellyMinConfidence > 1 {
			errs = append(errs, "risk.kelly_min_confidence must be in [0,1]")
		}
	}
	if cfg.Risk.KellyMinSampleCount != nil || sizing.Method == SizingMethodFractionalKelly {
		if sizing.KellyMinSampleCount < 0 {
			errs = append(errs, "risk.kelly_min_sample_count must be >= 0")
		}
	}
	return ValidationResult{Valid: len(errs) == 0, Errors: errs}
}

func ValidateHostedStrategyAPICompatibility(cfg StrategyConfig) ValidationResult {
	var errs []string
	if validation := ValidateStrategyConfig(cfg); !validation.Valid {
		errs = append(errs, "local strategy config is invalid: "+strings.Join(validation.Errors, "; "))
	}
	if usesAdvancedScoring(cfg.Scoring) {
		errs = append(errs, HostedStrategyAPIContract+" supports momentum_weight and profile_weight only; quality_weight, value_weight, and reversion_weight require a hosted API update")
	}
	return ValidationResult{Valid: len(errs) == 0, Errors: errs}
}

func EnsureHostedStrategyAPICompatible(cfg StrategyConfig) error {
	if compatibility := ValidateHostedStrategyAPICompatibility(cfg); !compatibility.Valid {
		return fmt.Errorf("strategy config is not compatible with %s: %s", HostedStrategyAPIContract, strings.Join(compatibility.Errors, "; "))
	}
	return nil
}

func StrategyScoringContract(cfg StrategyConfig) string {
	if usesAdvancedScoring(cfg.Scoring) {
		return ScoringContractV2
	}
	return ScoringContractV1
}

func scoringWeightTotal(scoring ScoringConfig) float64 {
	return scoring.MomentumWeight +
		scoring.ProfileWeight +
		scoring.QualityWeight +
		scoring.ValueWeight +
		scoring.ReversionWeight
}

func usesAdvancedScoring(scoring ScoringConfig) bool {
	return scoring.QualityWeight != 0 || scoring.ValueWeight != 0 || scoring.ReversionWeight != 0
}

func normalizedEventGuardrails(cfg StrategyConfig) EventGuardrailConfig {
	events := cfg.Events
	if events.LookbackDays <= 0 {
		events.LookbackDays = 30
	}
	if events.VetoScore == 0 {
		events.VetoScore = -0.6
	}
	if events.BoostScore == 0 {
		events.BoostScore = 0.6
	}
	if events.BoostAmount == 0 {
		events.BoostAmount = 0.05
	}
	return events
}

func LoadUniverse(path string) (Universe, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Universe{}, err
	}
	var universe Universe
	if err := json.Unmarshal(raw, &universe); err == nil && len(universe.Tickers) > 0 {
		universe.Tickers = NormalizeTickers(universe.Tickers)
		return universe, nil
	}
	var tickers []string
	if err := json.Unmarshal(raw, &tickers); err != nil {
		return Universe{}, err
	}
	return Universe{Tickers: NormalizeTickers(tickers)}, nil
}

func LoadPortfolio(path string) (Portfolio, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Portfolio{}, err
	}
	var portfolio Portfolio
	if err := json.Unmarshal(raw, &portfolio); err != nil {
		return Portfolio{}, err
	}
	if portfolio.Equity == 0 {
		for _, position := range portfolio.Positions {
			portfolio.Equity += position.MarketValue
		}
		portfolio.Equity += portfolio.Cash
	}
	return portfolio, nil
}

func NormalizeTickers(tickers []string) []string {
	seen := make(map[string]struct{}, len(tickers))
	out := make([]string, 0, len(tickers))
	for _, ticker := range tickers {
		ticker = strings.ToUpper(strings.TrimSpace(ticker))
		if ticker == "" {
			continue
		}
		if _, ok := seen[ticker]; ok {
			continue
		}
		seen[ticker] = struct{}{}
		out = append(out, ticker)
	}
	slices.Sort(out)
	return out
}

func HashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func ParseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "today") {
		return time.Now().UTC().Truncate(24 * time.Hour), nil
	}
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func RunID(prefix string, t time.Time) string {
	return fmt.Sprintf("%s_%s_%d", prefix, t.UTC().Format("20060102"), time.Now().UTC().UnixNano())
}

func AppendWarnings(dst []string, warnings ...string) []string {
	for _, warning := range warnings {
		if strings.TrimSpace(warning) != "" {
			dst = append(dst, warning)
		}
	}
	return dst
}

func SortSignals(signals []SignalResult) {
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].FinalScore == signals[j].FinalScore {
			return signals[i].Ticker < signals[j].Ticker
		}
		return signals[i].FinalScore > signals[j].FinalScore
	})
}

func CurrentWeights(portfolio Portfolio) map[string]float64 {
	weights := make(map[string]float64, len(portfolio.Positions))
	for _, pos := range portfolio.Positions {
		ticker := strings.ToUpper(strings.TrimSpace(pos.Ticker))
		if ticker == "" {
			continue
		}
		weight := pos.Weight
		if weight == 0 && portfolio.Equity > 0 {
			weight = pos.MarketValue / portfolio.Equity
		}
		weights[ticker] += weight
	}
	return weights
}

func EnsureApproved(cfg StrategyConfig) error {
	if !cfg.Approved {
		return errors.New("strategy config is not approved")
	}
	return nil
}
