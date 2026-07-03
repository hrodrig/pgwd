package metricsstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

func TestDriver_DefaultsToSQLiteWhenPathSet(t *testing.T) {
	cfg := &config.Config{SqlitePath: "/var/lib/pgwd.db"}
	if g := Driver(cfg); g != DriverSQLite {
		t.Fatalf("Driver: got %q want %s", g, DriverSQLite)
	}
}

func TestDriver_Explicit(t *testing.T) {
	cfg := &config.Config{MetricsStoreDriver: "postgres", SqlitePath: "/x.db"}
	if g := Driver(cfg); g != DriverPostgres {
		t.Fatalf("Driver: got %q", g)
	}
}

func TestDriver_PostgresqlAlias(t *testing.T) {
	cfg := &config.Config{MetricsStoreDriver: "postgresql", MetricsStoreDSN: "postgres://x"}
	if g := Driver(cfg); g != DriverPostgres {
		t.Fatalf("Driver: got %q want postgres", g)
	}
}

func TestExportRows_Postgres_InvalidDSN(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		MetricsStoreDriver: DriverPostgres,
		MetricsStoreDSN:    "postgres://127.0.0.1:65433/nonexistent?sslmode=disable",
	}
	_, err := ExportRows(ctx, cfg)
	if err == nil {
		t.Fatal("want error from unreachable postgres")
	}
}

func TestExportRows_nilConfig(t *testing.T) {
	_, err := ExportRows(context.Background(), nil)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestExportRows_noBackend(t *testing.T) {
	_, err := ExportRows(context.Background(), &config.Config{})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestExportRows_sqliteMissingPath(t *testing.T) {
	cfg := &config.Config{MetricsStoreDriver: DriverSQLite}
	_, err := ExportRows(context.Background(), cfg)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestExportRows_sqliteReadOnly(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "m.db")
	st, err := store.Open(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	cfg := &config.Config{SqlitePath: path}
	rows, err := ExportRows(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d", len(rows))
	}
}

func TestDriver_nilConfig(t *testing.T) {
	if Driver(nil) != "" {
		t.Fatal("want empty")
	}
}

func TestDriver_unknownDriver(t *testing.T) {
	cfg := &config.Config{MetricsStoreDriver: "redis", MetricsStoreDSN: "x"}
	if Driver(cfg) != "redis" {
		t.Fatalf("got %q", Driver(cfg))
	}
}
