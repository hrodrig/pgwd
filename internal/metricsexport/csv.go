package metricsexport

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/pgwd/internal/store"
)

// sanitizeCSVField prefixes spreadsheet formula triggers (=, +, -, @, tab) per OWASP CSV guidance.
func sanitizeCSVField(s string) string {
	if s == "" {
		return s
	}
	trimmed := strings.TrimLeft(s, " \t")
	if trimmed == "" {
		return s
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	default:
		return s
	}
}

// writeCSV writes rows to path (truncates). RFC4180. Uses ts from each stored row.
func writeCSV(path string, rows []store.ExportRow) error {
	if parent := filepath.Dir(path); parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	hdr := []string{
		"id", "ts_ms", "ts_utc", "client", "cluster", "namespace", "database",
		"total", "active", "idle", "stale", "max_connections", "state", "threshold",
	}
	if err := w.Write(hdr); err != nil {
		return err
	}
	for _, r := range rows {
		tsUTC := time.UnixMilli(r.TSMillis).UTC().Format(time.RFC3339Nano)
		row := []string{
			strconv.FormatInt(r.ID, 10),
			strconv.FormatInt(r.TSMillis, 10),
			tsUTC,
			sanitizeCSVField(r.Client),
			sanitizeCSVField(r.Cluster),
			sanitizeCSVField(r.Namespace),
			sanitizeCSVField(r.Database),
			strconv.Itoa(r.Total),
			strconv.Itoa(r.Active),
			strconv.Itoa(r.Idle),
			strconv.Itoa(r.Stale),
			strconv.Itoa(r.MaxConnections),
			sanitizeCSVField(r.State),
			sanitizeCSVField(r.Threshold),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
