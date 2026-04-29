package store

import "context"

// MetricsStorer persists check history and serves /metrics and hysteresis queries.
// Implemented by *Store (SQLite) and *SQLStore (PostgreSQL / MySQL).
type MetricsStorer interface {
	Insert(ctx context.Context, r Record) error
	Close() error
	Ping(ctx context.Context) error
	LatestRecords(ctx context.Context) ([]Record, error)
	LastStates(ctx context.Context, client, cluster, database string, n int) ([]string, error)
}
