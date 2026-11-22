package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hrodrig/pgwd/internal/config"
)

func TestCheckTimeout(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		want     time.Duration
	}{
		{"nil", nil, MaxCheckTimeout},
		{"one-shot", &config.Config{Interval: 0}, MaxCheckTimeout},
		{"short interval", &config.Config{Interval: 10}, 10 * time.Second},
		{"one second", &config.Config{Interval: 1}, time.Second},
		{"caps at max", &config.Config{Interval: 120}, MaxCheckTimeout},
		{"negative interval like one-shot", &config.Config{Interval: -5}, MaxCheckTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckTimeout(tt.cfg)
			if got != tt.want {
				t.Fatalf("CheckTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyDBURLOverride_clearsDatabases(t *testing.T) {
	cfg := &config.Config{
		DBURL:    "postgres://localhost/db",
		Interval: 0,
		Databases: []config.DatabaseTarget{
			{Client: "a", URL: "postgres://localhost/a"},
		},
	}
	ApplyDBURLOverride(cfg)
	if cfg.UsesDatabases() {
		t.Fatal("expected databases cleared")
	}
}

func TestApplyDBURLOverride_keepsDatabasesWhenInterval(t *testing.T) {
	cfg := &config.Config{
		DBURL:    "postgres://localhost/db",
		Interval: 60,
		Databases: []config.DatabaseTarget{
			{Client: "a", URL: "postgres://localhost/a"},
		},
	}
	ApplyDBURLOverride(cfg)
	if !cfg.UsesDatabases() {
		t.Fatal("expected databases kept")
	}
}

func TestLogConfigTrace_unit(t *testing.T) {
	LogConfigTrace("/etc/pgwd/pgwd.conf", true, true)
	LogConfigTrace("", false, false)
}

func TestMaybeLogConfigTrace_debug(t *testing.T) {
	MaybeLogConfigTrace(&config.Config{LogLevel: "debug"}, "/tmp/x.conf", false, true)
}

func TestLogStartupBanner_unit(t *testing.T) {
	SetBuildInfo("0.7.0", "abc", "2026-07-03", "develop")
	LogStartupBanner(&config.Config{LogLevel: "info"})
}

func TestOpenStoreIfConfigured_sqlite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.db")
	cfg := &config.Config{SqlitePath: path, SqliteMaxMetrics: 10}
	st := openStoreIfConfigured(cfg)
	if st == nil {
		t.Fatal("want store")
	}
	st.Close()
}

func TestSetupHTTPIfConfigured_disabled(t *testing.T) {
	cleanup := setupHTTPIfConfigured(&config.Config{}, nil)
	cleanup()
}

func TestPrintVersion_smoke(t *testing.T) {
	SetBuildInfo("0.7.0-test", "abc", "2026-07-03", "develop")
	printVersion()
}
