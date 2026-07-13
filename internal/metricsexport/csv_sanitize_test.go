package metricsexport

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

func TestSanitizeCSVField(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"prod", "prod"},
		{"", ""},
		{"=1+1", "'=1+1"},
		{"+cmd", "'+cmd"},
		{"-2+3", "'-2+3"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"  =hidden", "'  =hidden"},
	}
	for _, tt := range tests {
		if got := sanitizeCSVField(tt.in); got != tt.want {
			t.Errorf("sanitizeCSVField(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExport_CSV_FormulaInjection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "m.db")
	st, err := store.Open(dbPath, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Insert(ctx, store.Record{
		Client: "=cmd", Cluster: "cl", Namespace: "", Database: "db",
		Total: 1, Active: 0, Idle: 1, Stale: 0, MaxConnections: 10,
		State: "+evil", Threshold: "",
	}); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.csv")
	cfg := &config.Config{SqlitePath: dbPath}
	if _, err := Export(ctx, FormatCSV, outPath, cfg); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	all, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if all[1][3] != "'=cmd" || all[1][12] != "'+evil" {
		t.Fatalf("sanitized row: %v", all[1])
	}
}
