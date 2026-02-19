package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectionStats holds counts from pg_stat_activity.
type ConnectionStats struct {
	Total  int
	Active int
	Idle   int
}

// Stats returns connection counts (total, active, idle) from the database.
func Stats(ctx context.Context, pool *pgxpool.Pool) (ConnectionStats, error) {
	const q = `
SELECT
	count(*) FILTER (WHERE state = 'active')   AS active,
	count(*) FILTER (WHERE state = 'idle')     AS idle,
	count(*)                                   AS total
FROM pg_stat_activity
WHERE datname = current_database()
`
	var s ConnectionStats
	err := pool.QueryRow(ctx, q).Scan(&s.Active, &s.Idle, &s.Total)
	return s, err
}

// StaleCount returns the number of connections that have been open longer than maxAgeSeconds
// (based on backend_start). Use this to detect connections that stay open and never close.
func StaleCount(ctx context.Context, pool *pgxpool.Pool, maxAgeSeconds int) (int, error) {
	const q = `
SELECT count(*)
FROM pg_stat_activity
WHERE datname = current_database()
  AND (now() - backend_start) > (make_interval(secs => $1))
`
	var n int
	err := pool.QueryRow(ctx, q, maxAgeSeconds).Scan(&n)
	return n, err
}

// Pool creates a connection pool for the given DSN.
func Pool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

// MaxConnections returns the server's max_connections setting.
func MaxConnections(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, "SELECT current_setting('max_connections')::int").Scan(&n)
	return n, err
}
