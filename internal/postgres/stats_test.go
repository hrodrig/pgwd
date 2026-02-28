package postgres

import (
	"context"
	"os"
	"testing"
)

// Integration tests require a running PostgreSQL. Set PGWD_TEST_DB_URL
// (e.g. postgres://user:pass@localhost:5432/postgres?sslmode=disable) to run them.
// In CI or when unset, tests are skipped.

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PGWD_TEST_DB_URL")
	if dsn == "" {
		t.Skip("PGWD_TEST_DB_URL not set (skip integration tests)")
	}
	return dsn
}

func TestPool_Integration(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN(t)
	pool, err := Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}
	defer pool.Close()
	// Pool created and closed without error
}

func TestStats_Integration(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN(t)
	pool, err := Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}
	defer pool.Close()

	stats, err := Stats(ctx, pool)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Total < 0 || stats.Active < 0 || stats.Idle < 0 {
		t.Errorf("Stats: expected non-negative counts, got total=%d active=%d idle=%d", stats.Total, stats.Active, stats.Idle)
	}
	if stats.Total != stats.Active+stats.Idle {
		// Other states (e.g. idle in transaction) can exist; at least total should be >= active+idle
		if stats.Total < stats.Active+stats.Idle {
			t.Errorf("Stats: total (%d) should be >= active+idle (%d+%d)", stats.Total, stats.Active, stats.Idle)
		}
	}
}

func TestMaxConnections_Integration(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN(t)
	pool, err := Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}
	defer pool.Close()

	n, err := MaxConnections(ctx, pool)
	if err != nil {
		t.Fatalf("MaxConnections: %v", err)
	}
	if n < 1 {
		t.Errorf("MaxConnections: expected >= 1, got %d", n)
	}
}

func TestStaleCount_Integration(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN(t)
	pool, err := Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("Pool: %v", err)
	}
	defer pool.Close()

	// Connections older than 1 year: normally 0
	n, err := StaleCount(ctx, pool, 365*24*3600)
	if err != nil {
		t.Fatalf("StaleCount: %v", err)
	}
	if n < 0 {
		t.Errorf("StaleCount: expected non-negative, got %d", n)
	}
}
