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
