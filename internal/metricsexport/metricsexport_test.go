package metricsexport

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

func TestExport_CSV(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "m.db")
	st, err := store.Open(dbPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Insert(ctx, store.Record{
		Client: "c1", Cluster: "cl", Namespace: "", Database: "db1",
		Total: 10, Active: 2, Idle: 8, Stale: 0, MaxConnections: 100,
		State: "ok", Threshold: "",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Insert(ctx, store.Record{
		Client: "c1", Cluster: "cl", Namespace: "", Database: "db1",
		Total: 20, Active: 3, Idle: 7, Stale: 0, MaxConnections: 100,
		State: "attention", Threshold: "total",
	}); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.csv")
	cfg := &config.Config{SqlitePath: dbPath}
	n, err := Export(ctx, FormatCSV, outPath, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows: got %d want 2", n)
	}
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cr := csv.NewReader(f)
	all, err := cr.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("csv lines: got %d want header+2", len(all))
	}
	if all[1][3] != "c1" || all[1][12] != "ok" {
		t.Fatalf("row1: %v", all[1])
	}
	if all[2][12] != "attention" {
		t.Fatalf("row2 state: %v", all[2])
	}
}

func TestExport_Postgres_ConnectError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.csv")
	cfg := &config.Config{
		MetricsStoreDriver: "postgres",
		MetricsStoreDSN:    "postgres://127.0.0.1:65433/nonexistent?sslmode=disable",
	}
	_, err := Export(ctx, FormatCSV, outPath, cfg)
	if err == nil {
		t.Fatal("want error from unreachable postgres")
	}
}

func TestExport_UnsupportedCSVFormat(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "m.db")
	st, err := store.Open(dbPath, 10)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	cfg := &config.Config{SqlitePath: dbPath}
	_, err = Export(ctx, "parquet", "/tmp/x", cfg)
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("want ErrUnsupportedFormat: %v", err)
	}
}
