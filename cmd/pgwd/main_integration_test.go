package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestMain_DryRunWithDBURL(t *testing.T) {
	dsn := testDBURL(t)
	_, stderr, code := runBinary(
		"-client", "pgwd-cover-test",
		"-db-url", dsn,
		"-dry-run",
		"-db-threshold-levels", "75,85,95",
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr)
	}
}

func TestMain_ExportMetricsCSV(t *testing.T) {
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

	sqliteConf := filepath.Join(dir, "pgwd.conf")
	conf := "client: export-test\nsqlite:\n  path: " + dbPath + "\n"
	if err := os.WriteFile(sqliteConf, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	outCSV := filepath.Join(dir, "out.csv")
	_, stderr, code := runBinary(
		"-config", sqliteConf,
		"-export-metrics-format", "csv",
		"-export-metrics-destination", outCSV,
	)
	if code != 0 {
		t.Fatalf("export exit %d stderr=%q", code, stderr)
	}
	data, err := os.ReadFile(outCSV)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "export-test") {
		t.Fatalf("csv missing client: %q", string(data))
	}
}

func TestMain_DryRunWithSQLite(t *testing.T) {
	dsn := testDBURL(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metrics.db")
	sqliteConf := filepath.Join(dir, "pgwd.conf")
	conf := "client: sqlite-dry\nsqlite:\n  path: " + dbPath + "\ndatabases:\n  - url: " + dsn + "\n"
	if err := os.WriteFile(sqliteConf, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runBinary(
		"-config", sqliteConf,
		"-dry-run",
		"-db-threshold-levels", "75,85,95",
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr)
	}
}

func TestMain_LogConfigTrace_debug(t *testing.T) {
	dsn := testDBURL(t)
	_, _, code := runBinary(
		"-client", "pgwd-cover-test",
		"-db-url", dsn,
		"-dry-run",
		"-log-level", "debug",
		"-db-threshold-levels", "75,85,95",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
}
