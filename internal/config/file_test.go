package config

import (
	"os"
	"path/filepath"
	"testing"
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
