package metricsstore

import (
	"context"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
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
