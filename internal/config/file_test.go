package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromFile_NotFound(t *testing.T) {
	cfg, loaded, err := FromFile("/nonexistent/path/pgwd.conf")
	if err != nil {
		t.Fatalf("FromFile(nonexistent): unexpected error: %v", err)
	}
	if loaded {
		t.Error("expected loaded=false when file not found")
	}
	if cfg.DBURL != "" {
		t.Errorf("expected empty config when file not found, got DBURL=%q", cfg.DBURL)
	}
}

func TestFromFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: test-monitor
db:
  url: postgres://localhost/testdb
  threshold:
    levels: "70,85,95"
interval: 120
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Error("expected loaded=true when file exists")
	}
	if cfg.DBURL != "postgres://localhost/testdb" {
		t.Errorf("DBURL: got %q", cfg.DBURL)
	}
	if cfg.Interval != 120 {
		t.Errorf("Interval: got %d", cfg.Interval)
	}
	if cfg.ThresholdLevels != "70,85,95" {
		t.Errorf("ThresholdLevels: got %q", cfg.ThresholdLevels)
	}
	if cfg.Client != "test-monitor" {
		t.Errorf("Client: got %q", cfg.Client)
	}
}

func TestFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.conf")
	if err := os.WriteFile(path, []byte("invalid: yaml: [:"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := FromFile(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestFromFile_SingleDbWrappedAsDatabases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor-01
db:
  url: postgres://user:pass@host:5432/prod
  threshold:
    levels: "75,85,95"
interval: 60
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	// Single db is normalized to Databases with one entry
	if len(cfg.Databases) != 1 {
		t.Fatalf("Databases: expected 1 entry, got %d", len(cfg.Databases))
	}
	t0 := cfg.Databases[0]
	if t0.URL != "postgres://user:pass@host:5432/prod" {
		t.Errorf("URL: got %q", t0.URL)
	}
	if t0.Client != "monitor-01-prod" {
		t.Errorf("Client (derived): got %q", t0.Client)
	}
	if t0.ThresholdLevels != "75,85,95" {
		t.Errorf("ThresholdLevels: got %q", t0.ThresholdLevels)
	}
	if !cfg.UsesDatabases() {
		t.Error("UsesDatabases: expected true")
	}
	targets := cfg.Targets()
	if len(targets) != 1 {
		t.Fatalf("Targets(): expected 1, got %d", len(targets))
	}
}

func TestFromFile_DatabasesSingle(t *testing.T) {
	// Canonical single-DB: databases with one entry (no deprecated db).
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor-01
interval: 60
databases:
  - url: postgres://localhost:5432/mydb
    stale_age: 0
    threshold:
      levels: "75,85,95"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	if len(cfg.Databases) != 1 {
		t.Fatalf("Databases: expected 1 entry, got %d", len(cfg.Databases))
	}
	t0 := cfg.Databases[0]
	if t0.URL != "postgres://localhost:5432/mydb" {
		t.Errorf("URL: got %q", t0.URL)
	}
	if t0.ThresholdLevels != "75,85,95" {
		t.Errorf("ThresholdLevels: got %q", t0.ThresholdLevels)
	}
	if t0.DefaultThresholdPercent != 80 {
		t.Errorf("DefaultThresholdPercent (default): got %d", t0.DefaultThresholdPercent)
	}
}

func TestFromFile_DatabasesMulti(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor-01
interval: 60
databases:
  - url: postgres://host1:5432/prod
    client: monitor-01-prod
  - url: postgres://host2:5432/analytics
    client: monitor-01-analytics
  - url: postgres://host3:5432/replica
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	if len(cfg.Databases) != 3 {
		t.Fatalf("Databases: expected 3 entries, got %d", len(cfg.Databases))
	}
	// Entry 1: explicit client
	if cfg.Databases[0].URL != "postgres://host1:5432/prod" || cfg.Databases[0].Client != "monitor-01-prod" {
		t.Errorf("entry 0: url=%q client=%q", cfg.Databases[0].URL, cfg.Databases[0].Client)
	}
	// Entry 2: explicit client
	if cfg.Databases[1].URL != "postgres://host2:5432/analytics" || cfg.Databases[1].Client != "monitor-01-analytics" {
		t.Errorf("entry 1: url=%q client=%q", cfg.Databases[1].URL, cfg.Databases[1].Client)
	}
	// Entry 3: client derived from base + db name from URL
	if cfg.Databases[2].URL != "postgres://host3:5432/replica" || cfg.Databases[2].Client != "monitor-01-replica" {
		t.Errorf("entry 2: url=%q client=%q (expected derived)", cfg.Databases[2].URL, cfg.Databases[2].Client)
	}
}

func TestFromFile_DatabasesOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor-01
interval: 60
db:
  url: postgres://default:5432/base
  stale_age: 300
  default_threshold_percent: 80
  threshold:
    levels: "75,85,95"
databases:
  - url: postgres://host1:5432/prod
    client: prod-monitor
    threshold:
      levels: "80,90,98"
  - url: postgres://host2:5432/analytics
    stale_age: 600
    default_threshold_percent: 70
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	if len(cfg.Databases) != 2 {
		t.Fatalf("Databases: expected 2 entries, got %d", len(cfg.Databases))
	}
	// prod: overrides threshold.levels, explicit client
	p := cfg.Databases[0]
	if p.ThresholdLevels != "80,90,98" {
		t.Errorf("prod ThresholdLevels: got %q", p.ThresholdLevels)
	}
	if p.StaleAge != 300 {
		t.Errorf("prod StaleAge (inherited): got %d", p.StaleAge)
	}
	if p.DefaultThresholdPercent != 80 {
		t.Errorf("prod DefaultThresholdPercent (inherited): got %d", p.DefaultThresholdPercent)
	}
	// analytics: overrides stale_age and default_threshold_percent
	a := cfg.Databases[1]
	if a.StaleAge != 600 {
		t.Errorf("analytics StaleAge (override): got %d", a.StaleAge)
	}
	if a.DefaultThresholdPercent != 70 {
		t.Errorf("analytics DefaultThresholdPercent (override): got %d", a.DefaultThresholdPercent)
	}
	if a.ThresholdLevels != "75,85,95" {
		t.Errorf("analytics ThresholdLevels (inherited): got %q", a.ThresholdLevels)
	}
}

func TestFromFile_Sqlite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor
interval: 60
databases:
  - url: postgres://localhost/mydb
sqlite:
  path: /var/lib/pgwd/pgwd.db
  max_metrics: 50000
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	if cfg.SqlitePath != "/var/lib/pgwd/pgwd.db" {
		t.Errorf("SqlitePath: got %q", cfg.SqlitePath)
	}
	if cfg.SqliteMaxMetrics != 50000 {
		t.Errorf("SqliteMaxMetrics: got %d", cfg.SqliteMaxMetrics)
	}
}

func TestFromFile_SqliteDefaultMaxMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor
databases:
  - url: postgres://localhost/mydb
sqlite:
  path: /tmp/pgwd.db
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if cfg.SqliteMaxMetrics != 10000 {
		t.Errorf("SqliteMaxMetrics (default): got %d", cfg.SqliteMaxMetrics)
	}
}

func TestFromFile_ConfirmAndHttp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor
databases:
  - url: postgres://localhost/mydb
confirm_alert: 3
confirm_ok: 2
sqlite:
  path: /var/lib/pgwd/pgwd.db
  stale_age: 300
http:
  listen: ":8080"
  base_path: "/api/pgwd/v1"
  healthz_path: "/health"
  metrics_path: "/m"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if cfg.ConfirmAlert != 3 {
		t.Errorf("ConfirmAlert: got %d", cfg.ConfirmAlert)
	}
	if cfg.ConfirmOk != 2 {
		t.Errorf("ConfirmOk: got %d", cfg.ConfirmOk)
	}
	if cfg.SqliteStaleAge != 300 {
		t.Errorf("SqliteStaleAge: got %d", cfg.SqliteStaleAge)
	}
	if cfg.HTTPListen != ":8080" {
		t.Errorf("HTTPListen: got %q", cfg.HTTPListen)
	}
	if cfg.HTTPBasePath != "/api/pgwd/v1" {
		t.Errorf("HTTPBasePath: got %q", cfg.HTTPBasePath)
	}
	if cfg.HTTPHealthPath != "/health" {
		t.Errorf("HTTPHealthPath: got %q", cfg.HTTPHealthPath)
	}
	if cfg.HTTPMetricsPath != "/m" {
		t.Errorf("HTTPMetricsPath: got %q", cfg.HTTPMetricsPath)
	}
}

func TestFromFile_NotificationsExtended(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.conf")
	content := `
client: monitor
databases:
  - url: postgres://localhost/mydb
notifications:
  pagerduty:
    enabled: true
    routing_key: pd-key
    severity: critical
    source: pgwd-prod
  teams:
    enabled: true
    webhook_url: https://teams.example/hook
  generic:
    enabled: true
    webhook_url: https://api.example/hook
    json_key: message
    headers:
      Authorization: Bearer tok
    extra_fields:
      source: pgwd
    hmac_header: X-Signature
  retry:
    max_attempts: 5
    initial_backoff: 2s
    max_backoff: 30s
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, loaded, err := FromFile(path)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	assertExtendedNotifications(t, cfg)
}

func assertExtendedNotifications(t *testing.T, cfg Config) {
	t.Helper()
	assertPagerDutyConfig(t, cfg)
	assertTeamsConfig(t, cfg)
	assertGenericConfig(t, cfg)
	assertRetryConfig(t, cfg)
}

func assertPagerDutyConfig(t *testing.T, cfg Config) {
	t.Helper()
	if !cfg.PagerDutyEnabled || cfg.PagerDutyRoutingKey != "pd-key" || cfg.PagerDutySeverity != "critical" {
		t.Errorf("PagerDuty: enabled=%v key=%q severity=%q", cfg.PagerDutyEnabled, cfg.PagerDutyRoutingKey, cfg.PagerDutySeverity)
	}
}

func assertTeamsConfig(t *testing.T, cfg Config) {
	t.Helper()
	if !cfg.TeamsEnabled || cfg.TeamsWebhook != "https://teams.example/hook" {
		t.Errorf("Teams: enabled=%v webhook=%q", cfg.TeamsEnabled, cfg.TeamsWebhook)
	}
}

func assertGenericConfig(t *testing.T, cfg Config) {
	t.Helper()
	if !cfg.GenericEnabled || cfg.GenericWebhookURL != "https://api.example/hook" {
		t.Errorf("Generic: enabled=%v url=%q", cfg.GenericEnabled, cfg.GenericWebhookURL)
	}
	if cfg.GenericJSONKey != "message" || cfg.GenericHeaders["Authorization"] != "Bearer tok" {
		t.Errorf("Generic json_key/headers: key=%q headers=%v", cfg.GenericJSONKey, cfg.GenericHeaders)
	}
	if cfg.GenericExtraFields["source"] != "pgwd" || cfg.GenericHMACHeader != "X-Signature" {
		t.Errorf("Generic extra/hmac: extra=%v hmac=%q", cfg.GenericExtraFields, cfg.GenericHMACHeader)
	}
}

func assertRetryConfig(t *testing.T, cfg Config) {
	t.Helper()
	if cfg.RetryMaxAttempts != 5 || cfg.RetryInitialBackoff != 2*time.Second || cfg.RetryMaxBackoff != 30*time.Second {
		t.Errorf("Retry: attempts=%d initial=%v max=%v", cfg.RetryMaxAttempts, cfg.RetryInitialBackoff, cfg.RetryMaxBackoff)
	}
}
