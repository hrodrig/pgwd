package store

import (
	"context"
	"database/sql"
	"time"
)

const alertCooldownKindLongQuery = "long_query"

// AlertCooldownRecorder persists last notification time for rate-limited alert types.
type AlertCooldownRecorder interface {
	LastLongQueryAlert(ctx context.Context, client, cluster, database string) (at time.Time, ok bool, err error)
	SetLongQueryAlert(ctx context.Context, client, cluster, database string, at time.Time) error
}

var (
	_ AlertCooldownRecorder = (*Store)(nil)
	_ AlertCooldownRecorder = (*SQLStore)(nil)
)

// LastLongQueryAlert returns the last time a long_query notification was sent for this target, if any.
func (s *Store) LastLongQueryAlert(ctx context.Context, client, cluster, database string) (time.Time, bool, error) {
	return lastCooldownSQLite(ctx, s.db, client, cluster, database, alertCooldownKindLongQuery)
}

// SetLongQueryAlert records that a long_query notification was sent at at.
func (s *Store) SetLongQueryAlert(ctx context.Context, client, cluster, database string, at time.Time) error {
	return upsertCooldownSQLite(ctx, s.db, client, cluster, database, alertCooldownKindLongQuery, at)
}

func lastCooldownSQLite(ctx context.Context, db *sql.DB, client, cluster, database, kind string) (time.Time, bool, error) {
	var ms sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT last_alert_ms FROM alert_cooldown WHERE client=? AND cluster=? AND database=? AND kind=?`,
		client, cluster, database, kind,
	).Scan(&ms)
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

func upsertCooldownSQLite(ctx context.Context, db *sql.DB, client, cluster, database, kind string, at time.Time) error {
	ms := at.UnixMilli()
	_, err := db.ExecContext(ctx,
		`INSERT INTO alert_cooldown (client, cluster, database, kind, last_alert_ms) VALUES (?,?,?,?,?)
		 ON CONFLICT(client, cluster, database, kind) DO UPDATE SET last_alert_ms=excluded.last_alert_ms`,
		client, cluster, database, kind, ms,
	)
	return err
}
