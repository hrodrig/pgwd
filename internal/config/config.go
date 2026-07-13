package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultThresholdLevels is the default comma-separated percentages for 3-tier alerts (MySQL-style).
const DefaultThresholdLevels = "75,85,95"

// DatabaseTarget is one Postgres instance to monitor. Used when config has multiple databases.
type DatabaseTarget struct {
	URL                      string
	Client                   string // empty = derive from base client + "-" + db name from URL
	StaleAge                 int
	DefaultThresholdPercent  int
	ThresholdTotal           int
	ThresholdActive          int
	ThresholdIdle            int
	ThresholdStale           int
	ThresholdLevels          string
	LongQueryMinSeconds      int
	LongQueryCooldownSeconds int
	LongQueryMinCount        int
}

// KubePasswordFromSecret loads the DB password or full DSN from a Kubernetes Secret (read-only API; no pods/exec).
type KubePasswordFromSecret struct {
	Namespace string
	Name      string
	Key       string // default "password"; use "url" for a full postgres:// DSN in the Secret
}

// Config holds all pgwd settings from CLI and env (PGWD_*).
type Config struct {
	// Database (single-DB mode; used when Databases is empty)
	DBURL string
	// Databases (multi-DB mode); when non-empty, DBURL is ignored. From file only.
	Databases []DatabaseTarget

	// Kubernetes: connect to Postgres via port-forward (client-go, optional)
	KubePostgres           string                 // e.g. "default/svc/postgres" or "default/pod/postgres-0"
	KubeContext            string                 // kubeconfig context to use (empty = current context)
	KubeLocalPort          int                    // local port for port-forward (default 5432)
	KubePasswordFromSecret KubePasswordFromSecret // optional: read password or full URL from Secret (no pods/exec)
	// Kubernetes: connect to Loki via port-forward when Loki is inside the cluster (optional)
	KubeLoki           string // e.g. "monitoring/svc/loki" — same format as kube-postgres
	KubeLokiLocalPort  int    // local port for Loki port-forward (default 3100)
	KubeLokiRemotePort int    // remote port on the Loki service (default 3100)

	// Optional context for notifications (Slack health-check style): client (custom name for this monitor).
	// Cluster is computed from kubeconfig when -kube-postgres is set; not configurable.
	Client string

	// Thresholds (0 = disabled)
	ThresholdTotal  int // Deprecated: use ThresholdLevels; will be removed in v1.0.0
	ThresholdActive int // Deprecated: use ThresholdLevels; will be removed in v1.0.0
	ThresholdIdle   int
	StaleAge        int // seconds; connections open longer than this are "stale"
	ThresholdStale  int // alert when count of stale connections >= this
	// Long-running query alerts (state=active, query_start age). Requires a metrics store for cooldown timestamps.
	LongQueryMinSeconds      int // 0 = disabled; min query runtime in seconds to count as "long"
	LongQueryCooldownSeconds int // min time between long_query notifications per target (default when min set: 3600)
	LongQueryMinCount        int // alert when count of long-running queries >= this (default 1)

	// Notifications
	SlackWebhook    string
	LokiURL         string
	LokiLabels      string // comma-separated key=value
	LokiOrgID       string // X-Scope-OrgID header (Loki multi-tenancy); empty = not set
	LokiBearerToken string // Authorization: Bearer <token>; empty = not set

	PagerDutyEnabled    bool
	PagerDutyRoutingKey string
	PagerDutySeverity   string // default "warning"
	PagerDutySource     string // default "pgwd"

	TeamsEnabled bool
	TeamsWebhook string

	GenericEnabled         bool
	GenericWebhookURL      string
	GenericJSONKey         string // default "text"
	GenericHeaders         map[string]string
	GenericExtraFields     map[string]string
	GenericBodyTemplate    string
	GenericHMACSecret      string
	GenericHMACHeader      string // default "X-Pgwd-Signature"
	GenericHeadersJSON     string // CLI/env JSON; merged in FinalizeAfterFlags
	GenericExtraFieldsJSON string // CLI/env JSON; merged in FinalizeAfterFlags

	RetryMaxAttempts    int
	RetryInitialBackoff time.Duration
	RetryMaxBackoff     time.Duration

	// Behavior
	Interval                int    // seconds; 0 = run once
	LogLevel                string // "info" (default) or "debug"; debug = verbose dry-run stats
	DryRun                  bool
	Strict                  bool   // exit 4 when notifier delivery fails for a threshold event
	ForceNotification       bool   // send a test notification regardless of thresholds (to validate delivery/format)
	NotifyOnConnectFailure  bool   // when Postgres connection fails, send an alert to notifiers (infrastructure alert)
	DefaultThresholdPercent int    // when threshold-total/active are set, used for the one left at 0 (1-100, default 80)
	ThresholdLevels         string // comma-separated percentages for 3-tier alerts, e.g. "75,85,95" (attention/alert/danger). Used when both total and active are 0.
	// TestMaxConnections: if > 0, use instead of server max_connections for defaults and display (for testing alerts).
	TestMaxConnections int
	// ValidateK8sAccess: if true, validate cluster connectivity and list pods, then exit. Uses KubeContext if set.
	ValidateK8sAccess bool

	// SQLite: persistent store for metrics (resolution notifications, /metrics). Optional.
	SqlitePath       string // e.g. /var/lib/pgwd/pgwd.db
	SqliteMaxMetrics int    // max rows; FIFO eviction when exceeded (default 10000)
	SqliteStaleAge   int    // seconds for stale count in store; 0 = use db.stale_age or 0 (independent of alert stale)
	// MetricsStoreDriver / MetricsStoreDSN: PostgreSQL or MySQL metrics backend (optional; see internal/store/sqlstore.go).
	// Empty driver + sqlite.path implies sqlite. FIFO cap uses sqlite.max_metrics for all backends.
	MetricsStoreDriver string
	MetricsStoreDSN    string
	// ExportMetricsFormat + ExportMetricsDestination: one-shot export via internal/metricsstore + internal/metricsexport.
	// Format "csv" writes destination as a file path.
	ExportMetricsFormat      string
	ExportMetricsDestination string

	// Hysteresis: require N consecutive checks before alert/resolution (avoids brief spikes/false recoveries).
	ConfirmAlert int // consecutive "bad" checks before sending alert (default 1)
	ConfirmOk    int // consecutive "ok" checks before resolution notification (default 1)

	LoadedLegacyDBConfig     bool   // set when YAML used deprecated top-level db: (not databases:)
	HTTPListen               string // e.g. ":8080"; empty = disabled
	HTTPBasePath             string // e.g. "/api/pgwd/v1"; paths relative to this
	HTTPHealthPath           string // e.g. "/healthz" → base_path + health_path
	HTTPMetricsPath          string // e.g. "/metrics" → base_path + metrics_path
	HTTPMetricsToken         string // optional; when set, /metrics requires Bearer token or ?token=
	HTTPMetricsBasicUser     string // optional basic auth user for /metrics only
	HTTPMetricsBasicPassword string // optional basic auth password for /metrics only
}

// ConfigPath returns the config file path: -config flag, PGWD_CONFIG, or DefaultConfigPath.
// Call before flag.Parse for other flags; parses -config and --config from os.Args.
func ConfigPath() string {
	if v := os.Getenv("PGWD_CONFIG"); v != "" {
		return v
	}
	for i := 1; i < len(os.Args)-1; i++ {
		if (os.Args[i] == "-config" || os.Args[i] == "--config") && os.Args[i+1] != "" {
			return os.Args[i+1]
		}
	}
	return DefaultConfigPath
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

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv("PGWD_" + key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envJSONMap(key string) map[string]string {
	v := os.Getenv("PGWD_" + key)
	if v == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(v), &m); err != nil {
		return nil
	}
	return m
}

// ApplyEnv overrides cfg with environment variables (PGWD_*) when set.
// Call after loading from file; CLI flags override env.
func ApplyEnv(cfg *Config) {
	applyEnvDBAndContext(cfg)
	applyEnvKube(cfg)
	applyEnvThresholds(cfg)
	applyEnvNotifiers(cfg)
	applyEnvSqliteAndHTTP(cfg)
	applyEnvMetricsStoreAndExport(cfg)
	applyEnvBehaviour(cfg)
}

func applyEnvSqliteAndHTTP(cfg *Config) {
	if v := env("SQLITE_PATH", ""); v != "" {
		cfg.SqlitePath = v
	}
	if v := envInt("SQLITE_MAX_METRICS", -1); v >= 0 {
		cfg.SqliteMaxMetrics = v
	}
	if v := envInt("SQLITE_STALE_AGE", -1); v >= 0 {
		cfg.SqliteStaleAge = v
	}
	if v := envInt("CONFIRM_ALERT", -1); v >= 0 {
		cfg.ConfirmAlert = v
	}
	if v := envInt("CONFIRM_OK", -1); v >= 0 {
		cfg.ConfirmOk = v
	}
	applyEnvHTTP(cfg)
}

func applyEnvHTTP(cfg *Config) {
	if v := env("HTTP_LISTEN", ""); v != "" {
		cfg.HTTPListen = v
		// Apply defaults for paths when only HTTP_LISTEN is set
		if cfg.HTTPBasePath == "" {
			cfg.HTTPBasePath = "/api/pgwd/v1"
		}
		if cfg.HTTPHealthPath == "" {
			cfg.HTTPHealthPath = "/healthz"
		}
		if cfg.HTTPMetricsPath == "" {
			cfg.HTTPMetricsPath = "/metrics"
		}
	}
	if v := env("HTTP_BASE_PATH", ""); v != "" {
		cfg.HTTPBasePath = v
	}
	if v := env("HTTP_HEALTHZ_PATH", ""); v != "" {
		cfg.HTTPHealthPath = v
	}
	if v := env("HTTP_METRICS_PATH", ""); v != "" {
		cfg.HTTPMetricsPath = v
	}
	if v := env("HTTP_METRICS_TOKEN", ""); v != "" {
		cfg.HTTPMetricsToken = v
	}
	if v := env("HTTP_METRICS_BASIC_USER", ""); v != "" {
		cfg.HTTPMetricsBasicUser = v
	}
	if v := env("HTTP_METRICS_BASIC_PASSWORD", ""); v != "" {
		cfg.HTTPMetricsBasicPassword = v
	}
}

func applyEnvMetricsStoreAndExport(cfg *Config) {
	if v := env("METRICS_STORE_DRIVER", ""); v != "" {
		cfg.MetricsStoreDriver = v
	}
	if v := env("METRICS_STORE_DSN", ""); v != "" {
		cfg.MetricsStoreDSN = v
	}
	if v := env("EXPORT_METRICS_FORMAT", ""); v != "" {
		cfg.ExportMetricsFormat = v
	}
	if v := env("EXPORT_METRICS_DESTINATION", ""); v != "" {
		cfg.ExportMetricsDestination = v
	}
}

func applyEnvDBAndContext(cfg *Config) {
	if v := env("DB_URL", ""); v != "" {
		cfg.DBURL = v
	}
	if v := env("CLIENT", ""); v != "" {
		cfg.Client = v
	}
}

func applyEnvKube(cfg *Config) {
	if v := env("KUBE_POSTGRES", ""); v != "" {
		cfg.KubePostgres = v
	}
	if v := env("KUBE_CONTEXT", ""); v != "" {
		cfg.KubeContext = v
	}
	if v := envInt("KUBE_LOCAL_PORT", -1); v >= 0 {
		cfg.KubeLocalPort = v
	}
	if v := env("KUBE_LOKI", ""); v != "" {
		cfg.KubeLoki = v
	}
	if v := envInt("KUBE_LOKI_LOCAL_PORT", -1); v >= 0 {
		cfg.KubeLokiLocalPort = v
	}
	if v := envInt("KUBE_LOKI_REMOTE_PORT", -1); v >= 0 {
		cfg.KubeLokiRemotePort = v
	}
}

func applyEnvThresholds(cfg *Config) {
	if v := envInt("DB_THRESHOLD_TOTAL", -1); v >= 0 {
		cfg.ThresholdTotal = v
	}
	if v := envInt("DB_THRESHOLD_ACTIVE", -1); v >= 0 {
		cfg.ThresholdActive = v
	}
	if v := envInt("DB_THRESHOLD_IDLE", -1); v >= 0 {
		cfg.ThresholdIdle = v
	}
	if v := envInt("DB_STALE_AGE", -1); v >= 0 {
		cfg.StaleAge = v
	}
	if v := envInt("DB_THRESHOLD_STALE", -1); v >= 0 {
		cfg.ThresholdStale = v
	}
	if v := env("DB_THRESHOLD_LEVELS", ""); v != "" {
		cfg.ThresholdLevels = v
	}
	if v := envInt("DB_DEFAULT_THRESHOLD_PERCENT", -1); v >= 0 {
		cfg.DefaultThresholdPercent = v
	}
	if v := envInt("DB_LONG_QUERY_MIN_SECONDS", -1); v >= 0 {
		cfg.LongQueryMinSeconds = v
	}
	if v := envInt("DB_LONG_QUERY_COOLDOWN_SECONDS", -1); v >= 0 {
		cfg.LongQueryCooldownSeconds = v
	}
	if v := envInt("DB_LONG_QUERY_MIN_COUNT", -1); v >= 0 {
		cfg.LongQueryMinCount = v
	}
}

func applyEnvNotifiers(cfg *Config) {
	applyEnvSlackLoki(cfg)
	applyEnvPagerDuty(cfg)
	applyEnvTeams(cfg)
	applyEnvGenericWebhook(cfg)
	applyEnvNotifyRetry(cfg)
}

func applyEnvBehaviour(cfg *Config) {
	if v, ok := os.LookupEnv("PGWD_INTERVAL"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Interval = n
		}
	}
	if _, ok := os.LookupEnv("PGWD_DRY_RUN"); ok {
		cfg.DryRun = envBool("DRY_RUN", false)
	}
	if _, ok := os.LookupEnv("PGWD_STRICT"); ok {
		cfg.Strict = envBool("STRICT", false)
	}
	if _, ok := os.LookupEnv("PGWD_FORCE_NOTIFICATION"); ok {
		cfg.ForceNotification = envBool("FORCE_NOTIFICATION", false)
	}
	if _, ok := os.LookupEnv("PGWD_NOTIFY_ON_CONNECT_FAILURE"); ok {
		cfg.NotifyOnConnectFailure = envBool("NOTIFY_ON_CONNECT_FAILURE", false)
	}
	if v := envInt("TEST_MAX_CONNECTIONS", -1); v >= 0 {
		cfg.TestMaxConnections = v
	}
	if _, ok := os.LookupEnv("PGWD_VALIDATE_K8S_ACCESS"); ok {
		cfg.ValidateK8sAccess = envBool("VALIDATE_K8S_ACCESS", false)
	}
	if v := env("LOG_LEVEL", ""); v != "" {
		cfg.LogLevel = v
		if cfg.LogLevel != "debug" && cfg.LogLevel != "info" {
			cfg.LogLevel = "info"
		}
	}
}

// FromEnv builds config from environment variables (PGWD_*).
func FromEnv() Config {
	return Config{
		DBURL:                    env("DB_URL", ""),
		KubePostgres:             env("KUBE_POSTGRES", ""),
		KubeContext:              env("KUBE_CONTEXT", ""),
		KubeLocalPort:            envInt("KUBE_LOCAL_PORT", 5432),
		KubeLoki:                 env("KUBE_LOKI", ""),
		KubeLokiLocalPort:        envInt("KUBE_LOKI_LOCAL_PORT", 3100),
		KubeLokiRemotePort:       envInt("KUBE_LOKI_REMOTE_PORT", 3100),
		Client:                   env("CLIENT", ""),
		ThresholdTotal:           envInt("DB_THRESHOLD_TOTAL", 0),
		ThresholdActive:          envInt("DB_THRESHOLD_ACTIVE", 0),
		ThresholdIdle:            envInt("DB_THRESHOLD_IDLE", 0),
		StaleAge:                 envInt("DB_STALE_AGE", 0),
		ThresholdStale:           envInt("DB_THRESHOLD_STALE", 0),
		SlackWebhook:             env("NOTIFICATIONS_SLACK_WEBHOOK", ""),
		LokiURL:                  env("NOTIFICATIONS_LOKI_URL", ""),
		LokiLabels:               env("NOTIFICATIONS_LOKI_LABELS", ""),
		LokiOrgID:                env("NOTIFICATIONS_LOKI_ORG_ID", ""),
		LokiBearerToken:          env("NOTIFICATIONS_LOKI_BEARER_TOKEN", ""),
		Interval:                 envInt("INTERVAL", 0),
		LogLevel:                 env("LOG_LEVEL", "info"),
		DryRun:                   envBool("DRY_RUN", false),
		ForceNotification:        envBool("FORCE_NOTIFICATION", false),
		NotifyOnConnectFailure:   envBool("NOTIFY_ON_CONNECT_FAILURE", false),
		DefaultThresholdPercent:  envInt("DB_DEFAULT_THRESHOLD_PERCENT", 80),
		ThresholdLevels:          env("DB_THRESHOLD_LEVELS", DefaultThresholdLevels),
		TestMaxConnections:       envInt("TEST_MAX_CONNECTIONS", 0),
		ValidateK8sAccess:        envBool("VALIDATE_K8S_ACCESS", false),
		SqlitePath:               env("SQLITE_PATH", ""),
		SqliteMaxMetrics:         envInt("SQLITE_MAX_METRICS", 0),
		SqliteStaleAge:           envInt("SQLITE_STALE_AGE", 0),
		ConfirmAlert:             envInt("CONFIRM_ALERT", 1),
		ConfirmOk:                envInt("CONFIRM_OK", 1),
		HTTPListen:               env("HTTP_LISTEN", ""),
		HTTPBasePath:             env("HTTP_BASE_PATH", ""),
		HTTPHealthPath:           env("HTTP_HEALTHZ_PATH", ""),
		HTTPMetricsPath:          env("HTTP_METRICS_PATH", ""),
		MetricsStoreDriver:       env("METRICS_STORE_DRIVER", ""),
		MetricsStoreDSN:          env("METRICS_STORE_DSN", ""),
		ExportMetricsFormat:      env("EXPORT_METRICS_FORMAT", ""),
		ExportMetricsDestination: env("EXPORT_METRICS_DESTINATION", ""),
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
		c.ThresholdStale > 0 || c.UsesLevelMode() || c.LongQueryMinSeconds > 0
}

// HasAnyNotifier returns true if at least one notification channel is configured.
func (c *Config) HasAnyNotifier() bool {
	return c.SlackWebhook != "" || c.LokiURL != "" || c.KubeLoki != "" ||
		c.PagerDutyActive() || c.TeamsActive() || c.GenericActive()
}

// PagerDutyActive reports whether PagerDuty notifications are enabled.
func (c *Config) PagerDutyActive() bool {
	return c.PagerDutyEnabled || c.PagerDutyRoutingKey != ""
}

// TeamsActive reports whether Microsoft Teams notifications are enabled.
func (c *Config) TeamsActive() bool {
	return c.TeamsEnabled || c.TeamsWebhook != ""
}

// GenericActive reports whether generic webhook notifications are enabled.
func (c *Config) GenericActive() bool {
	return c.GenericEnabled || c.GenericWebhookURL != ""
}

// Targets returns the database targets to monitor. When Databases is non-empty, returns those.
// Otherwise returns a single target built from DBURL and base config (single-DB mode).
func (c *Config) Targets() []DatabaseTarget {
	if len(c.Databases) > 0 {
		return c.Databases
	}
	return []DatabaseTarget{{
		URL:                      c.DBURL,
		Client:                   c.Client,
		StaleAge:                 c.StaleAge,
		DefaultThresholdPercent:  c.DefaultThresholdPercent,
		ThresholdTotal:           c.ThresholdTotal,
		ThresholdActive:          c.ThresholdActive,
		ThresholdIdle:            c.ThresholdIdle,
		ThresholdStale:           c.ThresholdStale,
		ThresholdLevels:          c.ThresholdLevels,
		LongQueryMinSeconds:      c.LongQueryMinSeconds,
		LongQueryCooldownSeconds: c.LongQueryCooldownSeconds,
		LongQueryMinCount:        c.LongQueryMinCount,
	}}
}

// UsesDatabases returns true when config has multiple database targets (from databases: in YAML).
func (c *Config) UsesDatabases() bool {
	return len(c.Databases) > 0
}

// ConfigForTarget returns a Config with base values and target-specific overrides for one check.
// Callers must not modify the returned config's non-target fields (notifications, etc.).
func (c *Config) ConfigForTarget(t DatabaseTarget) *Config {
	out := *c
	out.DBURL = t.URL
	out.Client = t.Client
	out.StaleAge = t.StaleAge
	out.DefaultThresholdPercent = t.DefaultThresholdPercent
	out.ThresholdTotal = t.ThresholdTotal
	out.ThresholdActive = t.ThresholdActive
	out.ThresholdIdle = t.ThresholdIdle
	out.ThresholdStale = t.ThresholdStale
	out.ThresholdLevels = t.ThresholdLevels
	out.LongQueryMinSeconds = t.LongQueryMinSeconds
	out.LongQueryCooldownSeconds = t.LongQueryCooldownSeconds
	out.LongQueryMinCount = t.LongQueryMinCount
	return &out
}
