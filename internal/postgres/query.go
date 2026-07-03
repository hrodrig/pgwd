package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier runs read queries against Postgres (implemented by *pgxpool.Pool and test mocks).
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
