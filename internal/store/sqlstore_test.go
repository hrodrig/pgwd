package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenSQLMetrics_invalidDriver(t *testing.T) {
	_, err := OpenSQLMetrics("sqlite", "postgres://x", 100)
	if err == nil {
		t.Fatal("want error for unsupported driver")
	}
}

func TestOpenSQLMetrics_emptyDSN(t *testing.T) {
	_, err := OpenSQLMetrics("postgres", "  ", 100)
	if err == nil {
		t.Fatal("want error for empty DSN")
	}
}

func TestQueryAllMetricsFromDSN_emptyDSN(t *testing.T) {
	_, err := QueryAllMetricsFromDSN(context.Background(), "postgres", "")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestQueryAllMetricsFromDSN_invalidDriver(t *testing.T) {
	_, err := QueryAllMetricsFromDSN(context.Background(), "bad", "postgres://x")
	if err == nil {
		t.Fatal("want error")
	}
}

func testPostgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PGWD_TEST_DB_URL")
	if dsn == "" {
		t.Skip("PGWD_TEST_DB_URL not set")
	}
	return dsn
}

func TestSQLStore_Postgres_Integration(t *testing.T) {
	ctx := context.Background()
	dsn := testPostgresDSN(t)
	st, err := OpenSQLMetrics("postgres", dsn, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	rec := Record{
		Client: "sqlstore-test", Cluster: "cl", Database: "db1",
		Total: 10, Active: 2, Idle: 8, Stale: 0, MaxConnections: 100,
		State: "ok", Threshold: "",
	}
	for i := 0; i < 5; i++ {
		if err := st.Insert(ctx, rec); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}

	states, err := st.LastStates(ctx, rec.Client, rec.Cluster, rec.Database, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("states = %v", states)
	}

	latest, err := st.LatestRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) < 1 {
		t.Fatal("want latest records")
	}

	rows, err := QueryAllMetricsFromDSN(ctx, "postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("export rows empty")
	}

	if err := st.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestStore_QueryAllMetricsReadOnly_withRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "m.db")
	st, err := Open(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	rec := Record{Client: "c", Cluster: "cl", Database: "d", Total: 1, Active: 1, Idle: 0, Stale: 0, MaxConnections: 10, State: "ok"}
	if err := st.Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	st.Close()

	rows, err := QueryAllMetricsReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
}

func TestStore_Open_invalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/pgwd.db", 10)
	if err == nil {
		t.Fatal("want open error")
	}
}

func TestStore_Ping(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "p.db"), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}
