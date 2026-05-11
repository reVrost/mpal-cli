package mpal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	JournalTypeBaselinePlan       = "baseline_plan"
	JournalTypeAgentFinalAction   = "agent_final_action"
	JournalTypeAgentVeto          = "agent_veto"
	JournalTypeAgentOverride      = "agent_override"
	JournalTypeWeeklyTradeReview  = "weekly_trade_review"
	JournalTypeWeeklyTradeOutcome = "weekly_trade_outcome"
	JournalTypeBacktest           = "backtest"
)

type JournalEntry struct {
	ID                string       `json:"id"`
	RunID             string       `json:"run_id,omitempty"`
	Type              string       `json:"type"`
	BaselineJournalID string       `json:"baseline_journal_id,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	AsOf              *time.Time   `json:"as_of,omitempty"`
	Strategy          *StrategyRef `json:"strategy,omitempty"`
	Input             any          `json:"input,omitempty"`
	Output            any          `json:"output,omitempty"`
	Warnings          []string     `json:"warnings,omitempty"`
}

type FileJournal struct {
	Path string
}

func DefaultJournalPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return filepath.Join(".marketpal", "journal.jsonl")
	}
	return filepath.Join(home, ".marketpal", "journal.jsonl")
}

func (j FileJournal) Append(ctx context.Context, entry JournalEntry) (JournalEntry, error) {
	_ = ctx
	if entry.ID == "" {
		entry.ID = RunID("jrnl", time.Now().UTC())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Dir(j.Path), 0o755); err != nil {
		return JournalEntry{}, err
	}
	f, err := os.OpenFile(j.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return JournalEntry{}, err
	}
	defer f.Close()
	raw, err := json.Marshal(entry)
	if err != nil {
		return JournalEntry{}, err
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return JournalEntry{}, err
	}
	return entry, nil
}

func (j FileJournal) List(ctx context.Context, limit int) ([]JournalEntry, error) {
	_ = ctx
	entries, err := j.readAll()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	reversed := make([]JournalEntry, 0, limit)
	for i := len(entries) - 1; i >= 0 && len(reversed) < limit; i-- {
		reversed = append(reversed, entries[i])
	}
	return reversed, nil
}

func (j FileJournal) Get(ctx context.Context, id string) (JournalEntry, error) {
	_ = ctx
	entries, err := j.readAll()
	if err != nil {
		return JournalEntry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return JournalEntry{}, fmt.Errorf("journal entry %q not found", id)
}

func (j FileJournal) readAll() ([]JournalEntry, error) {
	f, err := os.Open(j.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return []JournalEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var entries []JournalEntry
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var entry JournalEntry
			if unmarshalErr := json.Unmarshal(line, &entry); unmarshalErr == nil {
				entries = append(entries, entry)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}
	return entries, nil
}
