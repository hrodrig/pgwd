package config

import (
	"os"
	"strconv"
	"strings"
)

// DefaultThresholdLevels is the default comma-separated percentages for 3-tier alerts (MySQL-style).
const DefaultThresholdLevels = "75,85,95"

// Config holds all pgwd settings from CLI and env (PGWD_*).
type Config struct {
	// Database
	DBURL string

	// Kubernetes: connect to Postgres via kubectl port-forward (optional)
	KubePostgres          string // e.g. "default/svc/postgres" or "default/pod/postgres-0"
	KubeContext           string // kubectl context to use (empty = current context)
	KubeLocalPort         int    // local port for port-forward (default 5432)
	KubePasswordVar       string // pod env var for password when URL has DISCOVER_MY_PASSWORD (default POSTGRES_PASSWORD)
	KubePasswordContainer string // container name in pod if not default

	// Optional context for notifications (Slack health-check style): cluster name, client (service/pod or hostname).
	// When -kube-postgres is set, Client and namespace are derived from it; Cluster can be detected from kubeconfig or set via PGWD_CLUSTER.
	Cluster string
	Client  string

	// Thresholds (0 = disabled)
	ThresholdTotal  int // Deprecated: use ThresholdLevels; will be removed in v1.0.0
	ThresholdActive int // Deprecated: use ThresholdLevels; will be removed in v1.0.0
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
	ForceNotification       bool   // send a test notification regardless of thresholds (to validate delivery/format)
	NotifyOnConnectFailure  bool   // when Postgres connection fails, send an alert to notifiers (infrastructure alert)
	DefaultThresholdPercent int    // when threshold-total/active are set, used for the one left at 0 (1-100, default 80)
	ThresholdLevels         string // comma-separated percentages for 3-tier alerts, e.g. "75,85,95" (attention/alert/danger). Used when both total and active are 0.
	// TestMaxConnections: if > 0, use instead of server max_connections for defaults and display (for testing alerts).
	TestMaxConnections int
	// ValidateK8sAccess: if true, validate kubectl connectivity and list pods, then exit. Uses KubeContext if set.
	ValidateK8sAccess bool
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
		KubeContext:             env("KUBE_CONTEXT", ""),
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
		ThresholdLevels:         env("THRESHOLD_LEVELS", DefaultThresholdLevels),
		TestMaxConnections:      envInt("TEST_MAX_CONNECTIONS", 0),
		ValidateK8sAccess:       envBool("VALIDATE_K8S_ACCESS", false),
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
	ThresholdLevels         *string
}) {
	c.applyOverridesThresholds(overrides.DBURL, overrides.ThresholdTotal, overrides.ThresholdActive, overrides.ThresholdIdle, overrides.StaleAge, overrides.ThresholdStale)
	c.applyOverridesNotifiers(overrides.SlackWebhook, overrides.LokiURL, overrides.LokiLabels)
	c.applyOverridesBehaviour(overrides.Interval, overrides.DryRun, overrides.ForceNotification, overrides.DefaultThresholdPercent, overrides.ThresholdLevels)
}

func (c *Config) applyOverridesThresholds(dbURL *string, total, active, idle, staleAge, stale *int) {
	if dbURL != nil {
		c.DBURL = *dbURL
	}
	if total != nil {
		c.ThresholdTotal = *total
	}
	if active != nil {
		c.ThresholdActive = *active
	}
	if idle != nil {
		c.ThresholdIdle = *idle
	}
	if staleAge != nil {
		c.StaleAge = *staleAge
	}
	if stale != nil {
		c.ThresholdStale = *stale
	}
}

func (c *Config) applyOverridesNotifiers(slack, lokiURL, lokiLabels *string) {
	if slack != nil {
		c.SlackWebhook = *slack
	}
	if lokiURL != nil {
		c.LokiURL = *lokiURL
	}
	if lokiLabels != nil {
		c.LokiLabels = *lokiLabels
	}
}

func (c *Config) applyOverridesBehaviour(interval *int, dryRun, force *bool, percent *int, levels *string) {
	if interval != nil {
		c.Interval = *interval
	}
	if dryRun != nil {
		c.DryRun = *dryRun
	}
	if force != nil {
		c.ForceNotification = *force
	}
	if percent != nil {
		c.DefaultThresholdPercent = *percent
	}
	if levels != nil {
		c.ThresholdLevels = *levels
	}
}

// ParseThresholdLevels parses "75,85,95" into [75, 85, 95]. Returns nil if empty or invalid.
// Each value must be 1-100 and in ascending order.
func ParseThresholdLevels(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 1 || n > 100 {
			return nil
		}
		if len(out) > 0 && n <= out[len(out)-1] {
			return nil
		}
		out = append(out, n)
	}
	if len(out) < 3 {
		return nil
	}
	return out
}

// UsesLevelMode returns true when both threshold-total and threshold-active are 0 and ThresholdLevels is valid (3+ percentages).
func (c *Config) UsesLevelMode() bool {
	return c.ThresholdTotal == 0 && c.ThresholdActive == 0 && len(ParseThresholdLevels(c.ThresholdLevels)) >= 3
}

// HasAnyThreshold returns true if at least one threshold is set or level mode is active.
func (c *Config) HasAnyThreshold() bool {
	return c.ThresholdTotal > 0 || c.ThresholdActive > 0 || c.ThresholdIdle > 0 ||
		c.ThresholdStale > 0 || c.UsesLevelMode()
}

// HasAnyNotifier returns true if Slack or Loki is configured.
func (c *Config) HasAnyNotifier() bool {
	return c.SlackWebhook != "" || c.LokiURL != ""
}
