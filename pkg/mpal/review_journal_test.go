package mpal

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/revrost/mpal-cli/pkg/mpal/sqlitejournal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteReviewJournalAppendGetList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	journal := openTestReviewJournal(t)

	err := journal.AppendReview(ctx, testReview("review_1"), []sqlitejournal.CreateTradeReviewPositionParams{
		testPosition("pos_1", "review_1", "MU", "proposed", "trade", "trade"),
		testPosition("pos_2", "review_1", "DOCN", "user_requested", "watchlist", "skip"),
	})
	require.NoError(t, err)

	review, positions, err := journal.GetReview(ctx, "review_1")
	require.NoError(t, err)
	require.Equal(t, "engine_weekly_swing_v1", review.StrategyID)
	require.Equal(t, "TRADE", review.ExecutionResult)
	require.True(t, review.AgentSummary.Valid)
	require.Len(t, positions, 2)
	require.Equal(t, "DOCN", positions[0].Ticker)
	require.Equal(t, "MU", positions[1].Ticker)

	reviews, err := journal.ListReviews(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	require.Equal(t, "review_1", reviews[0].ID)

	tickerRows, err := journal.Queries().ListTradeReviewPositionsByTicker(ctx, "MU")
	require.NoError(t, err)
	require.Len(t, tickerRows, 1)
	require.Equal(t, "review_1", tickerRows[0].TradeReviewID)
}

func TestSQLiteReviewJournalFinalizeUpdatesHumanDecision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	journal := openTestReviewJournal(t)

	require.NoError(t, journal.AppendReview(ctx, testReview("review_1"), []sqlitejournal.CreateTradeReviewPositionParams{
		testPosition("pos_1", "review_1", "MU", "proposed", "trade", ""),
	}))

	valid := true
	weight := 0.01
	final, positions, err := (TradeReviewFinalizeInput{
		FinalDecision:          "trade",
		HumanReasoningText:     "Accepted the agent-reviewed trade.",
		FinalValidationValid:   &valid,
		FinalValidationSummary: "Validated.",
		Positions: []TradeReviewHumanPositionInput{{
			Ticker:        "MU",
			HumanDecision: "trade",
			HumanWeight:   &weight,
			HumanReason:   "Final human call.",
		}},
	}).ToFinalizeParams("review_1", time.Now().UTC())
	require.NoError(t, err)
	require.NoError(t, journal.FinalizeReview(ctx, final, positions))

	review, gotPositions, err := journal.GetReview(ctx, "review_1")
	require.NoError(t, err)
	require.Equal(t, "trade", review.FinalDecision.String)
	require.Len(t, gotPositions, 1)
	require.Equal(t, "trade", gotPositions[0].HumanDecision.String)
	require.Equal(t, 0.01, gotPositions[0].HumanWeight.Float64)
}

func TestSQLiteReviewJournalConstraints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	journal := openTestReviewJournal(t)

	invalidJSON := testReview("bad_json")
	invalidJSON.UniverseTickersJson = "MU"
	require.Error(t, journal.Queries().CreateTradeReview(ctx, invalidJSON))

	require.NoError(t, journal.Queries().CreateTradeReview(ctx, testReview("review_1")))

	invalidDecision := testPosition("bad_decision", "review_1", "MU", "proposed", "buy_more", "trade")
	require.Error(t, journal.Queries().CreateTradeReviewPosition(ctx, invalidDecision))

	require.NoError(t, journal.Queries().CreateTradeReviewPosition(ctx, testPosition("pos_1", "review_1", "MU", "proposed", "trade", "trade")))
	duplicateTicker := testPosition("pos_2", "review_1", "MU", "alternate", "watchlist", "skip")
	require.Error(t, journal.Queries().CreateTradeReviewPosition(ctx, duplicateTicker))
}

func TestSQLiteReviewJournalCascadeDeletesPositions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	journal := openTestReviewJournal(t)

	require.NoError(t, journal.AppendReview(ctx, testReview("review_1"), []sqlitejournal.CreateTradeReviewPositionParams{
		testPosition("pos_1", "review_1", "MU", "proposed", "trade", "trade"),
	}))
	require.NoError(t, journal.Queries().DeleteTradeReview(ctx, "review_1"))

	positions, err := journal.Queries().ListTradeReviewPositions(ctx, "review_1")
	require.NoError(t, err)
	assert.Empty(t, positions)
}

func openTestReviewJournal(t *testing.T) *SQLiteReviewJournal {
	t.Helper()

	journal, err := OpenSQLiteReviewJournal(filepath.Join(t.TempDir(), "mpal.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, journal.Close())
	})
	require.NoError(t, journal.Migrate(context.Background()))
	return journal
}

func testReview(id string) sqlitejournal.CreateTradeReviewParams {
	return sqlitejournal.CreateTradeReviewParams{
		ID:                       id,
		AsOf:                     "2026-05-11",
		StrategyID:               "engine_weekly_swing_v1",
		StrategyConfigText:       "id: engine_weekly_swing_v1\nrisk:\n  sizing_method: fractional_kelly\n",
		PortfolioScope:           nullString("engine"),
		UniverseTickersJson:      `["MU","META","DOCN"]`,
		UserRequestedTickersJson: nullString(`["DOCN"]`),
		ExecutionResult:          "TRADE",
		AgentHarness:             nullString("codex"),
		AgentModel:               nullString("gpt-5"),
		AgentSkill:               nullString("marketpal-trader"),
		UserPromptText:           nullString("Run weekly engine review."),
		ChatHistoryText:          nullString("User asked for review; agent evaluated proposed trades and requested tickers."),
		AgentSummary:             nullString("Accept MU, keep DOCN on watchlist."),
		FinalDecision:            nullString("trade"),
		HumanReasoningText:       nullString("Accepted MU but did not add DOCN."),
		FinalValidationValid:     sql.NullInt64{Int64: 1, Valid: true},
		FinalValidationSummary:   nullString("Final plan validated."),
		ReportPath:               nullString("tmp/mpal-runs/review.html"),
		WarningsText:             nullString("Kelly calibration is heuristic."),
	}
}

func testPosition(id, reviewID, ticker, modelBucket, agentDecision, humanDecision string) sqlitejournal.CreateTradeReviewPositionParams {
	return sqlitejournal.CreateTradeReviewPositionParams{
		ID:                id,
		TradeReviewID:     reviewID,
		Ticker:            ticker,
		ModelBucket:       nullString(modelBucket),
		ModelIntent:       nullString("STARTER"),
		ModelScore:        sql.NullFloat64{Float64: 0.91, Valid: true},
		ModelWeight:       sql.NullFloat64{Float64: 0.015, Valid: true},
		ModelDeltaWeight:  sql.NullFloat64{Float64: 0.015, Valid: true},
		ModelReason:       nullString("starter position sized by deterministic strategy output"),
		SizingMethod:      nullString("fractional_kelly"),
		FinalTargetWeight: sql.NullFloat64{Float64: 0.015, Valid: true},
		BindingConstraint: nullString("max_single_trade_pct"),
		CalibrationStatus: nullString("heuristic_markov"),
		AgentDecision:     nullString(agentDecision),
		AgentWeight:       sql.NullFloat64{Float64: 0.01, Valid: true},
		AgentReason:       nullString("agent reviewed events and sizing"),
		HumanDecision:     nullString(humanDecision),
		HumanWeight:       sql.NullFloat64{Float64: 0.01, Valid: true},
		ExecutionPrice:    sql.NullFloat64{},
		ExecutionDate:     sql.NullString{},
		HumanReason:       nullString("final human call"),
	}
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
