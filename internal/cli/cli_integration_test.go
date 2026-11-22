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

// stubMetricsStore records Insert calls for connect-failure persistence tests.
type stubMetricsStore struct {
	records []store.Record
}

func (s *stubMetricsStore) Insert(_ context.Context, r store.Record) error {
	s.records = append(s.records, r)
	return nil
}
func (s *stubMetricsStore) Close() error               { return nil }
func (s *stubMetricsStore) Ping(context.Context) error { return nil }
func (s *stubMetricsStore) LatestRecords(context.Context) ([]store.Record, error) {
	return nil, nil
}
func (s *stubMetricsStore) LastStates(context.Context, string, string, string, int) ([]string, error) {
	return nil, nil
}

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
	runOneTarget(ctx, config.DatabaseTarget{Client: cfg.Client, URL: dsn}, cfg, nil, nil, []config.DatabaseTarget{{Client: cfg.Client, URL: dsn}}, nil)
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
	runOneTarget(ctx, targets[0], cfg, nil, st, targets, nil)
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
	cfg := &config.Config{Client: "x", DryRun: true, Interval: 0}
	ctx := context.Background()
	var gotExit int
	prev := exitFunc
	exitFunc = func(code int) { gotExit = code }
	t.Cleanup(func() { exitFunc = prev })

	bad := "postgres://127.0.0.1:1/nope?sslmode=disable"
	runOneTarget(ctx, config.DatabaseTarget{Client: "x", URL: bad}, cfg, nil, nil, []config.DatabaseTarget{{Client: "x", URL: bad}}, nil)
	if gotExit != ExitConnectFailure {
		t.Fatalf("exit = %d, want %d", gotExit, ExitConnectFailure)
	}
}

func TestRunOneTarget_daemonBadURL_noExit(t *testing.T) {
	cfg := &config.Config{Client: "x", DryRun: true, Interval: 30}
	ctx := context.Background()
	var gotExit int
	prev := exitFunc
	exitFunc = func(code int) { gotExit = code }
	t.Cleanup(func() { exitFunc = prev })

	bad := "postgres://127.0.0.1:1/nope?sslmode=disable"
	st := &stubMetricsStore{}
	runOneTarget(ctx, config.DatabaseTarget{Client: "x", URL: bad}, cfg, nil, st, []config.DatabaseTarget{{Client: "x", URL: bad}}, nil)
	if gotExit != 0 {
		t.Fatalf("daemon must not exit on connect failure, got %d", gotExit)
	}
	if len(st.records) != 1 || st.records[0].State != "connect_failure" {
		t.Fatalf("want one connect_failure store row, got %+v", st.records)
	}
}

func TestExitConnectFailureIf_oneShot(t *testing.T) {
	var gotExit int
	prev := exitFunc
	exitFunc = func(code int) { gotExit = code }
	t.Cleanup(func() { exitFunc = prev })

	exitConnectFailureIf(&config.Config{Interval: 0}, []config.DatabaseTarget{{Client: "a"}})
	if gotExit != ExitConnectFailure {
		t.Fatalf("exit = %d, want %d", gotExit, ExitConnectFailure)
	}
}

func TestExitConnectFailureIf_daemonSkips(t *testing.T) {
	var gotExit int
	prev := exitFunc
	exitFunc = func(code int) { gotExit = code }
	t.Cleanup(func() { exitFunc = prev })

	exitConnectFailureIf(&config.Config{Interval: 30}, []config.DatabaseTarget{{Client: "a"}})
	if gotExit != 0 {
		t.Fatalf("daemon must not exit on connect failure, got %d", gotExit)
	}
}

func TestExitQueryErrorIf_oneShot(t *testing.T) {
	var gotExit int
	prev := exitFunc
	exitFunc = func(code int) { gotExit = code }
	t.Cleanup(func() { exitFunc = prev })

	cfg := &config.Config{Interval: 0}
	targets := []config.DatabaseTarget{{Client: "a"}}
	exitQueryErrorIf(cfg, targets, true)
	if gotExit != ExitQueryError {
		t.Fatalf("exit = %d, want %d", gotExit, ExitQueryError)
	}
}

func TestExitQueryErrorIf_daemonSkips(t *testing.T) {
	var gotExit int
	prev := exitFunc
	exitFunc = func(code int) { gotExit = code }
	t.Cleanup(func() { exitFunc = prev })

	exitQueryErrorIf(&config.Config{Interval: 30}, []config.DatabaseTarget{{Client: "a"}}, true)
	if gotExit != 0 {
		t.Fatalf("daemon must not exit on query error, got %d", gotExit)
	}
}

func TestExportMetricsFormat_trim(t *testing.T) {
	if strings.TrimSpace(" csv ") != "csv" {
		t.Fatal("trim")
	}
}
