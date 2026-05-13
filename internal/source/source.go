// Package source defines the PlanSource interface and the RawPlan type that all
// source implementations emit. The watcher depends only on these types — no
// source-specific logic leaks into the rest of the codebase.
package source

import (
	"context"
	"time"
)

// RawPlan is a query execution plan captured from a PlanSource.
// It carries the raw EXPLAIN JSON along with metadata extracted from the log line.
type RawPlan struct {
	// PlanJSON holds the raw EXPLAIN (ANALYZE, FORMAT JSON) output.
	// Pass this directly to parser.Parse() from the pgexplain library.
	PlanJSON []byte

	// DurationMs is the total wall-clock execution time reported in the log prefix
	// (e.g. "duration: 1243.821 ms"). It matches Execution Time in the plan JSON.
	DurationMs float64

	// Timestamp is when the query finished, parsed from the log line prefix.
	Timestamp time.Time

	// Database is the PostgreSQL database name the query ran against.
	Database string

	// User is the PostgreSQL role that executed the query.
	User string

	// SourceName identifies which PlanSource produced this plan
	// (e.g. "auto_explain", "pg_store_plans").
	SourceName string
}

// PlanSource is implemented by any component that can stream query execution plans.
// The watcher depends only on this interface — no source-specific logic leaks out.
// Adding a new source (pg_store_plans, proxy, etc.) means implementing PlanSource
// and wiring it in cmd/pgwatch/wire.go.
type PlanSource interface {
	// Name returns a human-readable identifier for this source (e.g. "auto_explain").
	Name() string

	// Plans opens the source and returns a channel of RawPlans.
	// The channel is closed when the source is exhausted (EOF in one-shot mode)
	// or when ctx is cancelled (daemon mode). Plans must not block the caller —
	// streaming happens in a background goroutine.
	Plans(ctx context.Context) (<-chan RawPlan, error)

	// Close releases any resources held by the source (open files, connections).
	Close() error
}
