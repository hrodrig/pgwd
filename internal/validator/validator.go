// Package validator provides config validation that returns errors instead of exiting.
// Used by cmd/pgwd; main logs and exits on error.
package validator

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/kube"
	"github.com/hrodrig/pgwd/internal/metricsstore"
)

// Validate runs all config validations. Returns the first error encountered.
func Validate(cfg *config.Config) error {
	if err := ValidateRemovedThresholdEnv(); err != nil {
		return err
	}
	if err := ValidateRemovedNotifyOnConnectFailureEnv(); err != nil {
		return err
	}
	if err := ValidateDatabases(cfg); err != nil {
		return err
	}
	if err := ValidateDBURL(cfg); err != nil {
		return err
	}
	if err := ValidateClient(cfg); err != nil {
		return err
	}
	WarnNotifierTLS(cfg)
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
	if err := ValidateLongQueryAlerts(cfg); err != nil {
		return err
	}
	return nil
}

// WarnNotifierTLS logs a startup warning when notifier URLs use plain HTTP (non-loopback).
func WarnNotifierTLS(cfg *config.Config) {
	warnHTTPNotifierURL("Slack", cfg.SlackWebhook)
	warnHTTPNotifierURL("Loki", cfg.LokiURL)
	if cfg.TeamsActive() {
		warnHTTPNotifierURL("Teams", cfg.TeamsWebhook)
	}
	if cfg.GenericActive() {
		warnHTTPNotifierURL("generic webhook", cfg.GenericWebhookURL)
	}
}

func warnHTTPNotifierURL(channel, rawURL string) {
	if rawURL == "" {
		return
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "http" {
		return
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return
	}
	fmt.Fprintf(os.Stderr, "pgwd: notifier %s uses http:// — prefer https:// for production traffic\n", channel)
}

// ValidateRemovedThresholdEnv rejects removed threshold env vars.
func ValidateRemovedThresholdEnv() error {
	if _, ok := os.LookupEnv("PGWD_DB_THRESHOLD_TOTAL"); ok {
		return fmt.Errorf("pgwd: PGWD_DB_THRESHOLD_TOTAL was removed in v1.0; use PGWD_DB_THRESHOLD_LEVELS (e.g. 75,85,95)")
	}
	if _, ok := os.LookupEnv("PGWD_DB_THRESHOLD_ACTIVE"); ok {
		return fmt.Errorf("pgwd: PGWD_DB_THRESHOLD_ACTIVE was removed in v1.0; use PGWD_DB_THRESHOLD_LEVELS (e.g. 75,85,95)")
	}
	return nil
}

// ValidateRemovedNotifyOnConnectFailureEnv rejects removed connect-failure env vars.
func ValidateRemovedNotifyOnConnectFailureEnv() error {
	if _, ok := os.LookupEnv("PGWD_NOTIFY_ON_CONNECT_FAILURE"); ok {
		return fmt.Errorf("pgwd: PGWD_NOTIFY_ON_CONNECT_FAILURE was removed in v1.0; connect failure notifications are always sent when notifiers are configured")
	}
	return nil
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
	if err := validateNotifierChannels(cfg); err != nil {
		return err
	}
	if err := validateNotifyRetry(cfg); err != nil {
		return err
	}
	if !cfg.HasAnyNotifier() && !cfg.DryRun {
		return fmt.Errorf("pgwd: no notifier configured: set a notification channel (Slack, Loki, PagerDuty, Teams, generic webhook, or kube-loki), or use -dry-run")
	}
	if cfg.ForceNotification && !cfg.HasAnyNotifier() {
		return fmt.Errorf("pgwd: force-notification requires at least one notifier")
	}
	return nil
}

func validateNotifierChannels(cfg *config.Config) error {
	if cfg.PagerDutyActive() && cfg.PagerDutyRoutingKey == "" {
		return fmt.Errorf("pgwd: notifications.pagerduty.routing_key is required when PagerDuty is enabled")
	}
	if cfg.TeamsActive() && cfg.TeamsWebhook == "" {
		return fmt.Errorf("pgwd: notifications.teams.webhook_url is required when Teams is enabled")
	}
	if cfg.GenericActive() && cfg.GenericWebhookURL == "" {
		return fmt.Errorf("pgwd: notifications.generic.webhook_url is required when generic webhook is enabled")
	}
	if cfg.GenericBodyTemplate != "" {
		if _, err := template.New("generic").Parse(cfg.GenericBodyTemplate); err != nil {
			return fmt.Errorf("pgwd: notifications.generic.body_template: %w", err)
		}
	}
	return nil
}

func validateNotifyRetry(cfg *config.Config) error {
	if cfg.RetryMaxAttempts < 0 {
		return fmt.Errorf("pgwd: notifications.retry.max_attempts must be >= 0")
	}
	if cfg.RetryInitialBackoff < 0 {
		return fmt.Errorf("pgwd: notifications.retry.initial_backoff must be >= 0")
	}
	if cfg.RetryMaxBackoff < 0 {
		return fmt.Errorf("pgwd: notifications.retry.max_backoff must be >= 0")
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

// ValidateLongQueryAlerts ensures long-query alerting is only enabled with a metrics store and sane cooldown.
func ValidateLongQueryAlerts(cfg *config.Config) error {
	if cfg.LongQueryMinSeconds <= 0 {
		return nil
	}
	if metricsstore.Driver(cfg) == "" {
		return fmt.Errorf("pgwd: long-query alerts require a metrics store (sqlite.path or metrics_store.driver + metrics_store.dsn) for notification cooldown")
	}
	if cfg.LongQueryCooldownSeconds <= 0 {
		return fmt.Errorf("pgwd: long-query alerts require db.long_query_cooldown_seconds > 0 (or omit to use default 3600)")
	}
	if cfg.LongQueryMinCount <= 0 {
		return fmt.Errorf("pgwd: long-query alerts require db.long_query_min_count > 0 (or omit to use default 1)")
	}
	return nil
}
