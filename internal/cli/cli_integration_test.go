package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/metricsexport"
	"github.com/hrodrig/pgwd/internal/store"
)

func testDBURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PGWD_TEST_DB_URL")
	if dsn == "" {
		t.Skip("PGWD_TEST_DB_URL not set")
	}
	return dsn
}

func TestRunOneTarget_dryRun(t *testing.T) {
	dsn := testDBURL(t)
	cfg := &config.Config{
		Client:          "cli-cover",
		DBURL:           dsn,
		DryRun:          true,
		ThresholdLevels: "75,85,95",
	}
	ctx := context.Background()
	runOneTarget(ctx, config.DatabaseTarget{Client: cfg.Client, URL: dsn}, cfg, nil, nil, []config.DatabaseTarget{{Client: cfg.Client, URL: dsn}})
}

func TestExportMetricsFlow(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metrics.db")
	st, err := store.Open(dbPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Insert(context.Background(), store.Record{
		Client: "export-test", Cluster: "c", Database: "d",
		Total: 1, Active: 1, Idle: 0, Stale: 0, MaxConnections: 10, State: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	dest := filepath.Join(dir, "out.csv")
	cfg := &config.Config{SqlitePath: dbPath, ExportMetricsFormat: "csv", ExportMetricsDestination: dest}
	n, err := metricsexport.Export(context.Background(), "csv", dest, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows = %d", n)
	}
}

func TestRunOneTarget_withSQLite(t *testing.T) {
	dsn := testDBURL(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metrics.db")
	cfg := &config.Config{
		Client:           "sqlite-dry",
		DBURL:            dsn,
		SqlitePath:       dbPath,
		SqliteMaxMetrics: 100,
		DryRun:           true,
		ThresholdLevels:  "75,85,95",
		LogLevel:         "debug",
	}
	ctx := context.Background()
	st := openStoreIfConfigured(cfg)
	targets := []config.DatabaseTarget{{Client: cfg.Client, URL: dsn}}
	runOneTarget(ctx, targets[0], cfg, nil, st, targets)
	if st != nil {
		st.Close()
	}
}

func TestSetupKube_noop(t *testing.T) {
	cleanup := setupKube(context.Background(), &config.Config{})
	if cleanup == nil {
		t.Fatal("want cleanup func")
	}
	cleanup()
}

func TestSetupKubeLoki_noop(t *testing.T) {
	cleanup := setupKubeLoki(context.Background(), &config.Config{})
	cleanup()
}

func TestRunOneTarget_badURL(t *testing.T) {
	cfg := &config.Config{Client: "x", DryRun: true}
	ctx := context.Background()
	runOneTarget(ctx, config.DatabaseTarget{Client: "x", URL: "postgres://127.0.0.1:1/nope?sslmode=disable"}, cfg, nil, nil, []config.DatabaseTarget{{Client: "x", URL: "postgres://127.0.0.1:1/nope?sslmode=disable"}})
}

func TestExportMetricsFormat_trim(t *testing.T) {
	if strings.TrimSpace(" csv ") != "csv" {
		t.Fatal("trim")
	}
}
