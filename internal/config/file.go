package config

import (
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath is the standard config file location.
const DefaultConfigPath = "/etc/pgwd/pgwd.conf"

// fileConfigDB is one entry in the databases array.
type fileConfigDB struct {
	URL                     string `yaml:"url"`
	Client                  string `yaml:"client"`
	StaleAge                int    `yaml:"stale_age"`
	DefaultThresholdPercent int    `yaml:"default_threshold_percent"`
	Threshold               struct {
		Active int    `yaml:"active"`
		Idle   int    `yaml:"idle"`
		Levels string `yaml:"levels"`
		Stale  int    `yaml:"stale"`
		Total  int    `yaml:"total"`
	} `yaml:"threshold"`
	LongQueryMinSeconds      int `yaml:"long_query_min_seconds"`
	LongQueryCooldownSeconds int `yaml:"long_query_cooldown_seconds"`
	LongQueryMinCount        int `yaml:"long_query_min_count"`
}

// fileConfig mirrors the YAML structure: db, databases, kube, notifications, and top-level keys.
type fileConfig struct {
	Client                 string         `yaml:"client"`
	DryRun                 bool           `yaml:"dry_run"`
	LogLevel               string         `yaml:"log_level"`
	Interval               int            `yaml:"interval"`
	Strict                 bool           `yaml:"strict"`
	NotifyOnConnectFailure bool           `yaml:"notify_on_connect_failure"`
	EnableCollector        bool           `yaml:"enable_collector"`
	EnableUpdateCheck      *bool          `yaml:"enable_update_check"`
	Databases              []fileConfigDB `yaml:"databases"`
	DB                     fileConfigDB   `yaml:"db"`
	Sqlite                 struct {
		Path       string `yaml:"path"`
		MaxMetrics int    `yaml:"max_metrics"`
		StaleAge   int    `yaml:"stale_age"`
	} `yaml:"sqlite"`
	MetricsStore struct {
		Driver string `yaml:"driver"`
		DSN    string `yaml:"dsn"`
	} `yaml:"metrics_store"`
	ConfirmAlert int `yaml:"confirm_alert"`
	ConfirmOk    int `yaml:"confirm_ok"`
	HTTP         struct {
		Listen               string `yaml:"listen"`
		BasePath             string `yaml:"base_path"`
		HealthPath           string `yaml:"healthz_path"`
		MetricsPath          string `yaml:"metrics_path"`
		MetricsToken         string `yaml:"metrics_token"`
		MetricsBasicUser     string `yaml:"metrics_basic_user"`
		MetricsBasicPassword string `yaml:"metrics_basic_password"`
	} `yaml:"http"`
	Kube struct {
		Context            string `yaml:"context"`
		LocalPort          int    `yaml:"local_port"`
		Loki               string `yaml:"loki"`
		LokiLocalPort      int    `yaml:"loki_local_port"`
		LokiRemotePort     int    `yaml:"loki_remote_port"`
		PasswordFromSecret struct {
			Namespace string `yaml:"namespace"`
			Name      string `yaml:"name"`
			Key       string `yaml:"key"`
		} `yaml:"password_from_secret"`
		Postgres string `yaml:"postgres"`
	} `yaml:"kube"`
	Notifications struct {
		Loki struct {
			URL         string `yaml:"url"`
			BearerToken string `yaml:"bearer_token"`
			Labels      string `yaml:"labels"`
			OrgID       string `yaml:"org_id"`
		} `yaml:"loki"`
		Slack struct {
			Webhook string `yaml:"webhook"`
		} `yaml:"slack"`
		PagerDuty struct {
			Enabled    bool   `yaml:"enabled"`
			RoutingKey string `yaml:"routing_key"`
			Severity   string `yaml:"severity"`
			Source     string `yaml:"source"`
		} `yaml:"pagerduty"`
		Teams struct {
			Enabled    bool   `yaml:"enabled"`
			WebhookURL string `yaml:"webhook_url"`
		} `yaml:"teams"`
		Generic struct {
			Enabled      bool              `yaml:"enabled"`
			WebhookURL   string            `yaml:"webhook_url"`
			JSONKey      string            `yaml:"json_key"`
			Headers      map[string]string `yaml:"headers"`
			ExtraFields  map[string]string `yaml:"extra_fields"`
			BodyTemplate string            `yaml:"body_template"`
			HMACSecret   string            `yaml:"hmac_secret"`
			HMACHeader   string            `yaml:"hmac_header"`
		} `yaml:"generic"`
		Retry struct {
			MaxAttempts    int    `yaml:"max_attempts"`
			InitialBackoff string `yaml:"initial_backoff"`
			MaxBackoff     string `yaml:"max_backoff"`
		} `yaml:"retry"`
	} `yaml:"notifications"`
}

// FromFile loads config from a YAML file. Returns (Config, loaded, error).
// When path is empty or file does not exist: returns empty Config, loaded=false, nil.
// When file exists and parses: returns Config, loaded=true, nil. When loaded=true,
// env vars (PGWD_*) are not applied; config file is the single source. Use -config
// to specify a custom path.
func FromFile(path string) (Config, bool, error) {
	if path == "" {
		return Config{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return Config{}, false, err
	}
	return fileConfigToConfig(fc), true, nil
}

func fileConfigToConfig(fc fileConfig) Config {
	updateCheck := true
	if fc.EnableUpdateCheck != nil {
		updateCheck = *fc.EnableUpdateCheck
	}
	c := Config{
		DBURL:                   fc.DB.URL,
		Client:                  fc.Client,
		DefaultThresholdPercent: fc.DB.DefaultThresholdPercent,
		DryRun:                  fc.DryRun,
		LogLevel:                fc.LogLevel,
		Interval:                fc.Interval,
		Strict:                  fc.Strict,
		EnableCollector:         fc.EnableCollector,
		EnableUpdateCheck:       updateCheck,
		LoadedFromFile:          true,
		KubePostgres:            fc.Kube.Postgres,
		KubeContext:             fc.Kube.Context,
		KubeLocalPort:           fc.Kube.LocalPort,
		KubeLoki:                fc.Kube.Loki,
		KubeLokiLocalPort:       fc.Kube.LokiLocalPort,
		KubeLokiRemotePort:      fc.Kube.LokiRemotePort,
		KubePasswordFromSecret: KubePasswordFromSecret{
			Namespace: fc.Kube.PasswordFromSecret.Namespace,
			Name:      fc.Kube.PasswordFromSecret.Name,
			Key:       fc.Kube.PasswordFromSecret.Key,
		},
		LokiURL:                  fc.Notifications.Loki.URL,
		LokiLabels:               fc.Notifications.Loki.Labels,
		LokiOrgID:                fc.Notifications.Loki.OrgID,
		LokiBearerToken:          fc.Notifications.Loki.BearerToken,
		NotifyOnConnectFailure:   fc.NotifyOnConnectFailure,
		SqlitePath:               fc.Sqlite.Path,
		SqliteMaxMetrics:         fc.Sqlite.MaxMetrics,
		SqliteStaleAge:           fc.Sqlite.StaleAge,
		MetricsStoreDriver:       fc.MetricsStore.Driver,
		MetricsStoreDSN:          fc.MetricsStore.DSN,
		ConfirmAlert:             fc.ConfirmAlert,
		ConfirmOk:                fc.ConfirmOk,
		HTTPListen:               fc.HTTP.Listen,
		HTTPBasePath:             fc.HTTP.BasePath,
		HTTPHealthPath:           fc.HTTP.HealthPath,
		HTTPMetricsPath:          fc.HTTP.MetricsPath,
		HTTPMetricsToken:         fc.HTTP.MetricsToken,
		HTTPMetricsBasicUser:     fc.HTTP.MetricsBasicUser,
		HTTPMetricsBasicPassword: fc.HTTP.MetricsBasicPassword,
		SlackWebhook:             fc.Notifications.Slack.Webhook,
		PagerDutyEnabled:         fc.Notifications.PagerDuty.Enabled,
		PagerDutyRoutingKey:      fc.Notifications.PagerDuty.RoutingKey,
		PagerDutySeverity:        fc.Notifications.PagerDuty.Severity,
		PagerDutySource:          fc.Notifications.PagerDuty.Source,
		TeamsEnabled:             fc.Notifications.Teams.Enabled,
		TeamsWebhook:             fc.Notifications.Teams.WebhookURL,
		GenericEnabled:           fc.Notifications.Generic.Enabled,
		GenericWebhookURL:        fc.Notifications.Generic.WebhookURL,
		GenericJSONKey:           fc.Notifications.Generic.JSONKey,
		GenericHeaders:           fc.Notifications.Generic.Headers,
		GenericExtraFields:       fc.Notifications.Generic.ExtraFields,
		GenericBodyTemplate:      fc.Notifications.Generic.BodyTemplate,
		GenericHMACSecret:        fc.Notifications.Generic.HMACSecret,
		GenericHMACHeader:        fc.Notifications.Generic.HMACHeader,
		RetryMaxAttempts:         fc.Notifications.Retry.MaxAttempts,
		RetryInitialBackoff:      parseDurationOrZero(fc.Notifications.Retry.InitialBackoff),
		RetryMaxBackoff:          parseDurationOrZero(fc.Notifications.Retry.MaxBackoff),
		StaleAge:                 fc.DB.StaleAge,
		ThresholdTotal:           fc.DB.Threshold.Total,
		ThresholdActive:          fc.DB.Threshold.Active,
		ThresholdIdle:            fc.DB.Threshold.Idle,
		ThresholdStale:           fc.DB.Threshold.Stale,
		ThresholdLevels:          fc.DB.Threshold.Levels,
		LongQueryMinSeconds:      fc.DB.LongQueryMinSeconds,
		LongQueryCooldownSeconds: fc.DB.LongQueryCooldownSeconds,
		LongQueryMinCount:        fc.DB.LongQueryMinCount,
	}

	// Normalize to Databases: use databases if present, else wrap db as single target.
	if len(fc.Databases) > 0 {
		c.Databases = make([]DatabaseTarget, 0, len(fc.Databases))
		for _, d := range fc.Databases {
			t := mergeDBTarget(fc.Client, fc.DB, d)
			c.Databases = append(c.Databases, t)
		}
	} else if fc.DB.URL != "" {
		c.LoadedLegacyDBConfig = true
		log.Printf("pgwd: config key 'db' is deprecated; use 'databases: [{ url: ... }]' instead. Support will be removed in v1.0.")
		t := mergeDBTarget(fc.Client, fc.DB, fc.DB)
		c.Databases = []DatabaseTarget{t}
	}

	ApplyDefaults(&c)
	FinalizeAfterFlags(&c)
	return c
}

// mergeDBTarget builds a DatabaseTarget from base db config and per-entry overrides.
func mergeDBTarget(baseClient string, base, over fileConfigDB) DatabaseTarget {
	t := DatabaseTarget{
		URL:                      over.URL,
		Client:                   over.Client,
		StaleAge:                 orZero(over.StaleAge, base.StaleAge),
		DefaultThresholdPercent:  orZero(over.DefaultThresholdPercent, base.DefaultThresholdPercent),
		ThresholdTotal:           orZero(over.Threshold.Total, base.Threshold.Total),
		ThresholdActive:          orZero(over.Threshold.Active, base.Threshold.Active),
		ThresholdIdle:            orZero(over.Threshold.Idle, base.Threshold.Idle),
		ThresholdStale:           orZero(over.Threshold.Stale, base.Threshold.Stale),
		ThresholdLevels:          orEmpty(over.Threshold.Levels, base.Threshold.Levels),
		LongQueryMinSeconds:      orZero(over.LongQueryMinSeconds, base.LongQueryMinSeconds),
		LongQueryCooldownSeconds: orZero(over.LongQueryCooldownSeconds, base.LongQueryCooldownSeconds),
		LongQueryMinCount:        orZero(over.LongQueryMinCount, base.LongQueryMinCount),
	}
	if t.Client == "" && baseClient != "" {
		t.Client = baseClient + "-" + databaseNameFromURL(t.URL)
	} else if t.Client == "" {
		t.Client = databaseNameFromURL(t.URL)
	}
	// Apply global defaults when base and override both leave zero/empty.
	if t.DefaultThresholdPercent == 0 {
		t.DefaultThresholdPercent = 80
	}
	if t.ThresholdLevels == "" {
		t.ThresholdLevels = DefaultThresholdLevels
	}
	return t
}

func orZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func orEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func databaseNameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "unknown"
	}
	db := strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
	if db == "" {
		return "postgres"
	}
	return db
}

// ApplyDefaults sets default values for fields that are zero. Call after FromFile when no file exists.
func ApplyDefaults(c *Config) {
	applyKubeDefaults(c)
	applyGeneralDefaults(c)
	applyCollectorDefaults(c)
	applyNotifyDefaults(c)
	applySqliteAndConfirmDefaults(c)
	applyHTTPDefaults(c)
}

func applyCollectorDefaults(c *Config) {
	if c.LoadedFromFile {
		return
	}
	if os.Getenv("PGWD_ENABLE_UPDATE_CHECK") == "" {
		c.EnableUpdateCheck = true
	}
}

func applyKubeDefaults(c *Config) {
	if c.KubePasswordFromSecret.Key == "" && c.KubePasswordFromSecret.Name != "" {
		c.KubePasswordFromSecret.Key = "password"
	}
	if c.KubeLocalPort == 0 {
		c.KubeLocalPort = 5432
	}
	if c.KubeLokiLocalPort == 0 {
		c.KubeLokiLocalPort = 3100
	}
	if c.KubeLokiRemotePort == 0 {
		c.KubeLokiRemotePort = 3100
	}
}

func applyGeneralDefaults(c *Config) {
	if c.DefaultThresholdPercent == 0 {
		c.DefaultThresholdPercent = 80
	}
	if c.ThresholdLevels == "" {
		c.ThresholdLevels = DefaultThresholdLevels
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	// Normalize log_level: only info and debug supported
	if c.LogLevel != "debug" && c.LogLevel != "info" {
		c.LogLevel = "info"
	}
}

// FinalizeAfterFlags applies derived defaults that depend on CLI flags (call after flag.Parse).
func FinalizeAfterFlags(c *Config) {
	MergeNotifyJSONFields(c)
	applyNotifyDefaults(c)
	if c.LongQueryMinSeconds > 0 {
		if c.LongQueryMinCount <= 0 {
			c.LongQueryMinCount = 1
		}
		if c.LongQueryCooldownSeconds <= 0 {
			c.LongQueryCooldownSeconds = 3600
		}
	}
}

func applySqliteAndConfirmDefaults(c *Config) {
	if c.SqlitePath != "" && c.SqliteMaxMetrics <= 0 {
		c.SqliteMaxMetrics = 10000
	}
	if c.ConfirmAlert <= 0 {
		c.ConfirmAlert = 1
	}
	if c.ConfirmOk <= 0 {
		c.ConfirmOk = 1
	}
}

func applyHTTPDefaults(c *Config) {
	if c.HTTPListen == "" {
		return
	}
	if c.HTTPBasePath == "" {
		c.HTTPBasePath = "/api/pgwd/v1"
	}
	if c.HTTPHealthPath == "" {
		c.HTTPHealthPath = "/healthz"
	}
	if c.HTTPMetricsPath == "" {
		c.HTTPMetricsPath = "/metrics"
	}
}

func parseDurationOrZero(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
