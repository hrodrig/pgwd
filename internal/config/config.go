package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all pgwd settings from CLI and env (PGWD_*).
type Config struct {
	// Database
	DBURL string

	// Kubernetes: connect to Postgres via kubectl port-forward (optional)
	KubePostgres          string // e.g. "default/svc/postgres" or "default/pod/postgres-0"
	KubeLocalPort         int    // local port for port-forward (default 5432)
	KubePasswordVar       string // pod env var for password when URL has DISCOVER_MY_PASSWORD (default POSTGRES_PASSWORD)
	KubePasswordContainer string // container name in pod if not default

	// Optional context for notifications (Slack health-check style): cluster name, client (service/pod or hostname).
	// When -kube-postgres is set, Client and namespace are derived from it; Cluster can be detected from kubeconfig or set via PGWD_CLUSTER.
	Cluster string
	Client  string

	// Thresholds (0 = disabled)
	ThresholdTotal  int
	ThresholdActive int
	ThresholdIdle   int
	StaleAge        int // seconds; connections open longer than this are "stale"
	ThresholdStale  int // alert when count of stale connections >= this

	// Notifications
	SlackWebhook string
	LokiURL      string
	LokiLabels   string // comma-separated key=value

	// Behavior
	Interval                int // seconds; 0 = run once
	DryRun                  bool
	ForceNotification       bool // send a test notification regardless of thresholds (to validate delivery/format)
	NotifyOnConnectFailure  bool // when Postgres connection fails, send an alert to notifiers (infrastructure alert)
	DefaultThresholdPercent int  // when threshold-total/active are 0, set to this % of max_connections (1-100, default 80)
	// TestMaxConnections: if > 0, use instead of server max_connections for defaults and display (for testing alerts).
	TestMaxConnections int
}

func env(key, def string) string {
	if v := os.Getenv("PGWD_" + key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv("PGWD_" + key)
	if v == "" {
		return def
	}
	n, _ := strconv.Atoi(v)
	return n
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv("PGWD_" + key))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}

// FromEnv builds config from environment variables (PGWD_*).
func FromEnv() Config {
	return Config{
		DBURL:                   env("DB_URL", ""),
		KubePostgres:            env("KUBE_POSTGRES", ""),
		KubeLocalPort:           envInt("KUBE_LOCAL_PORT", 5432),
		KubePasswordVar:         env("KUBE_PASSWORD_VAR", "POSTGRES_PASSWORD"),
		KubePasswordContainer:   env("KUBE_PASSWORD_CONTAINER", ""),
		Cluster:                 env("CLUSTER", ""),
		Client:                  env("CLIENT", ""),
		ThresholdTotal:          envInt("THRESHOLD_TOTAL", 0),
		ThresholdActive:         envInt("THRESHOLD_ACTIVE", 0),
		ThresholdIdle:           envInt("THRESHOLD_IDLE", 0),
		StaleAge:                envInt("STALE_AGE", 0),
		ThresholdStale:          envInt("THRESHOLD_STALE", 0),
		SlackWebhook:            env("SLACK_WEBHOOK", ""),
		LokiURL:                 env("LOKI_URL", ""),
		LokiLabels:              env("LOKI_LABELS", ""),
		Interval:                envInt("INTERVAL", 0),
		DryRun:                  envBool("DRY_RUN", false),
		ForceNotification:       envBool("FORCE_NOTIFICATION", false),
		NotifyOnConnectFailure:  envBool("NOTIFY_ON_CONNECT_FAILURE", false),
		DefaultThresholdPercent: envInt("DEFAULT_THRESHOLD_PERCENT", 80),
		TestMaxConnections:       envInt("TEST_MAX_CONNECTIONS", 0),
	}
}

// OverrideWith sets fields from a set of optional CLI overrides (pointers).
// Non-nil values override the config.
func (c *Config) OverrideWith(overrides struct {
	DBURL                   *string
	ThresholdTotal          *int
	ThresholdActive         *int
	ThresholdIdle           *int
	StaleAge                *int
	ThresholdStale          *int
	SlackWebhook            *string
	LokiURL                 *string
	LokiLabels              *string
	Interval                *int
	DryRun                  *bool
	ForceNotification       *bool
	DefaultThresholdPercent *int
}) {
	if overrides.DBURL != nil {
		c.DBURL = *overrides.DBURL
	}
	if overrides.ThresholdTotal != nil {
		c.ThresholdTotal = *overrides.ThresholdTotal
	}
	if overrides.ThresholdActive != nil {
		c.ThresholdActive = *overrides.ThresholdActive
	}
	if overrides.ThresholdIdle != nil {
		c.ThresholdIdle = *overrides.ThresholdIdle
	}
	if overrides.StaleAge != nil {
		c.StaleAge = *overrides.StaleAge
	}
	if overrides.ThresholdStale != nil {
		c.ThresholdStale = *overrides.ThresholdStale
	}
	if overrides.SlackWebhook != nil {
		c.SlackWebhook = *overrides.SlackWebhook
	}
	if overrides.LokiURL != nil {
		c.LokiURL = *overrides.LokiURL
	}
	if overrides.LokiLabels != nil {
		c.LokiLabels = *overrides.LokiLabels
	}
	if overrides.Interval != nil {
		c.Interval = *overrides.Interval
	}
	if overrides.DryRun != nil {
		c.DryRun = *overrides.DryRun
	}
	if overrides.ForceNotification != nil {
		c.ForceNotification = *overrides.ForceNotification
	}
	if overrides.DefaultThresholdPercent != nil {
		c.DefaultThresholdPercent = *overrides.DefaultThresholdPercent
	}
}

// HasAnyThreshold returns true if at least one threshold is set.
func (c *Config) HasAnyThreshold() bool {
	return c.ThresholdTotal > 0 || c.ThresholdActive > 0 || c.ThresholdIdle > 0 ||
		c.ThresholdStale > 0
}

// HasAnyNotifier returns true if Slack or Loki is configured.
func (c *Config) HasAnyNotifier() bool {
	return c.SlackWebhook != "" || c.LokiURL != ""
}
