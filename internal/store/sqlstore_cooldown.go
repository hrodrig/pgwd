package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	schemaAlertCooldownPostgres = `
CREATE TABLE IF NOT EXISTS alert_cooldown (
  client TEXT NOT NULL,
  cluster TEXT NOT NULL DEFAULT '',
  "database" TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL,
  last_alert_ms BIGINT NOT NULL,
  PRIMARY KEY (client, cluster, "database", kind)
)`
)

func schemaAlertCooldownMySQL() string {
	dbCol := "`database`"
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS alert_cooldown (
  client VARCHAR(255) NOT NULL,
  cluster VARCHAR(255) NOT NULL DEFAULT '',
  %s VARCHAR(255) NOT NULL DEFAULT '',
  kind VARCHAR(64) NOT NULL,
  last_alert_ms BIGINT NOT NULL,
  PRIMARY KEY (client(64), cluster(64), %s(64), kind(32))
)`, dbCol, dbCol)
}

func applyAlertCooldownSchema(db *sql.DB, d sqlDialect) error {
	if d == dialectPostgres {
		_, err := db.Exec(schemaAlertCooldownPostgres)
		return err
	}
	_, err := db.Exec(schemaAlertCooldownMySQL())
	return err
}

// LastLongQueryAlert returns the last time a long_query notification was sent for this target, if any.
func (s *SQLStore) LastLongQueryAlert(ctx context.Context, client, cluster, database string) (time.Time, bool, error) {
	if s.dialect == dialectPostgres {
		return lastCooldownPostgres(ctx, s.db, client, cluster, database)
	}
	return lastCooldownMySQL(ctx, s.db, client, cluster, database)
}

// SetLongQueryAlert records that a long_query notification was sent at at.
func (s *SQLStore) SetLongQueryAlert(ctx context.Context, client, cluster, database string, at time.Time) error {
	if s.dialect == dialectPostgres {
		return upsertCooldownPostgres(ctx, s.db, client, cluster, database, at)
	}
	return upsertCooldownMySQL(ctx, s.db, client, cluster, database, at)
}

func lastCooldownPostgres(ctx context.Context, db *sql.DB, client, cluster, database string) (time.Time, bool, error) {
	const q = `SELECT last_alert_ms FROM alert_cooldown WHERE client=$1 AND cluster=$2 AND "database"=$3 AND kind=$4`
	var ms sql.NullInt64
	err := db.QueryRowContext(ctx, q, client, cluster, database, alertCooldownKindLongQuery).Scan(&ms)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	if !ms.Valid {
		return time.Time{}, false, nil
	}
	return time.UnixMilli(ms.Int64), true, nil
}

func upsertCooldownPostgres(ctx context.Context, db *sql.DB, client, cluster, database string, at time.Time) error {
	ms := at.UnixMilli()
	const q = `INSERT INTO alert_cooldown (client, cluster, "database", kind, last_alert_ms) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (client, cluster, "database", kind) DO UPDATE SET last_alert_ms = EXCLUDED.last_alert_ms`
	_, err := db.ExecContext(ctx, q, client, cluster, database, alertCooldownKindLongQuery, ms)
	return err
}

func lastCooldownMySQL(ctx context.Context, db *sql.DB, client, cluster, database string) (time.Time, bool, error) {
	const q = "SELECT last_alert_ms FROM alert_cooldown WHERE client=? AND cluster=? AND `database`=? AND kind=?"
	var ms sql.NullInt64
	err := db.QueryRowContext(ctx, q, client, cluster, database, alertCooldownKindLongQuery).Scan(&ms)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	if !ms.Valid {
		return time.Time{}, false, nil
	}
	return time.UnixMilli(ms.Int64), true, nil
}

func upsertCooldownMySQL(ctx context.Context, db *sql.DB, client, cluster, database string, at time.Time) error {
	ms := at.UnixMilli()
	const q = "INSERT INTO alert_cooldown (client, cluster, `database`, kind, last_alert_ms) VALUES (?,?,?,?,?) " +
		"ON DUPLICATE KEY UPDATE last_alert_ms=VALUES(last_alert_ms)"
	_, err := db.ExecContext(ctx, q, client, cluster, database, alertCooldownKindLongQuery, ms)
	return err
}
