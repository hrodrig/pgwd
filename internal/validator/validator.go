// Package validator provides config validation that returns errors instead of exiting.
// Used by cmd/pgwd; main logs and exits on error.
package validator

import (
	"fmt"
	"os"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/kube"
	"github.com/hrodrig/pgwd/internal/metricsstore"
)

// Validate runs all config validations. Returns the first error encountered.
// WarnDeprecatedThresholds is called during validation (writes to stderr).
func Validate(cfg *config.Config) error {
	if err := ValidateDatabases(cfg); err != nil {
		return err
	}
	if err := ValidateDBURL(cfg); err != nil {
		return err
	}
	if err := ValidateClient(cfg); err != nil {
		return err
	}
	WarnDeprecatedThresholds(cfg)
	if err := ValidateStale(cfg); err != nil {
		return err
	}
	if err := ValidateNotifiers(cfg); err != nil {
		return err
	}
	if err := ValidateKubePostgres(cfg); err != nil {
		return err
	}
	if err := ValidateKubePostgresFormat(cfg); err != nil {
		return err
	}
	if err := ValidateKubeLoki(cfg); err != nil {
		return err
	}
	if err := ValidateMetricsStore(cfg); err != nil {
		return err
	}
	return nil
}

// WarnDeprecatedThresholds prints a deprecation warning when legacy thresholds are used.
func WarnDeprecatedThresholds(cfg *config.Config) {
	if cfg.ThresholdTotal > 0 || cfg.ThresholdActive > 0 {
		fmt.Fprintln(os.Stderr, "pgwd: -db-threshold-total and -db-threshold-active are deprecated and will be removed in v1.0.0; use -db-threshold-levels instead (e.g. -db-threshold-levels 75,85,95)")
	}
}

// ValidateDatabases checks databases config when UsesDatabases.
func ValidateDatabases(cfg *config.Config) error {
	if !cfg.UsesDatabases() {
		return nil
	}
	if cfg.KubePostgres != "" {
		return fmt.Errorf("pgwd: kube-postgres is not supported with databases (multi-DB); use db (single) or add per-db kube in a future release")
	}
	for i, t := range cfg.Databases {
		if t.URL == "" {
			return fmt.Errorf("pgwd: databases[%d] missing url", i)
		}
	}
	return nil
}

// ValidateClient ensures client is set.
func ValidateClient(cfg *config.Config) error {
	if cfg.Client != "" {
		return nil
	}
	if cfg.UsesDatabases() {
		return fmt.Errorf("pgwd: client is required when using databases (needed to derive per-target client names)")
	}
	return fmt.Errorf("pgwd: client is required: set client in config or -client (identifies this monitor instance)")
}

// ValidateDBURL ensures db URL is set in single-DB mode.
func ValidateDBURL(cfg *config.Config) error {
	if cfg.UsesDatabases() {
		return nil
	}
	if cfg.DBURL == "" {
		return fmt.Errorf("pgwd: missing database URL: set PGWD_DB_URL or -db-url")
	}
	return nil
}

// ValidateStale ensures stale-age is set when threshold-stale is used.
func ValidateStale(cfg *config.Config) error {
	if cfg.ThresholdStale > 0 && cfg.StaleAge <= 0 {
		return fmt.Errorf("pgwd: when using -db-threshold-stale, -db-stale-age must be > 0 (PGWD_DB_STALE_AGE)")
	}
	return nil
}

// ValidateNotifiers ensures at least one notifier or dry-run when needed.
func ValidateNotifiers(cfg *config.Config) error {
	if !cfg.HasAnyNotifier() && !cfg.DryRun {
		return fmt.Errorf("pgwd: no notifier configured: set PGWD_NOTIFICATIONS_SLACK_WEBHOOK and/or PGWD_NOTIFICATIONS_LOKI_URL (or -notifications-slack-webhook / -notifications-loki-url), or use -dry-run")
	}
	if cfg.ForceNotification && !cfg.HasAnyNotifier() {
		return fmt.Errorf("pgwd: force-notification requires at least one notifier (-notifications-slack-webhook or -notifications-loki-url)")
	}
	if cfg.NotifyOnConnectFailure && !cfg.HasAnyNotifier() {
		return fmt.Errorf("pgwd: notify-on-connect-failure requires at least one notifier (-notifications-slack-webhook or -notifications-loki-url)")
	}
	return nil
}

// ValidateKubePostgres ensures db-url is set when kube-postgres is used.
func ValidateKubePostgres(cfg *config.Config) error {
	if cfg.KubePostgres == "" || cfg.DBURL != "" {
		return nil
	}
	return fmt.Errorf("pgwd: kube-postgres requires PGWD_DB_URL or -db-url (use host localhost and the same port as -kube-local-port)")
}

// ValidateKubePostgresFormat validates kube-postgres format (namespace/type/name).
func ValidateKubePostgresFormat(cfg *config.Config) error {
	if cfg.KubePostgres == "" {
		return nil
	}
	_, _, err := kube.ParseKubePostgres(cfg.KubePostgres)
	return err
}

// ValidateKubeLoki ensures kube-loki and loki-url are not both set, and port ranges.
func ValidateKubeLoki(cfg *config.Config) error {
	if cfg.KubeLoki == "" {
		return nil
	}
	if cfg.LokiURL != "" {
		return fmt.Errorf("pgwd: use -kube-loki OR -notifications-loki-url, not both (-notifications-loki-url for exposed Loki, -kube-loki when Loki is inside the cluster)")
	}
	if cfg.KubeLokiLocalPort < 1 || cfg.KubeLokiLocalPort > 65535 {
		return fmt.Errorf("pgwd: kube-loki-local-port must be between 1 and 65535")
	}
	if cfg.KubeLokiRemotePort < 1 || cfg.KubeLokiRemotePort > 65535 {
		return fmt.Errorf("pgwd: kube-loki-remote-port must be between 1 and 65535")
	}
	_, _, err := kube.ParseKubePostgres(cfg.KubeLoki)
	return err
}

// ValidateMetricsStore checks metrics_store / sqlite when a backend is selected.
func ValidateMetricsStore(cfg *config.Config) error {
	d := metricsstore.Driver(cfg)
	switch d {
	case "":
		return nil
	case metricsstore.DriverSQLite:
		if strings.TrimSpace(cfg.SqlitePath) == "" {
			return fmt.Errorf("pgwd: metrics store (sqlite): sqlite.path is required when using the SQLite metrics backend")
		}
	case metricsstore.DriverPostgres, metricsstore.DriverMySQL:
		if strings.TrimSpace(cfg.MetricsStoreDSN) == "" {
			return fmt.Errorf("pgwd: metrics store (%s): metrics_store.dsn (or PGWD_METRICS_STORE_DSN) is required", d)
		}
	default:
		return fmt.Errorf("pgwd: metrics store: unsupported metrics_store.driver %q (use sqlite, postgres, or mysql)", cfg.MetricsStoreDriver)
	}
	return nil
}
