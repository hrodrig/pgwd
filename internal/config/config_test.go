package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(key, value string) func() {
	old := os.Getenv(key)
	os.Setenv(key, value)
	return func() {
		if old == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, old)
		}
	}
}

func TestFromEnv_Defaults(t *testing.T) {
	// Clear pgwd-related env so we get real defaults
	prefixes := []string{"PGWD_DB_URL", "PGWD_KUBE_POSTGRES", "PGWD_KUBE_LOKI", "PGWD_KUBE_LOCAL_PORT", "PGWD_KUBE_LOKI_LOCAL_PORT", "PGWD_KUBE_LOKI_REMOTE_PORT", "PGWD_KUBE_PASSWORD_VAR", "PGWD_KUBE_PASSWORD_CONTAINER",
		"PGWD_DB_THRESHOLD_IDLE",
		"PGWD_DB_STALE_AGE", "PGWD_DB_THRESHOLD_STALE", "PGWD_NOTIFICATIONS_SLACK_WEBHOOK", "PGWD_NOTIFICATIONS_LOKI_URL", "PGWD_NOTIFICATIONS_LOKI_LABELS", "PGWD_NOTIFICATIONS_LOKI_ORG_ID", "PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN",
		"PGWD_INTERVAL", "PGWD_DRY_RUN", "PGWD_FORCE_NOTIFICATION", "PGWD_DB_DEFAULT_THRESHOLD_PERCENT", "PGWD_VALIDATE_K8S_ACCESS"}
	for _, p := range prefixes {
		os.Unsetenv(p)
	}
	cfg := FromEnv()
	if cfg.DBURL != "" {
		t.Errorf("DBURL default: got %q", cfg.DBURL)
	}
	if cfg.ThresholdIdle != 0 {
		t.Errorf("threshold defaults: idle=%d", cfg.ThresholdIdle)
	}
	if cfg.Interval != 0 {
		t.Errorf("Interval default: got %d", cfg.Interval)
	}
	if cfg.DefaultThresholdPercent != 80 {
		t.Errorf("DefaultThresholdPercent default: got %d", cfg.DefaultThresholdPercent)
	}
	if cfg.DryRun || cfg.ForceNotification || cfg.ValidateK8sAccess {
		t.Errorf("DryRun=%v ForceNotification=%v ValidateK8sAccess=%v", cfg.DryRun, cfg.ForceNotification, cfg.ValidateK8sAccess)
	}
}

func TestFromEnv_ValidateK8sAccess(t *testing.T) {
	defer setEnv("PGWD_VALIDATE_K8S_ACCESS", "true")()
	cfg := FromEnv()
	if !cfg.ValidateK8sAccess {
		t.Error("ValidateK8sAccess: expected true when PGWD_VALIDATE_K8S_ACCESS=true")
	}
}

func TestFromEnv_Values(t *testing.T) {
	defer setEnv("PGWD_DB_URL", "postgres://localhost/mydb")()
	defer setEnv("PGWD_INTERVAL", "120")()
	defer setEnv("PGWD_DB_DEFAULT_THRESHOLD_PERCENT", "70")()
	defer setEnv("PGWD_DRY_RUN", "true")()
	cfg := FromEnv()
	if cfg.DBURL != "postgres://localhost/mydb" {
		t.Errorf("DBURL: got %q", cfg.DBURL)
	}
	if cfg.Interval != 120 {
		t.Errorf("Interval: got %d", cfg.Interval)
	}
	if cfg.DefaultThresholdPercent != 70 {
		t.Errorf("DefaultThresholdPercent: got %d", cfg.DefaultThresholdPercent)
	}
	if !cfg.DryRun {
		t.Error("DryRun: expected true")
	}
}

func TestHasAnyThreshold(t *testing.T) {
	tests := []struct {
		name string
		c    Config
		want bool
	}{
		{"none", Config{}, false},
		{"idle", Config{ThresholdIdle: 40}, true},
		{"stale", Config{ThresholdStale: 1}, true},
		{"level mode", Config{ThresholdLevels: "75,85,95"}, true},
		{"all", Config{ThresholdIdle: 1, ThresholdStale: 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.HasAnyThreshold(); got != tt.want {
				t.Errorf("HasAnyThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseThresholdLevels(t *testing.T) {
	tests := []struct {
		s    string
		want []int
	}{
		{"75,85,95", []int{75, 85, 95}},
		{"70,80,90", []int{70, 80, 90}},
		{" 75 , 85 , 95 ", []int{75, 85, 95}},
		{"", nil},
		{"75", nil},
		{"75,85", nil},
		{"75,85,95,99", []int{75, 85, 95, 99}},
		{"75,70,95", nil},
		{"0,85,95", nil},
		{"75,101,95", nil},
	}
	for _, tt := range tests {
		got := ParseThresholdLevels(tt.s)
		if len(got) != len(tt.want) {
			t.Errorf("ParseThresholdLevels(%q) = %v, want %v", tt.s, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParseThresholdLevels(%q) = %v, want %v", tt.s, got, tt.want)
				break
			}
		}
	}
}

func TestUsesLevelMode(t *testing.T) {
	tests := []struct {
		c    Config
		want bool
	}{
		{Config{ThresholdLevels: "75,85,95"}, true},
		{Config{ThresholdLevels: "75,85"}, false},
		{Config{ThresholdLevels: ""}, false},
	}
	for _, tt := range tests {
		if got := tt.c.UsesLevelMode(); got != tt.want {
			t.Errorf("UsesLevelMode() for %+v = %v, want %v", tt.c, got, tt.want)
		}
	}
}

func TestHasAnyNotifier(t *testing.T) {
	tests := []struct {
		name string
		c    Config
		want bool
	}{
		{"none", Config{}, false},
		{"slack", Config{SlackWebhook: "https://hooks.slack.com/..."}, true},
		{"loki", Config{LokiURL: "http://loki:3100/push"}, true},
		{"kube-loki", Config{KubeLoki: "monitoring/svc/loki"}, true},
		{"both", Config{SlackWebhook: "x", LokiURL: "y"}, true},
		{"pagerduty", Config{PagerDutyRoutingKey: "rk"}, true},
		{"pagerduty-enabled", Config{PagerDutyEnabled: true, PagerDutyRoutingKey: "rk"}, true},
		{"teams", Config{TeamsWebhook: "https://teams.example/hook"}, true},
		{"generic", Config{GenericWebhookURL: "https://api.example/hook"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.HasAnyNotifier(); got != tt.want {
				t.Errorf("HasAnyNotifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverrideWith(t *testing.T) {
	c := Config{
		DBURL:                   "postgres://old",
		ThresholdIdle:           10,
		SlackWebhook:            "https://old",
		DefaultThresholdPercent: 80,
	}
	db := "postgres://new"
	percent := 90
	c.OverrideWith(struct {
		DBURL                   *string
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
	}{
		DBURL: &db, DefaultThresholdPercent: &percent,
	})
	if c.DBURL != "postgres://new" {
		t.Errorf("DBURL after override: got %q", c.DBURL)
	}
	if c.DefaultThresholdPercent != 90 {
		t.Errorf("DefaultThresholdPercent after override: got %d", c.DefaultThresholdPercent)
	}
	if c.SlackWebhook != "https://old" {
		t.Errorf("SlackWebhook should be unchanged when nil override: got %q", c.SlackWebhook)
	}
}

// ---------------------------------------------------------------------------
// ApplyEnv
// ---------------------------------------------------------------------------

func TestApplyEnv_DBAndContext(t *testing.T) {
	t.Setenv("PGWD_DB_URL", "postgres://envhost/envdb")
	t.Setenv("PGWD_CLIENT", "my-client")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.DBURL != "postgres://envhost/envdb" {
		t.Errorf("DBURL: got %q", cfg.DBURL)
	}
	if cfg.Client != "my-client" {
		t.Errorf("Client: got %q", cfg.Client)
	}
}

func TestApplyEnv_Kube(t *testing.T) {
	t.Setenv("PGWD_KUBE_POSTGRES", "ns/svc/pg")
	t.Setenv("PGWD_KUBE_CONTEXT", "staging")
	t.Setenv("PGWD_KUBE_LOCAL_PORT", "6543")
	t.Setenv("PGWD_KUBE_LOKI", "monitoring/svc/loki")
	t.Setenv("PGWD_KUBE_LOKI_LOCAL_PORT", "3200")
	t.Setenv("PGWD_KUBE_LOKI_REMOTE_PORT", "3201")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.KubePostgres != "ns/svc/pg" {
		t.Errorf("KubePostgres: got %q", cfg.KubePostgres)
	}
	if cfg.KubeContext != "staging" {
		t.Errorf("KubeContext: got %q", cfg.KubeContext)
	}
	if cfg.KubeLocalPort != 6543 {
		t.Errorf("KubeLocalPort: got %d", cfg.KubeLocalPort)
	}
	if cfg.KubeLoki != "monitoring/svc/loki" {
		t.Errorf("KubeLoki: got %q", cfg.KubeLoki)
	}
	if cfg.KubeLokiLocalPort != 3200 {
		t.Errorf("KubeLokiLocalPort: got %d", cfg.KubeLokiLocalPort)
	}
	if cfg.KubeLokiRemotePort != 3201 {
		t.Errorf("KubeLokiRemotePort: got %d", cfg.KubeLokiRemotePort)
	}
}

func TestApplyEnv_Thresholds(t *testing.T) {
	t.Setenv("PGWD_DB_THRESHOLD_IDLE", "30")
	t.Setenv("PGWD_DB_STALE_AGE", "3600")
	t.Setenv("PGWD_DB_THRESHOLD_STALE", "5")
	t.Setenv("PGWD_DB_THRESHOLD_LEVELS", "70,80,90")
	t.Setenv("PGWD_DB_DEFAULT_THRESHOLD_PERCENT", "75")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.ThresholdIdle != 30 {
		t.Errorf("ThresholdIdle: got %d", cfg.ThresholdIdle)
	}
	if cfg.StaleAge != 3600 {
		t.Errorf("StaleAge: got %d", cfg.StaleAge)
	}
	if cfg.ThresholdStale != 5 {
		t.Errorf("ThresholdStale: got %d", cfg.ThresholdStale)
	}
	if cfg.ThresholdLevels != "70,80,90" {
		t.Errorf("ThresholdLevels: got %q", cfg.ThresholdLevels)
	}
	if cfg.DefaultThresholdPercent != 75 {
		t.Errorf("DefaultThresholdPercent: got %d", cfg.DefaultThresholdPercent)
	}
}

func TestApplyEnv_Notifiers(t *testing.T) {
	t.Setenv("PGWD_NOTIFICATIONS_SLACK_WEBHOOK", "https://hooks.slack.com/test")
	t.Setenv("PGWD_NOTIFICATIONS_LOKI_URL", "http://loki:3100/loki/api/v1/push")
	t.Setenv("PGWD_NOTIFICATIONS_LOKI_LABELS", "env=prod,app=pgwd")
	t.Setenv("PGWD_NOTIFICATIONS_LOKI_ORG_ID", "tenant-1")
	t.Setenv("PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN", "secret-token")
	t.Setenv("PGWD_NOTIFICATIONS_REPEAT_WHILE_FIRING", "true")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.SlackWebhook != "https://hooks.slack.com/test" {
		t.Errorf("SlackWebhook: got %q", cfg.SlackWebhook)
	}
	if cfg.LokiURL != "http://loki:3100/loki/api/v1/push" {
		t.Errorf("LokiURL: got %q", cfg.LokiURL)
	}
	if cfg.LokiLabels != "env=prod,app=pgwd" {
		t.Errorf("LokiLabels: got %q", cfg.LokiLabels)
	}
	if cfg.LokiOrgID != "tenant-1" {
		t.Errorf("LokiOrgID: got %q", cfg.LokiOrgID)
	}
	if cfg.LokiBearerToken != "secret-token" {
		t.Errorf("LokiBearerToken: got %q", cfg.LokiBearerToken)
	}
	if !cfg.RepeatWhileFiring {
		t.Error("RepeatWhileFiring: expected true")
	}
}

func TestApplyEnv_PagerDutyNotifier(t *testing.T) {
	t.Setenv("PGWD_NOTIFICATIONS_PAGERDUTY_ROUTING_KEY", "pd-rk")
	t.Setenv("PGWD_NOTIFICATIONS_PAGERDUTY_SEVERITY", "critical")
	t.Setenv("PGWD_NOTIFICATIONS_PAGERDUTY_SOURCE", "pgwd-test")
	var cfg Config
	ApplyEnv(&cfg)
	ApplyDefaults(&cfg)
	if !cfg.PagerDutyEnabled || cfg.PagerDutyRoutingKey != "pd-rk" {
		t.Fatalf("enabled=%v routing_key=%q", cfg.PagerDutyEnabled, cfg.PagerDutyRoutingKey)
	}
	if cfg.PagerDutySeverity != "critical" || cfg.PagerDutySource != "pgwd-test" {
		t.Fatalf("severity/source: %q / %q", cfg.PagerDutySeverity, cfg.PagerDutySource)
	}
}

func TestApplyEnv_TeamsNotifier(t *testing.T) {
	t.Setenv("PGWD_NOTIFICATIONS_TEAMS_WEBHOOK", "https://teams.example/hook")
	var cfg Config
	ApplyEnv(&cfg)
	ApplyDefaults(&cfg)
	if !cfg.TeamsEnabled || cfg.TeamsWebhook != "https://teams.example/hook" {
		t.Fatalf("enabled=%v webhook=%q", cfg.TeamsEnabled, cfg.TeamsWebhook)
	}
}

func TestApplyEnv_GenericNotifier(t *testing.T) {
	t.Setenv("PGWD_NOTIFICATIONS_GENERIC_WEBHOOK_URL", "https://api.example/hook")
	t.Setenv("PGWD_NOTIFICATIONS_GENERIC_JSON_KEY", "message")
	t.Setenv("PGWD_NOTIFICATIONS_GENERIC_HEADERS", `{"Authorization":"Bearer tok"}`)
	t.Setenv("PGWD_NOTIFICATIONS_GENERIC_EXTRA_FIELDS", `{"source":"pgwd"}`)
	var cfg Config
	ApplyEnv(&cfg)
	ApplyDefaults(&cfg)
	if !cfg.GenericEnabled || cfg.GenericWebhookURL != "https://api.example/hook" {
		t.Fatalf("enabled=%v url=%q", cfg.GenericEnabled, cfg.GenericWebhookURL)
	}
	if cfg.GenericJSONKey != "message" {
		t.Fatalf("json_key=%q", cfg.GenericJSONKey)
	}
	if cfg.GenericHeaders["Authorization"] != "Bearer tok" {
		t.Fatalf("headers=%v", cfg.GenericHeaders)
	}
	if cfg.GenericExtraFields["source"] != "pgwd" {
		t.Fatalf("extra=%v", cfg.GenericExtraFields)
	}
}

func TestApplyEnv_NotifyRetry(t *testing.T) {
	t.Setenv("PGWD_NOTIFICATIONS_RETRY_MAX_ATTEMPTS", "5")
	t.Setenv("PGWD_NOTIFICATIONS_RETRY_INITIAL_BACKOFF", "2s")
	t.Setenv("PGWD_NOTIFICATIONS_RETRY_MAX_BACKOFF", "30s")
	var cfg Config
	ApplyEnv(&cfg)
	ApplyDefaults(&cfg)
	if cfg.RetryMaxAttempts != 5 {
		t.Fatalf("max_attempts=%d", cfg.RetryMaxAttempts)
	}
	if cfg.RetryInitialBackoff != 2*time.Second || cfg.RetryMaxBackoff != 30*time.Second {
		t.Fatalf("backoff initial=%v max=%v", cfg.RetryInitialBackoff, cfg.RetryMaxBackoff)
	}
}

func TestApplyEnv_SqliteAndHTTP(t *testing.T) {
	t.Setenv("PGWD_SQLITE_PATH", "/tmp/pgwd.db")
	t.Setenv("PGWD_SQLITE_MAX_METRICS", "5000")
	t.Setenv("PGWD_SQLITE_STALE_AGE", "900")
	t.Setenv("PGWD_CONFIRM_ALERT", "3")
	t.Setenv("PGWD_CONFIRM_OK", "2")
	t.Setenv("PGWD_HTTP_LISTEN", ":9090")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.SqlitePath != "/tmp/pgwd.db" {
		t.Errorf("SqlitePath: got %q", cfg.SqlitePath)
	}
	if cfg.SqliteMaxMetrics != 5000 {
		t.Errorf("SqliteMaxMetrics: got %d", cfg.SqliteMaxMetrics)
	}
	if cfg.SqliteStaleAge != 900 {
		t.Errorf("SqliteStaleAge: got %d", cfg.SqliteStaleAge)
	}
	if cfg.ConfirmAlert != 3 {
		t.Errorf("ConfirmAlert: got %d", cfg.ConfirmAlert)
	}
	if cfg.ConfirmOk != 2 {
		t.Errorf("ConfirmOk: got %d", cfg.ConfirmOk)
	}
	if cfg.HTTPListen != ":9090" {
		t.Errorf("HTTPListen: got %q", cfg.HTTPListen)
	}
	if cfg.HTTPBasePath != "/api/pgwd/v1" {
		t.Errorf("HTTPBasePath default when listen set: got %q", cfg.HTTPBasePath)
	}
	if cfg.HTTPHealthPath != "/healthz" {
		t.Errorf("HTTPHealthPath default: got %q", cfg.HTTPHealthPath)
	}
	if cfg.HTTPMetricsPath != "/metrics" {
		t.Errorf("HTTPMetricsPath default: got %q", cfg.HTTPMetricsPath)
	}
}

func TestApplyEnv_HTTPPathOverrides(t *testing.T) {
	t.Setenv("PGWD_HTTP_LISTEN", ":8080")
	t.Setenv("PGWD_HTTP_BASE_PATH", "/custom")
	t.Setenv("PGWD_HTTP_HEALTHZ_PATH", "/health")
	t.Setenv("PGWD_HTTP_METRICS_PATH", "/m")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.HTTPBasePath != "/custom" {
		t.Errorf("HTTPBasePath override: got %q", cfg.HTTPBasePath)
	}
	if cfg.HTTPHealthPath != "/health" {
		t.Errorf("HTTPHealthPath override: got %q", cfg.HTTPHealthPath)
	}
	if cfg.HTTPMetricsPath != "/m" {
		t.Errorf("HTTPMetricsPath override: got %q", cfg.HTTPMetricsPath)
	}
}

func TestApplyEnv_Behaviour(t *testing.T) {
	t.Setenv("PGWD_INTERVAL", "60")
	t.Setenv("PGWD_DRY_RUN", "true")
	t.Setenv("PGWD_FORCE_NOTIFICATION", "yes")
	t.Setenv("PGWD_TEST_MAX_CONNECTIONS", "200")
	t.Setenv("PGWD_VALIDATE_K8S_ACCESS", "true")
	t.Setenv("PGWD_LOG_LEVEL", "debug")

	var cfg Config
	ApplyEnv(&cfg)

	if cfg.Interval != 60 {
		t.Errorf("Interval: got %d", cfg.Interval)
	}
	if !cfg.DryRun {
		t.Error("DryRun: expected true")
	}
	if !cfg.ForceNotification {
		t.Error("ForceNotification: expected true")
	}
	if cfg.TestMaxConnections != 200 {
		t.Errorf("TestMaxConnections: got %d", cfg.TestMaxConnections)
	}
	if !cfg.ValidateK8sAccess {
		t.Error("ValidateK8sAccess: expected true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q", cfg.LogLevel)
	}
}

func TestApplyEnv_LogLevelNormalized(t *testing.T) {
	t.Setenv("PGWD_LOG_LEVEL", "warn")
	var cfg Config
	ApplyEnv(&cfg)
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel should normalize to info for invalid value: got %q", cfg.LogLevel)
	}
}

func TestApplyEnv_DoesNotOverrideUnsetVars(t *testing.T) {
	cfg := Config{
		DBURL:         "postgres://keep-me",
		ThresholdIdle: 42,
		SlackWebhook:  "https://keep",
	}
	ApplyEnv(&cfg)

	if cfg.DBURL != "postgres://keep-me" {
		t.Errorf("DBURL should be preserved when env unset: got %q", cfg.DBURL)
	}
	if cfg.ThresholdIdle != 42 {
		t.Errorf("ThresholdIdle should be preserved: got %d", cfg.ThresholdIdle)
	}
	if cfg.SlackWebhook != "https://keep" {
		t.Errorf("SlackWebhook should be preserved: got %q", cfg.SlackWebhook)
	}
}

// ---------------------------------------------------------------------------
// ApplyDefaults
// ---------------------------------------------------------------------------

func TestApplyDefaults_ZeroValue(t *testing.T) {
	var cfg Config
	ApplyDefaults(&cfg)

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"KubeLocalPort", cfg.KubeLocalPort, 5432},
		{"KubeLokiLocalPort", cfg.KubeLokiLocalPort, 3100},
		{"KubeLokiRemotePort", cfg.KubeLokiRemotePort, 3100},
		{"DefaultThresholdPercent", cfg.DefaultThresholdPercent, 80},
		{"ThresholdLevels", cfg.ThresholdLevels, "75,85,95"},
		{"LogLevel", cfg.LogLevel, "info"},
		{"ConfirmAlert", cfg.ConfirmAlert, 1},
		{"ConfirmOk", cfg.ConfirmOk, 1},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestApplyDefaults_PreservesNonZero(t *testing.T) {
	cfg := Config{
		KubePasswordFromSecret:  KubePasswordFromSecret{Name: "pg-secret"},
		KubeLocalPort:           9999,
		KubeLokiLocalPort:       4000,
		KubeLokiRemotePort:      4001,
		DefaultThresholdPercent: 60,
		ThresholdLevels:         "60,70,80",
		LogLevel:                "debug",
		ConfirmAlert:            5,
		ConfirmOk:               3,
	}
	ApplyDefaults(&cfg)

	if cfg.KubePasswordFromSecret.Key != "password" {
		t.Errorf("KubePasswordFromSecret.Key default: got %q", cfg.KubePasswordFromSecret.Key)
	}
	if cfg.KubeLocalPort != 9999 {
		t.Errorf("KubeLocalPort should be preserved: got %d", cfg.KubeLocalPort)
	}
	if cfg.KubeLokiLocalPort != 4000 {
		t.Errorf("KubeLokiLocalPort should be preserved: got %d", cfg.KubeLokiLocalPort)
	}
	if cfg.KubeLokiRemotePort != 4001 {
		t.Errorf("KubeLokiRemotePort should be preserved: got %d", cfg.KubeLokiRemotePort)
	}
	if cfg.DefaultThresholdPercent != 60 {
		t.Errorf("DefaultThresholdPercent should be preserved: got %d", cfg.DefaultThresholdPercent)
	}
	if cfg.ThresholdLevels != "60,70,80" {
		t.Errorf("ThresholdLevels should be preserved: got %q", cfg.ThresholdLevels)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel should be preserved: got %q", cfg.LogLevel)
	}
	if cfg.ConfirmAlert != 5 {
		t.Errorf("ConfirmAlert should be preserved: got %d", cfg.ConfirmAlert)
	}
	if cfg.ConfirmOk != 3 {
		t.Errorf("ConfirmOk should be preserved: got %d", cfg.ConfirmOk)
	}
}

func TestApplyDefaults_HTTPDefaultsWhenListenSet(t *testing.T) {
	cfg := Config{HTTPListen: ":8080"}
	ApplyDefaults(&cfg)

	if cfg.HTTPBasePath != "/api/pgwd/v1" {
		t.Errorf("HTTPBasePath: got %q", cfg.HTTPBasePath)
	}
	if cfg.HTTPHealthPath != "/healthz" {
		t.Errorf("HTTPHealthPath: got %q", cfg.HTTPHealthPath)
	}
	if cfg.HTTPMetricsPath != "/metrics" {
		t.Errorf("HTTPMetricsPath: got %q", cfg.HTTPMetricsPath)
	}
}

func TestApplyDefaults_HTTPNoDefaultsWhenListenEmpty(t *testing.T) {
	var cfg Config
	ApplyDefaults(&cfg)

	if cfg.HTTPBasePath != "" {
		t.Errorf("HTTPBasePath should be empty: got %q", cfg.HTTPBasePath)
	}
	if cfg.HTTPHealthPath != "" {
		t.Errorf("HTTPHealthPath should be empty: got %q", cfg.HTTPHealthPath)
	}
	if cfg.HTTPMetricsPath != "" {
		t.Errorf("HTTPMetricsPath should be empty: got %q", cfg.HTTPMetricsPath)
	}
}

func TestApplyDefaults_SqliteMaxMetricsWhenPathSet(t *testing.T) {
	cfg := Config{SqlitePath: "/tmp/pgwd.db"}
	ApplyDefaults(&cfg)

	if cfg.SqliteMaxMetrics != 10000 {
		t.Errorf("SqliteMaxMetrics default: got %d, want 10000", cfg.SqliteMaxMetrics)
	}
}

func TestApplyDefaults_SqliteMaxMetricsPreserved(t *testing.T) {
	cfg := Config{SqlitePath: "/tmp/pgwd.db", SqliteMaxMetrics: 500}
	ApplyDefaults(&cfg)

	if cfg.SqliteMaxMetrics != 500 {
		t.Errorf("SqliteMaxMetrics should be preserved: got %d", cfg.SqliteMaxMetrics)
	}
}

func TestApplyDefaults_SqliteNoDefaultWithoutPath(t *testing.T) {
	var cfg Config
	ApplyDefaults(&cfg)

	if cfg.SqliteMaxMetrics != 0 {
		t.Errorf("SqliteMaxMetrics should be 0 without path: got %d", cfg.SqliteMaxMetrics)
	}
}

func TestApplyDefaults_InvalidLogLevelNormalized(t *testing.T) {
	cfg := Config{LogLevel: "trace"}
	ApplyDefaults(&cfg)
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel should normalize to info: got %q", cfg.LogLevel)
	}
}

// ---------------------------------------------------------------------------
// ConfigPath
// ---------------------------------------------------------------------------

func TestConfigPath_EnvVar(t *testing.T) {
	t.Setenv("PGWD_CONFIG", "/custom/path/pgwd.conf")
	if got := ConfigPath(); got != "/custom/path/pgwd.conf" {
		t.Errorf("ConfigPath via env: got %q, want /custom/path/pgwd.conf", got)
	}
}

func TestConfigPath_Default(t *testing.T) {
	t.Setenv("PGWD_CONFIG", "")
	os.Unsetenv("PGWD_CONFIG")
	got := ConfigPath()
	if got != DefaultConfigPath {
		t.Errorf("ConfigPath default: got %q, want %q", got, DefaultConfigPath)
	}
}

// ---------------------------------------------------------------------------
// Targets
// ---------------------------------------------------------------------------

func TestTargets_SingleDBMode(t *testing.T) {
	cfg := Config{
		DBURL:                   "postgres://localhost/singledb",
		Client:                  "prod-monitor",
		StaleAge:                300,
		DefaultThresholdPercent: 80,
		ThresholdIdle:           20,
		ThresholdStale:          3,
		ThresholdLevels:         "75,85,95",
	}
	targets := cfg.Targets()

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	tgt := targets[0]
	if tgt.URL != cfg.DBURL {
		t.Errorf("URL: got %q", tgt.URL)
	}
	if tgt.Client != cfg.Client {
		t.Errorf("Client: got %q", tgt.Client)
	}
	if tgt.StaleAge != cfg.StaleAge {
		t.Errorf("StaleAge: got %d", tgt.StaleAge)
	}
	if tgt.DefaultThresholdPercent != cfg.DefaultThresholdPercent {
		t.Errorf("DefaultThresholdPercent: got %d", tgt.DefaultThresholdPercent)
	}
	if tgt.ThresholdIdle != cfg.ThresholdIdle {
		t.Errorf("ThresholdIdle: got %d", tgt.ThresholdIdle)
	}
	if tgt.ThresholdStale != cfg.ThresholdStale {
		t.Errorf("ThresholdStale: got %d", tgt.ThresholdStale)
	}
	if tgt.ThresholdLevels != cfg.ThresholdLevels {
		t.Errorf("ThresholdLevels: got %q", tgt.ThresholdLevels)
	}
}

func TestTargets_MultiDBMode(t *testing.T) {
	dbs := []DatabaseTarget{
		{URL: "postgres://host1/db1", Client: "c1"},
		{URL: "postgres://host2/db2", Client: "c2"},
	}
	cfg := Config{
		DBURL:     "postgres://ignored",
		Databases: dbs,
	}
	targets := cfg.Targets()

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	if targets[0].URL != "postgres://host1/db1" {
		t.Errorf("target[0].URL: got %q", targets[0].URL)
	}
	if targets[1].URL != "postgres://host2/db2" {
		t.Errorf("target[1].URL: got %q", targets[1].URL)
	}
}

func TestTargets_EmptyDatabasesReturnsSingle(t *testing.T) {
	cfg := Config{DBURL: "postgres://host/db"}
	if got := cfg.Targets(); len(got) != 1 {
		t.Errorf("empty Databases should return 1 target, got %d", len(got))
	}
}

func TestUsesDatabases(t *testing.T) {
	tests := []struct {
		name string
		c    Config
		want bool
	}{
		{"empty", Config{}, false},
		{"single-db mode", Config{DBURL: "postgres://h/d"}, false},
		{"multi-db mode", Config{Databases: []DatabaseTarget{{URL: "x"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.UsesDatabases(); got != tt.want {
				t.Errorf("UsesDatabases() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPromoteSingleDatabaseForKube(t *testing.T) {
	t.Run("promotes one entry", func(t *testing.T) {
		c := Config{
			Client:       "base",
			KubePostgres: "ns/svc/pg",
			Databases: []DatabaseTarget{{
				URL:             "postgres://u@localhost:15432/db",
				Client:          "tgt",
				ThresholdLevels: "70,80,90",
			}},
		}
		c.PromoteSingleDatabaseForKube()
		if c.UsesDatabases() {
			t.Fatal("want Databases cleared")
		}
		if c.DBURL != "postgres://u@localhost:15432/db" || c.Client != "tgt" || c.ThresholdLevels != "70,80,90" {
			t.Fatalf("got DBURL=%q client=%q levels=%q", c.DBURL, c.Client, c.ThresholdLevels)
		}
	})
	t.Run("noop without kube", func(t *testing.T) {
		c := Config{Databases: []DatabaseTarget{{URL: "postgres://x"}}}
		c.PromoteSingleDatabaseForKube()
		if !c.UsesDatabases() {
			t.Fatal("want Databases kept")
		}
	})
	t.Run("noop with multiple entries", func(t *testing.T) {
		c := Config{
			KubePostgres: "ns/svc/pg",
			Databases:    []DatabaseTarget{{URL: "a"}, {URL: "b"}},
		}
		c.PromoteSingleDatabaseForKube()
		if len(c.Databases) != 2 {
			t.Fatalf("want 2 entries, got %d", len(c.Databases))
		}
	})
}

// ---------------------------------------------------------------------------
// ConfigForTarget
// ---------------------------------------------------------------------------

func assertStrField(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", name, got, want)
	}
}

func assertIntField(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", name, got, want)
	}
}

func TestConfigForTarget(t *testing.T) {
	base := Config{
		DBURL:                   "postgres://base/db",
		Client:                  "base-client",
		StaleAge:                100,
		DefaultThresholdPercent: 80,
		ThresholdIdle:           10,
		ThresholdStale:          2,
		ThresholdLevels:         "75,85,95",
		SlackWebhook:            "https://hooks.slack.com/base",
		LokiURL:                 "http://loki:3100",
		Interval:                60,
		DryRun:                  true,
		LogLevel:                "debug",
	}

	target := DatabaseTarget{
		URL:                      "postgres://target/targetdb",
		Client:                   "target-client",
		StaleAge:                 600,
		DefaultThresholdPercent:  90,
		ThresholdIdle:            50,
		ThresholdStale:           10,
		ThresholdLevels:          "60,70,80",
		LongQueryMinSeconds:      120,
		LongQueryCooldownSeconds: 1800,
		LongQueryMinCount:        2,
	}

	got := base.ConfigForTarget(target)

	// Target-specific fields must come from DatabaseTarget
	assertStrField(t, "DBURL", got.DBURL, target.URL)
	assertStrField(t, "Client", got.Client, target.Client)
	assertIntField(t, "StaleAge", got.StaleAge, target.StaleAge)
	assertIntField(t, "DefaultThresholdPercent", got.DefaultThresholdPercent, target.DefaultThresholdPercent)
	assertIntField(t, "ThresholdIdle", got.ThresholdIdle, target.ThresholdIdle)
	assertIntField(t, "ThresholdStale", got.ThresholdStale, target.ThresholdStale)
	assertStrField(t, "ThresholdLevels", got.ThresholdLevels, target.ThresholdLevels)
	assertIntField(t, "LongQueryMinSeconds", got.LongQueryMinSeconds, target.LongQueryMinSeconds)
	assertIntField(t, "LongQueryCooldownSeconds", got.LongQueryCooldownSeconds, target.LongQueryCooldownSeconds)
	assertIntField(t, "LongQueryMinCount", got.LongQueryMinCount, target.LongQueryMinCount)

	// Non-target fields must be preserved from base
	assertStrField(t, "SlackWebhook", got.SlackWebhook, base.SlackWebhook)
	assertStrField(t, "LokiURL", got.LokiURL, base.LokiURL)
	assertIntField(t, "Interval", got.Interval, base.Interval)
	assertStrField(t, "LogLevel", got.LogLevel, base.LogLevel)
	if got.DryRun != base.DryRun {
		t.Errorf("DryRun should be preserved: got %v", got.DryRun)
	}
}

func TestConfigForTarget_DoesNotMutateBase(t *testing.T) {
	base := Config{DBURL: "postgres://base/db", Client: "base"}
	target := DatabaseTarget{URL: "postgres://new/db", Client: "new"}

	_ = base.ConfigForTarget(target)

	if base.DBURL != "postgres://base/db" {
		t.Errorf("base.DBURL mutated: got %q", base.DBURL)
	}
	if base.Client != "base" {
		t.Errorf("base.Client mutated: got %q", base.Client)
	}
}

// ---------------------------------------------------------------------------
// databaseNameFromURL
// ---------------------------------------------------------------------------

func TestDatabaseNameFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"normal", "postgres://localhost:5432/mydb", "mydb"},
		{"with user", "postgres://user:pass@host/proddb", "proddb"},
		{"empty path", "postgres://localhost:5432", "postgres"},
		{"slash only", "postgres://localhost:5432/", "postgres"},
		{"invalid url", "://bad", "unknown"},
		{"empty string", "", "postgres"},
		{"nested path", "postgres://host/db/extra", "db/extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := databaseNameFromURL(tt.url); got != tt.want {
				t.Errorf("databaseNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
