package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  client TEXT NOT NULL,
  cluster TEXT,
  namespace TEXT,
  database TEXT,
  total INTEGER NOT NULL,
  active INTEGER NOT NULL,
  idle INTEGER NOT NULL,
  stale INTEGER NOT NULL,
  max_connections INTEGER NOT NULL,
  state TEXT NOT NULL,
  threshold TEXT
);
CREATE INDEX IF NOT EXISTS idx_metrics_ts ON metrics(ts);
CREATE INDEX IF NOT EXISTS idx_metrics_target ON metrics(client, cluster, database);
`

// Record is one metrics row to insert.
type Record struct {
	Client         string
	Cluster        string
	Namespace      string
	Database       string
	Total          int
	Active         int
	Idle           int
	Stale          int
	MaxConnections int
	State          string // ok, attention, alert, danger, connect_failure
	Threshold      string // e.g. total, active, or empty for ok
}

// Store persists metrics to SQLite with FIFO eviction.
type Store struct {
	db         *sql.DB
	maxMetrics int
}

// Open opens or creates the SQLite database and applies the schema.
// Creates parent directory if needed.
func Open(path string, maxMetrics int) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	if maxMetrics <= 0 {
		maxMetrics = 10000
	}
	return &Store{db: db, maxMetrics: maxMetrics}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Insert inserts one metrics record and evicts oldest rows if over maxMetrics (FIFO).
func (s *Store) Insert(ctx context.Context, r Record) error {
	ts := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metrics (ts, client, cluster, namespace, database, total, active, idle, stale, max_connections, state, threshold)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts, r.Client, r.Cluster, r.Namespace, r.Database,
		r.Total, r.Active, r.Idle, r.Stale, r.MaxConnections,
		r.State, r.Threshold,
	)
	if err != nil {
		return err
	}
	return s.evictIfNeeded(ctx)
}

func (s *Store) evictIfNeeded(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM metrics").Scan(&count); err != nil {
		return err
	}
	if count <= s.maxMetrics {
		return nil
	}
	toDelete := count - s.maxMetrics
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM metrics WHERE id IN (SELECT id FROM metrics ORDER BY ts ASC LIMIT ?)`,
		toDelete,
	)
	return err
}

// LatestRecords returns the most recent record per target (client, cluster, database).
// Used for /metrics Prometheus export.
func (s *Store) LatestRecords(ctx context.Context) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.client, m.cluster, m.namespace, m.database, m.total, m.active, m.idle, m.stale, m.max_connections, m.state, m.threshold
		 FROM metrics m
		 INNER JOIN (
		   SELECT client, COALESCE(cluster,'') as cluster, COALESCE(database,'') as database, max(id) as mid
		   FROM metrics GROUP BY client, COALESCE(cluster,''), COALESCE(database,'')
		 ) sub ON m.client=sub.client AND COALESCE(m.cluster,'')=sub.cluster AND COALESCE(m.database,'')=sub.database AND m.id=sub.mid`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var cluster, ns, db sql.NullString
		var thr sql.NullString
		if err := rows.Scan(&r.Client, &cluster, &ns, &db, &r.Total, &r.Active, &r.Idle, &r.Stale, &r.MaxConnections, &r.State, &thr); err != nil {
			return nil, err
		}
		if cluster.Valid {
			r.Cluster = cluster.String
		}
		if ns.Valid {
			r.Namespace = ns.String
		}
		if db.Valid {
			r.Database = db.String
		}
		if thr.Valid {
			r.Threshold = thr.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastStates returns the last N state values for a target, newest first.
// Used for hysteresis (confirm_ok, confirm_alert) and resolution detection.
func (s *Store) LastStates(ctx context.Context, client, cluster, database string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT state FROM metrics WHERE client=? AND cluster=? AND database=? ORDER BY ts DESC LIMIT ?`,
		client, cluster, database, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var states []string
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			return nil, err
		}
		states = append(states, st)
	}
	return states, rows.Err()
}
