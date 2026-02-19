package config

import (
	"os"
	"testing"
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
	prefixes := []string{"PGWD_DB_URL", "PGWD_THRESHOLD_TOTAL", "PGWD_THRESHOLD_ACTIVE", "PGWD_THRESHOLD_IDLE",
		"PGWD_STALE_AGE", "PGWD_THRESHOLD_STALE", "PGWD_SLACK_WEBHOOK", "PGWD_LOKI_URL", "PGWD_LOKI_LABELS",
		"PGWD_INTERVAL", "PGWD_DRY_RUN", "PGWD_FORCE_NOTIFICATION", "PGWD_DEFAULT_THRESHOLD_PERCENT"}
	for _, p := range prefixes {
		os.Unsetenv(p)
	}
	cfg := FromEnv()
	if cfg.DBURL != "" {
		t.Errorf("DBURL default: got %q", cfg.DBURL)
	}
	if cfg.ThresholdTotal != 0 || cfg.ThresholdActive != 0 || cfg.ThresholdIdle != 0 {
		t.Errorf("threshold defaults: total=%d active=%d idle=%d", cfg.ThresholdTotal, cfg.ThresholdActive, cfg.ThresholdIdle)
	}
	if cfg.Interval != 0 {
		t.Errorf("Interval default: got %d", cfg.Interval)
	}
	if cfg.DefaultThresholdPercent != 80 {
		t.Errorf("DefaultThresholdPercent default: got %d", cfg.DefaultThresholdPercent)
	}
	if cfg.DryRun || cfg.ForceNotification {
		t.Errorf("DryRun=%v ForceNotification=%v", cfg.DryRun, cfg.ForceNotification)
	}
}

func TestFromEnv_Values(t *testing.T) {
	defer setEnv("PGWD_DB_URL", "postgres://localhost/mydb")()
	defer setEnv("PGWD_THRESHOLD_TOTAL", "90")()
	defer setEnv("PGWD_THRESHOLD_ACTIVE", "50")()
	defer setEnv("PGWD_INTERVAL", "120")()
	defer setEnv("PGWD_DEFAULT_THRESHOLD_PERCENT", "70")()
	defer setEnv("PGWD_DRY_RUN", "true")()
	cfg := FromEnv()
	if cfg.DBURL != "postgres://localhost/mydb" {
		t.Errorf("DBURL: got %q", cfg.DBURL)
	}
	if cfg.ThresholdTotal != 90 || cfg.ThresholdActive != 50 {
		t.Errorf("thresholds: total=%d active=%d", cfg.ThresholdTotal, cfg.ThresholdActive)
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
		{"total", Config{ThresholdTotal: 80}, true},
		{"active", Config{ThresholdActive: 50}, true},
		{"idle", Config{ThresholdIdle: 40}, true},
		{"stale", Config{ThresholdStale: 1}, true},
		{"all", Config{ThresholdTotal: 1, ThresholdActive: 1, ThresholdIdle: 1, ThresholdStale: 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.HasAnyThreshold(); got != tt.want {
				t.Errorf("HasAnyThreshold() = %v, want %v", got, tt.want)
			}
		})
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
		{"both", Config{SlackWebhook: "x", LokiURL: "y"}, true},
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
		DBURL:           "postgres://old",
		ThresholdTotal:  10,
		SlackWebhook:    "https://old",
		DefaultThresholdPercent: 80,
	}
	db := "postgres://new"
	total := 20
	percent := 90
	c.OverrideWith(struct {
		DBURL           *string
		ThresholdTotal  *int
		ThresholdActive *int
		ThresholdIdle   *int
		StaleAge        *int
		ThresholdStale  *int
		SlackWebhook    *string
		LokiURL         *string
		LokiLabels      *string
		Interval               *int
		DryRun                 *bool
		ForceNotification      *bool
		DefaultThresholdPercent *int
	}{
		DBURL: &db, ThresholdTotal: &total, DefaultThresholdPercent: &percent,
	})
	if c.DBURL != "postgres://new" {
		t.Errorf("DBURL after override: got %q", c.DBURL)
	}
	if c.ThresholdTotal != 20 {
		t.Errorf("ThresholdTotal after override: got %d", c.ThresholdTotal)
	}
	if c.DefaultThresholdPercent != 90 {
		t.Errorf("DefaultThresholdPercent after override: got %d", c.DefaultThresholdPercent)
	}
	if c.SlackWebhook != "https://old" {
		t.Errorf("SlackWebhook should be unchanged when nil override: got %q", c.SlackWebhook)
	}
}
