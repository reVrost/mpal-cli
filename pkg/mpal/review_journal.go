package mpal

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/revrost/mpal-cli/pkg/mpal/sqlitejournal"
	_ "modernc.org/sqlite"
)

type SQLiteReviewJournal struct {
	Path string

	db      *sql.DB
	queries *sqlitejournal.Queries
}

func DefaultReviewJournalPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return filepath.Join(".marketpal", "mpal.db")
	}
	return filepath.Join(home, ".marketpal", "mpal.db")
}

func OpenSQLiteReviewJournal(path string) (*SQLiteReviewJournal, error) {
	if path == "" {
		path = DefaultReviewJournalPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteReviewJournal{
		Path:    path,
		db:      db,
		queries: sqlitejournal.New(db),
	}, nil
}

func (j *SQLiteReviewJournal) Close() error {
	if j == nil || j.db == nil {
		return nil
	}
	return j.db.Close()
}

func (j *SQLiteReviewJournal) Migrate(ctx context.Context) error {
	if j == nil || j.db == nil {
		return fmt.Errorf("sqlite review journal is not open")
	}
	if _, err := j.db.ExecContext(ctx, sqlitejournal.Schema); err != nil {
		return err
	}
	for _, column := range []struct {
		name string
		typ  string
	}{
		{name: "model_estimated_value", typ: "REAL"},
		{name: "raw_kelly", typ: "REAL"},
		{name: "fractional_kelly", typ: "REAL"},
		{name: "kelly_target_weight", typ: "REAL"},
	} {
		if err := j.ensureTradeReviewPositionColumn(ctx, column.name, column.typ); err != nil {
			return err
		}
	}
	return nil
}

func (j *SQLiteReviewJournal) Queries() *sqlitejournal.Queries {
	if j == nil {
		return nil
	}
	return j.queries
}

func (j *SQLiteReviewJournal) AppendReview(
	ctx context.Context,
	review sqlitejournal.CreateTradeReviewParams,
	positions []sqlitejournal.CreateTradeReviewPositionParams,
) error {
	if j == nil || j.db == nil {
		return fmt.Errorf("sqlite review journal is not open")
	}
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := j.queries.WithTx(tx)
	if err := qtx.CreateTradeReview(ctx, review); err != nil {
		return err
	}
	for _, position := range positions {
		if err := qtx.CreateTradeReviewPosition(ctx, position); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (j *SQLiteReviewJournal) FinalizeReview(
	ctx context.Context,
	review sqlitejournal.FinalizeTradeReviewParams,
	positions []sqlitejournal.UpsertHumanTradeReviewPositionParams,
) error {
	if j == nil || j.db == nil {
		return fmt.Errorf("sqlite review journal is not open")
	}
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := j.queries.WithTx(tx)
	if err := qtx.FinalizeTradeReview(ctx, review); err != nil {
		return err
	}
	for _, position := range positions {
		if err := qtx.UpsertHumanTradeReviewPosition(ctx, position); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (j *SQLiteReviewJournal) GetReview(
	ctx context.Context,
	id string,
) (sqlitejournal.TradeReview, []sqlitejournal.TradeReviewPosition, error) {
	if j == nil || j.queries == nil {
		return sqlitejournal.TradeReview{}, nil, fmt.Errorf("sqlite review journal is not open")
	}
	review, err := j.queries.GetTradeReview(ctx, id)
	if err != nil {
		return sqlitejournal.TradeReview{}, nil, err
	}
	positions, err := j.queries.ListTradeReviewPositions(ctx, id)
	if err != nil {
		return sqlitejournal.TradeReview{}, nil, err
	}
	return review, positions, nil
}

func (j *SQLiteReviewJournal) ListReviews(ctx context.Context, limit int64) ([]sqlitejournal.TradeReview, error) {
	if j == nil || j.queries == nil {
		return nil, fmt.Errorf("sqlite review journal is not open")
	}
	if limit <= 0 {
		limit = 20
	}
	return j.queries.ListTradeReviews(ctx, limit)
}

func (j *SQLiteReviewJournal) SetReportPath(ctx context.Context, id string, path string) error {
	if j == nil || j.queries == nil {
		return fmt.Errorf("sqlite review journal is not open")
	}
	return j.queries.SetTradeReviewReportPath(ctx, sqlitejournal.SetTradeReviewReportPathParams{
		ID:         id,
		ReportPath: nullSQLString(path),
	})
}

func (j *SQLiteReviewJournal) ensureTradeReviewPositionColumn(ctx context.Context, name string, typ string) error {
	rows, err := j.db.QueryContext(ctx, "PRAGMA table_info(trade_review_positions)")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = j.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE trade_review_positions ADD COLUMN %s %s", name, typ))
	return err
}
