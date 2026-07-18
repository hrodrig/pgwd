package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeSQLMetricsDriver(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantD   sqlDialect
		wantDrv string
		wantErr bool
	}{
		{"postgres", dialectPostgres, "pgx", false},
		{"PostgreSQL", dialectPostgres, "pgx", false},
		{"  mysql  ", dialectMySQL, "mysql", false},
		{"sqlite", 0, "", true},
		{"", 0, "", true},
	}
	for _, tc := range cases {
		d, drv, err := normalizeSQLMetricsDriver(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if d != tc.wantD || drv != tc.wantDrv {
			t.Fatalf("%q: got (%v,%q) want (%v,%q)", tc.in, d, drv, tc.wantD, tc.wantDrv)
		}
	}
}

func TestSchemaBuilders_MySQL(t *testing.T) {
	t.Parallel()
	tbl := schemaMySQLTable()
	if !strings.Contains(tbl, "`database`") {
		t.Fatal("metrics DDL missing backtick database column")
	}
	if !strings.Contains(tbl, "AUTO_INCREMENT") {
		t.Fatal("metrics DDL missing AUTO_INCREMENT")
	}
	cd := schemaAlertCooldownMySQL()
	if !strings.Contains(cd, "`database`") || !strings.Contains(cd, "PRIMARY KEY") {
		t.Fatalf("cooldown DDL unexpected: %s", cd)
	}
}

func TestColDatabaseQuotedAndAlias(t *testing.T) {
	t.Parallel()
	if got := colDatabaseQuoted(dialectPostgres); got != `"database"` {
		t.Fatalf("postgres col: %q", got)
	}
	if got := colDatabaseQuoted(dialectMySQL); got != "`database`" {
		t.Fatalf("mysql col: %q", got)
	}
	if got := aliasColDB("m", dialectPostgres); got != `m."database"` {
		t.Fatalf("postgres alias: %q", got)
	}
	if got := aliasColDB("m", dialectMySQL); got != "m.`database`" {
		t.Fatalf("mysql alias: %q", got)
	}
}

func TestSQLStore_insertAndLatestSQL(t *testing.T) {
	t.Parallel()
	pg := &SQLStore{dialect: dialectPostgres}
	ins := pg.insertSQL()
	if !strings.Contains(ins, "$12") || !strings.Contains(ins, `"database"`) {
		t.Fatalf("postgres insertSQL: %s", ins)
	}
	latest := pg.latestRecordsSQL()
	if !strings.Contains(latest, `m."database"`) {
		t.Fatalf("postgres latest: %s", latest)
	}

	my := &SQLStore{dialect: dialectMySQL}
	ins = my.insertSQL()
	if strings.Contains(ins, "$1") || !strings.Contains(ins, "?,") {
		t.Fatalf("mysql insertSQL: %s", ins)
	}
	if !strings.Contains(ins, "`database`") {
		t.Fatalf("mysql insertSQL missing backticks: %s", ins)
	}
	latest = my.latestRecordsSQL()
	if !strings.Contains(latest, "m.`database`") {
		t.Fatalf("mysql latest: %s", latest)
	}
}

func TestApplyMetricsSchema_PostgresAndMySQL(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS metrics").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_metrics_ts").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_metrics_target").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS alert_cooldown").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := applyMetricsSchema(db, dialectPostgres); err != nil {
		t.Fatal(err)
	}

	db2, mock2, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	mock2.ExpectExec("CREATE TABLE IF NOT EXISTS metrics").WillReturnResult(sqlmock.NewResult(0, 0))
	mock2.ExpectExec("CREATE TABLE IF NOT EXISTS alert_cooldown").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := applyMetricsSchema(db2, dialectMySQL); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyMetricsSchema_error(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS metrics").WillReturnError(errors.New("boom"))
	if err := applyMetricsSchema(db, dialectPostgres); err == nil {
		t.Fatal("want schema error")
	}
}

func TestSQLStore_Insert_andEvict_Postgres(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectPostgres, maxMetrics: 2}
	ctx := context.Background()
	rec := Record{Client: "c", Cluster: "cl", Database: "d", Total: 1, Active: 1, MaxConnections: 10, State: "ok"}

	mock.ExpectExec(`INSERT INTO metrics`).WithArgs(
		sqlmock.AnyArg(), rec.Client, rec.Cluster, rec.Namespace, rec.Database,
		rec.Total, rec.Active, rec.Idle, rec.Stale, rec.MaxConnections, rec.State, rec.Threshold,
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM metrics`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	if err := s.Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec(`INSERT INTO metrics`).WithArgs(
		sqlmock.AnyArg(), rec.Client, rec.Cluster, rec.Namespace, rec.Database,
		rec.Total, rec.Active, rec.Idle, rec.Stale, rec.MaxConnections, rec.State, rec.Threshold,
	).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM metrics`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectExec(`DELETE FROM metrics WHERE id IN`).WithArgs(3).WillReturnResult(sqlmock.NewResult(0, 3))
	if err := s.Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLStore_Insert_andEvict_MySQL(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectMySQL, maxMetrics: 1}
	ctx := context.Background()
	rec := Record{Client: "c", Database: "d", Total: 2, State: "alert", Threshold: "total"}

	mock.ExpectExec(`INSERT INTO metrics`).WithArgs(
		sqlmock.AnyArg(), rec.Client, rec.Cluster, rec.Namespace, rec.Database,
		rec.Total, rec.Active, rec.Idle, rec.Stale, rec.MaxConnections, rec.State, rec.Threshold,
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM metrics`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectExec(`DELETE FROM metrics WHERE id IN`).WithArgs(2).WillReturnResult(sqlmock.NewResult(0, 2))
	if err := s.Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLStore_Insert_execError(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectPostgres, maxMetrics: 10}
	mock.ExpectExec(`INSERT INTO metrics`).WillReturnError(errors.New("insert failed"))
	if err := s.Insert(context.Background(), Record{Client: "c", State: "ok"}); err == nil {
		t.Fatal("want insert error")
	}
}

func TestSQLStore_LatestRecords(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectPostgres, maxMetrics: 10}

	rows := sqlmock.NewRows([]string{
		"client", "cluster", "namespace", "database", "total", "active", "idle", "stale", "max_connections", "state", "threshold",
	}).AddRow("c", "cl", "ns", "db1", 10, 2, 8, 0, 100, "ok", "total").
		AddRow("c2", nil, nil, nil, 1, 0, 1, 0, 50, "attention", nil)
	mock.ExpectQuery(`SELECT m.client`).WillReturnRows(rows)

	got, err := s.LatestRecords(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Client != "c" || got[0].Cluster != "cl" || got[0].Namespace != "ns" || got[0].Database != "db1" || got[0].Threshold != "total" {
		t.Fatalf("row0=%+v", got[0])
	}
	if got[1].Cluster != "" || got[1].Database != "" || got[1].Threshold != "" {
		t.Fatalf("nulls not cleared: %+v", got[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLStore_LatestRecords_queryError(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectMySQL, maxMetrics: 10}
	mock.ExpectQuery(`SELECT m.client`).WillReturnError(errors.New("q"))
	if _, err := s.LatestRecords(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestQueryAllMetrics(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "ts", "client", "cluster", "namespace", "database",
		"total", "active", "idle", "stale", "max_connections", "state", "threshold",
	}).AddRow(1, 1700000000000, "c", "cl", "ns", "db", 10, 2, 8, 0, 100, "ok", "total").
		AddRow(2, 1700000000001, "c2", nil, nil, nil, 1, 0, 1, 0, 50, "attention", nil)
	mock.ExpectQuery(`SELECT id, ts, client`).WillReturnRows(rows)

	got, err := queryAllMetrics(context.Background(), db, dialectPostgres)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	assertExportRowFilled(t, got[0])
	assertExportRowNullsCleared(t, got[1])
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func assertExportRowFilled(t *testing.T, r ExportRow) {
	t.Helper()
	if r.ID != 1 || r.TSMillis != 1700000000000 ||
		r.Client != "c" || r.Cluster != "cl" ||
		r.Namespace != "ns" || r.Database != "db" ||
		r.Threshold != "total" {
		t.Fatalf("row0=%+v", r)
	}
}

func assertExportRowNullsCleared(t *testing.T, r ExportRow) {
	t.Helper()
	if r.Cluster != "" || r.Namespace != "" ||
		r.Database != "" || r.Threshold != "" {
		t.Fatalf("nulls not cleared: %+v", r)
	}
}

func TestQueryAllMetrics_QueryError(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id, ts, client`).WillReturnError(errors.New("query failed"))

	if _, err := queryAllMetrics(context.Background(), db, dialectMySQL); err == nil {
		t.Fatal("want query error")
	}
}

func TestQueryAllMetrics_ScanError(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id, ts, client`).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1),
	)

	if _, err := queryAllMetrics(context.Background(), db, dialectPostgres); err == nil {
		t.Fatal("want scan error")
	}
}

func TestQueryAllMetrics_RowsError(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := sqlmock.NewRows([]string{
		"id", "ts", "client", "cluster", "namespace", "database",
		"total", "active", "idle", "stale", "max_connections", "state", "threshold",
	}).AddRow(1, 1700000000000, "c", nil, nil, nil, 1, 0, 1, 0, 10, "ok", nil).
		RowError(0, errors.New("rows failed"))
	mock.ExpectQuery(`SELECT id, ts, client`).WillReturnRows(rows)

	if _, err := queryAllMetrics(context.Background(), db, dialectPostgres); err == nil {
		t.Fatal("want rows error")
	}
}

func TestSQLStore_LastStates(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectPostgres, maxMetrics: 10}
	ctx := context.Background()

	got, err := s.LastStates(ctx, "c", "cl", "d", 0)
	if err != nil || got != nil {
		t.Fatalf("n<=0: got %v err %v", got, err)
	}

	mock.ExpectQuery(`SELECT state FROM metrics`).WithArgs("c", "cl", "d", 2).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow("ok").AddRow("alert"))
	got, err = s.LastStates(ctx, "c", "cl", "d", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "ok" || got[1] != "alert" {
		t.Fatalf("got %v", got)
	}

	db2, mock2, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	s2 := &SQLStore{db: db2, dialect: dialectMySQL, maxMetrics: 10}
	mock2.ExpectQuery(`SELECT state FROM metrics`).WithArgs("a", "b", "c", 1).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow("danger"))
	got, err = s2.LastStates(ctx, "a", "b", "c", 1)
	if err != nil || len(got) != 1 || got[0] != "danger" {
		t.Fatalf("mysql states=%v err=%v", got, err)
	}
}

func TestSQLStore_CloseAndPing(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	s := &SQLStore{db: db, dialect: dialectPostgres, maxMetrics: 10}
	mock.ExpectPing()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	mock.ExpectClose()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLStore_Cooldown_Postgres(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectPostgres, maxMetrics: 10}
	ctx := context.Background()

	mock.ExpectQuery(`SELECT last_alert_ms FROM alert_cooldown`).
		WithArgs("c", "cl", "db", alertCooldownKindLongQuery).
		WillReturnError(sql.ErrNoRows)
	_, ok, err := s.LastLongQueryAlert(ctx, "c", "cl", "db")
	if err != nil || ok {
		t.Fatalf("no rows: ok=%v err=%v", ok, err)
	}

	now := time.UnixMilli(1_700_000_000_000)
	mock.ExpectQuery(`SELECT last_alert_ms FROM alert_cooldown`).
		WithArgs("c", "cl", "db", alertCooldownKindLongQuery).
		WillReturnRows(sqlmock.NewRows([]string{"last_alert_ms"}).AddRow(now.UnixMilli()))
	got, ok, err := s.LastLongQueryAlert(ctx, "c", "cl", "db")
	if err != nil || !ok || !got.Equal(now) {
		t.Fatalf("got=%v ok=%v err=%v", got, ok, err)
	}

	mock.ExpectQuery(`SELECT last_alert_ms FROM alert_cooldown`).
		WithArgs("c", "cl", "db", alertCooldownKindLongQuery).
		WillReturnRows(sqlmock.NewRows([]string{"last_alert_ms"}).AddRow(nil))
	_, ok, err = s.LastLongQueryAlert(ctx, "c", "cl", "db")
	if err != nil || ok {
		t.Fatalf("null ms: ok=%v err=%v", ok, err)
	}

	mock.ExpectExec(`INSERT INTO alert_cooldown`).
		WithArgs("c", "cl", "db", alertCooldownKindLongQuery, now.UnixMilli()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := s.SetLongQueryAlert(ctx, "c", "cl", "db", now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLStore_Cooldown_MySQL(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &SQLStore{db: db, dialect: dialectMySQL, maxMetrics: 10}
	ctx := context.Background()
	now := time.UnixMilli(42)

	mock.ExpectQuery(`SELECT last_alert_ms FROM alert_cooldown`).
		WithArgs("c", "cl", "db", alertCooldownKindLongQuery).
		WillReturnError(sql.ErrNoRows)
	_, ok, err := s.LastLongQueryAlert(ctx, "c", "cl", "db")
	if err != nil || ok {
		t.Fatalf("no rows: ok=%v err=%v", ok, err)
	}

	mock.ExpectExec(`INSERT INTO alert_cooldown`).
		WithArgs("c", "cl", "db", alertCooldownKindLongQuery, now.UnixMilli()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := s.SetLongQueryAlert(ctx, "c", "cl", "db", now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenSQLMetrics_schemaConnectFail(t *testing.T) {
	t.Parallel()
	// sql.Open succeeds; first Exec fails without a live server (covers schema error path).
	_, err := OpenSQLMetrics("mysql", "root@tcp(127.0.0.1:1)/pgwd?timeout=250ms", 0)
	if err == nil {
		t.Fatal("want schema/open error against closed port")
	}
	if !strings.Contains(err.Error(), "metrics SQL store") {
		t.Fatalf("want wrapped error, got %v", err)
	}
}
