package cli

import (
	"path/filepath"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
)

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
