// Package metricsexport writes persisted metrics to external sinks (e.g. CSV file).
// Rows are read via internal/metricsstore (SQLite only; PostgreSQL/MySQL store drivers are not implemented).
package metricsexport

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/metricsstore"
)

// Known export format names (CLI / PGWD_EXPORT_METRICS_FORMAT).
const (
	FormatCSV = "csv"
)

// ErrUnsupportedFormat means the format is not implemented yet.
var ErrUnsupportedFormat = errors.New("unsupported metrics export format")

// Export reads all rows from the configured metrics store and writes them to destination using format.
// Returns the number of rows exported. destination meaning depends on format (e.g. file path for csv).
func Export(ctx context.Context, format, destination string, cfg *config.Config) (int, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	destination = strings.TrimSpace(destination)
	if format == "" {
		return 0, fmt.Errorf("metrics export: format is required")
	}
	if destination == "" {
		return 0, fmt.Errorf("metrics export: destination is required")
	}
	if cfg == nil {
		return 0, fmt.Errorf("metrics export: config is required")
	}

	rows, err := metricsstore.ExportRows(ctx, cfg)
	if err != nil {
		return 0, err
	}

	switch format {
	case FormatCSV:
		if err := writeCSV(destination, rows); err != nil {
			return 0, err
		}
		return len(rows), nil
	default:
		return 0, fmt.Errorf("%w: %q (supported: %s)", ErrUnsupportedFormat, format, FormatCSV)
	}
}
