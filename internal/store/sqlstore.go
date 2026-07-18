package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type sqlDialect int

const (
	dialectPostgres sqlDialect = iota
	dialectMySQL
)

// SQLStore persists metrics to PostgreSQL or MySQL (same logical schema as SQLite metrics table).
type SQLStore struct {
	db         *sql.DB
	dialect    sqlDialect
	maxMetrics int
}

var (
	_ MetricsStorer = (*SQLStore)(nil)
	_ MetricsStorer = (*Store)(nil)
)

const (
	schemaPostgresTable = `
CREATE TABLE IF NOT EXISTS metrics (
  id BIGSERIAL PRIMARY KEY,
  ts BIGINT NOT NULL,
  client TEXT NOT NULL,
  cluster TEXT,
  namespace TEXT,
  "database" TEXT,
  total INTEGER NOT NULL,
  active INTEGER NOT NULL,
  idle INTEGER NOT NULL,
  stale INTEGER NOT NULL,
  max_connections INTEGER NOT NULL,
  state TEXT NOT NULL,
  threshold TEXT
)`
	schemaPostgresIdxTS     = `CREATE INDEX IF NOT EXISTS idx_metrics_ts ON metrics(ts)`
	schemaPostgresIdxTarget = `CREATE INDEX IF NOT EXISTS idx_metrics_target ON metrics(client, cluster, "database")`
)

// MySQL reserves `database`; use backticks in DDL (cannot use raw string literals).
// Inline KEY clauses keep CREATE idempotent (no separate CREATE INDEX on every Open).
func schemaMySQLTable() string {
	dbCol := "`database`"
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS metrics (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  ts BIGINT NOT NULL,
  client VARCHAR(255) NOT NULL,
  cluster VARCHAR(255),
  namespace VARCHAR(255),
  %s VARCHAR(255),
  total INT NOT NULL,
  active INT NOT NULL,
  idle INT NOT NULL,
  stale INT NOT NULL,
  max_connections INT NOT NULL,
  state VARCHAR(64) NOT NULL,
  threshold VARCHAR(255),
  KEY idx_metrics_ts (ts),
  KEY idx_metrics_target (client, cluster, %s(191))
)`, dbCol, dbCol)
}

// OpenSQLMetrics opens a PostgreSQL or MySQL database for metrics persistence.
// driver must be "postgres", "postgresql", or "mysql" (case-insensitive).
// PostgreSQL DSN uses libpq format (e.g. postgres://user:pass@host:5432/dbname?sslmode=disable).
// MySQL DSN uses go-sql-driver format (e.g. user:pass@tcp(host:3306)/dbname?parseTime=true).
// maxMetrics is the FIFO cap (same semantics as sqlite.max_metrics; default 10000 if <= 0).
func OpenSQLMetrics(driver, dsn string, maxMetrics int) (*SQLStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("metrics SQL store: DSN is empty")
	}
	d, sqlDriver, err := normalizeSQLMetricsDriver(driver)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("metrics SQL store open: %w", err)
	}
	if err := applyMetricsSchema(db, d); err != nil {
		db.Close()
		return nil, fmt.Errorf("metrics SQL store schema: %w", err)
	}
	if maxMetrics <= 0 {
		maxMetrics = 10000
	}
	return &SQLStore{db: db, dialect: d, maxMetrics: maxMetrics}, nil
}

func applyMetricsSchema(db *sql.DB, d sqlDialect) error {
	if d == dialectPostgres {
		if _, err := db.Exec(schemaPostgresTable); err != nil {
			return err
		}
		if _, err := db.Exec(schemaPostgresIdxTS); err != nil {
			return err
		}
		if _, err := db.Exec(schemaPostgresIdxTarget); err != nil {
			return err
		}
		return applyAlertCooldownSchema(db, d)
	}
	if _, err := db.Exec(schemaMySQLTable()); err != nil {
		return err
	}
	return applyAlertCooldownSchema(db, d)
}

func normalizeSQLMetricsDriver(driver string) (sqlDialect, string, error) {
	d := strings.ToLower(strings.TrimSpace(driver))
	switch d {
	case "postgres", "postgresql":
		return dialectPostgres, "pgx", nil
	case "mysql":
		return dialectMySQL, "mysql", nil
	default:
		return 0, "", fmt.Errorf("metrics SQL store: unsupported driver %q (use postgres or mysql)", driver)
	}
}

// QueryAllMetricsFromDSN returns every metrics row, oldest first by id, for export.
func QueryAllMetricsFromDSN(ctx context.Context, driver, dsn string) ([]ExportRow, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("metrics SQL export: DSN is empty")
	}
	d, sqlDriver, err := normalizeSQLMetricsDriver(driver)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("metrics SQL export open: %w", err)
	}
	defer db.Close()

	return queryAllMetrics(ctx, db, d)
}

func queryAllMetrics(ctx context.Context, db *sql.DB, d sqlDialect) ([]ExportRow, error) {
	colDB := colDatabaseQuoted(d)
	q := fmt.Sprintf(
		`SELECT id, ts, client, cluster, namespace, %s, total, active, idle, stale, max_connections, state, threshold FROM metrics ORDER BY id ASC`,
		colDB,
	)
	qrows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer qrows.Close()
	var out []ExportRow
	for qrows.Next() {
		var r ExportRow
		var cluster, ns, dbcol sql.NullString
		var thr sql.NullString
		if err := qrows.Scan(&r.ID, &r.TSMillis, &r.Client, &cluster, &ns, &dbcol,
			&r.Total, &r.Active, &r.Idle, &r.Stale, &r.MaxConnections, &r.State, &thr); err != nil {
			return nil, err
		}
		if cluster.Valid {
			r.Cluster = cluster.String
		}
		if ns.Valid {
			r.Namespace = ns.String
		}
		if dbcol.Valid {
			r.Database = dbcol.String
		}
		if thr.Valid {
			r.Threshold = thr.String
		}
		out = append(out, r)
	}
	return out, qrows.Err()
}

func colDatabaseQuoted(d sqlDialect) string {
	if d == dialectMySQL {
		return "`database`"
	}
	return `"database"`
}

func (s *SQLStore) insertSQL() string {
	n := 12
	if s.dialect == dialectPostgres {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = "$" + strconv.Itoa(i+1)
		}
		colDB := colDatabaseQuoted(s.dialect)
		return fmt.Sprintf(
			`INSERT INTO metrics (ts, client, cluster, namespace, %s, total, active, idle, stale, max_connections, state, threshold) VALUES (%s)`,
			colDB, strings.Join(parts, ","),
		)
	}
	colDB := colDatabaseQuoted(s.dialect)
	return fmt.Sprintf(
		`INSERT INTO metrics (ts, client, cluster, namespace, %s, total, active, idle, stale, max_connections, state, threshold) VALUES (%s)`,
		colDB, strings.TrimSuffix(strings.Repeat("?,", n), ","),
	)
}

// Insert inserts one metrics record and evicts oldest rows if over maxMetrics (FIFO).
func (s *SQLStore) Insert(ctx context.Context, r Record) error {
	ts := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, s.insertSQL(),
		ts, r.Client, r.Cluster, r.Namespace, r.Database,
		r.Total, r.Active, r.Idle, r.Stale, r.MaxConnections,
		r.State, r.Threshold,
	)
	if err != nil {
		return err
	}
	return s.evictIfNeeded(ctx)
}

func (s *SQLStore) evictIfNeeded(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM metrics").Scan(&count); err != nil {
		return err
	}
	if count <= s.maxMetrics {
		return nil
	}
	toDelete := count - s.maxMetrics
	var q string
	var args []interface{}
	if s.dialect == dialectPostgres {
		q = `DELETE FROM metrics WHERE id IN (SELECT id FROM metrics ORDER BY ts ASC LIMIT $1)`
		args = []interface{}{toDelete}
	} else {
		q = `DELETE FROM metrics WHERE id IN (SELECT id FROM (SELECT id FROM metrics ORDER BY ts ASC LIMIT ?) AS del_ids)`
		args = []interface{}{toDelete}
	}
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *SQLStore) latestRecordsSQL() string {
	colDB := colDatabaseQuoted(s.dialect)
	mDB := aliasColDB("m", s.dialect)
	// Latest row per (client, cluster, database) by max(id). Avoid reserved alias "database".
	return fmt.Sprintf(`
SELECT m.client, m.cluster, m.namespace, %s, m.total, m.active, m.idle, m.stale, m.max_connections, m.state, m.threshold
FROM metrics m
INNER JOIN (
  SELECT client, COALESCE(cluster,'') AS ckey, COALESCE(%s,'') AS dkey, max(id) AS mid
  FROM metrics GROUP BY client, COALESCE(cluster,''), COALESCE(%s,'')
) sub ON m.client=sub.client AND COALESCE(m.cluster,'')=sub.ckey AND COALESCE(%s,'')=sub.dkey AND m.id=sub.mid`,
		mDB, colDB, colDB, mDB,
	)
}

func aliasColDB(alias string, d sqlDialect) string {
	if d == dialectMySQL {
		return fmt.Sprintf("%s.`database`", alias)
	}
	return fmt.Sprintf(`%s."database"`, alias)
}

// LatestRecords returns the most recent record per target (client, cluster, database).
func (s *SQLStore) LatestRecords(ctx context.Context) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, s.latestRecordsSQL())
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
func (s *SQLStore) LastStates(ctx context.Context, client, cluster, database string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	var q string
	var args []interface{}
	colDB := colDatabaseQuoted(s.dialect)
	if s.dialect == dialectPostgres {
		q = fmt.Sprintf(`SELECT state FROM metrics WHERE client=$1 AND COALESCE(cluster,'')=$2 AND COALESCE(%s,'')=$3 ORDER BY ts DESC LIMIT $4`, colDB)
		args = []interface{}{client, cluster, database, n}
	} else {
		q = fmt.Sprintf(`SELECT state FROM metrics WHERE client=? AND COALESCE(cluster,'')=? AND COALESCE(%s,'')=? ORDER BY ts DESC LIMIT ?`, colDB)
		args = []interface{}{client, cluster, database, n}
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
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

// Close closes the database connection.
func (s *SQLStore) Close() error {
	return s.db.Close()
}

// Ping verifies the database connection is alive.
func (s *SQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
