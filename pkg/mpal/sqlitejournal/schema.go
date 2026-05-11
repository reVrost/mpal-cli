package sqlitejournal

import _ "embed"

// Schema is the SQLite schema for durable trade review journaling.
//
//go:embed schema.sql
var Schema string
