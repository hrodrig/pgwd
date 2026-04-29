// Package metricsstore selects how pgwd persists check history (metrics time series).
//
// Backends: sqlite.path (SQLite), or metrics_store.driver + metrics_store.dsn (PostgreSQL, MySQL).
// Export and writers should go through this package where appropriate.
package metricsstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

// Driver names (config metrics_store.driver / PGWD_METRICS_STORE_DRIVER).
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"
)

// Driver returns the effective metrics persistence backend for cfg.
// When metrics_store.driver is empty but sqlite.path is set, defaults to sqlite.
// Aliases: postgresql → postgres.
func Driver(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	d := strings.ToLower(strings.TrimSpace(cfg.MetricsStoreDriver))
	switch d {
	case "postgresql":
		d = DriverPostgres
	case "sqlite":
		// explicit sqlite in YAML/env
	default:
		// keep d as-is for unknown (validated elsewhere)
	}
	if d != "" {
		return d
	}
	if cfg.SqlitePath != "" {
		return DriverSQLite
	}
	return ""
}

// ExportRows returns all persisted metrics rows from the configured store, for CSV and other sinks.
// For SQLite, uses a read-only connection so export can run while the daemon holds the database.
// For PostgreSQL/MySQL, opens a short-lived connection with the configured DSN.
func ExportRows(ctx context.Context, cfg *config.Config) ([]store.ExportRow, error) {
	if cfg == nil {
		return nil, fmt.Errorf("metrics store: config is nil")
	}
	switch Driver(cfg) {
	case DriverSQLite:
		if cfg.SqlitePath == "" {
			return nil, fmt.Errorf("metrics store (sqlite): sqlite.path is required")
		}
		return store.QueryAllMetricsReadOnly(ctx, cfg.SqlitePath)
	case DriverPostgres, DriverMySQL:
		if strings.TrimSpace(cfg.MetricsStoreDSN) == "" {
			return nil, fmt.Errorf("metrics store (%s): metrics_store.dsn or PGWD_METRICS_STORE_DSN is required", Driver(cfg))
		}
		return store.QueryAllMetricsFromDSN(ctx, Driver(cfg), cfg.MetricsStoreDSN)
	case "":
		return nil, fmt.Errorf("metrics store: no backend configured (set sqlite.path, or metrics_store.driver + metrics_store.dsn)")
	default:
		return nil, fmt.Errorf("metrics store: unknown driver %q (supported: %s, %s, %s)", Driver(cfg), DriverSQLite, DriverPostgres, DriverMySQL)
	}
}
