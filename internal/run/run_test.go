package run

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/postgres"
	"github.com/hrodrig/pgwd/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct {
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }

type fakeQuerier struct {
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f *fakeQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRow(ctx, sql, args...)
}

type fakeSender struct {
	err error
	n   int
}

func (f *fakeSender) Send(_ context.Context, _ notify.Event) error {
	f.n++
	return f.err
}

type memStore struct {
	states []string
	lastLQ time.Time
	hasLQ  bool
	lqErr  error
}

func (m *memStore) Insert(context.Context, store.Record) error { return nil }
func (m *memStore) LatestRecords(context.Context) ([]store.Record, error) {
	return nil, nil
}
func (m *memStore) LastStates(_ context.Context, _, _, _ string, n int) ([]string, error) {
	if n <= 0 || len(m.states) == 0 {
		return nil, nil
	}
	if n > len(m.states) {
		n = len(m.states)
	}
	out := make([]string, n)
	copy(out, m.states[:n])
	return out, nil
}
func (m *memStore) Close() error               { return nil }
func (m *memStore) Ping(context.Context) error { return nil }
func (m *memStore) LastLongQueryAlert(context.Context, string, string, string) (time.Time, bool, error) {
	return m.lastLQ, m.hasLQ, m.lqErr
}
func (m *memStore) SetLongQueryAlert(_ context.Context, _, _, _ string, t time.Time) error {
	m.lastLQ, m.hasLQ = t, true
	return nil
}

func TestIsTooManyClientsError(t *testing.T) {
	t.Parallel()
	italian := &pgconn.PgError{Code: "53300", Message: "troppi client"}
	wrapped := fmt.Errorf("connect: %w", italian)
	if !IsTooManyClientsError(wrapped) {
		t.Fatal("want true for wrapped 53300")
	}
	if IsTooManyClientsError(errors.New("connection refused")) {
		t.Fatal("want false for generic error")
	}
}

func TestNotifyRetryFromConfig_defaults(t *testing.T) {
	rc := NotifyRetryFromConfig(&config.Config{})
	if rc.MaxAttempts != notify.DefaultRetryConfig.MaxAttempts {
		t.Fatalf("MaxAttempts = %d", rc.MaxAttempts)
	}
}

func TestBuildSenders_allChannels(t *testing.T) {
	cfg := &config.Config{
		SlackWebhook:        "https://slack.example/hook",
		LokiURL:             "http://loki/push",
		PagerDutyEnabled:    true,
		PagerDutyRoutingKey: "key",
		TeamsEnabled:        true,
		TeamsWebhook:        "https://teams.example/hook",
		GenericEnabled:      true,
		GenericWebhookURL:   "https://generic.example/hook",
	}
	senders := BuildSenders(cfg)
	if len(senders) != 5 {
		t.Fatalf("senders = %d, want 5", len(senders))
	}
}

func TestRunContextStrings(t *testing.T) {
	SetKubeHelpers(
		func(context.Context, string) string { return "prod-cluster" },
		func(string) (string, string, error) { return "ns1", "svc/pg", nil },
	)
	cfg := &config.Config{
		Client:       "mon",
		KubePostgres: "ns1/svc/pg",
	}
	cluster, client, ns, db := RunContextStrings(context.Background(), cfg, "postgres://u:p@host/mydb?sslmode=disable")
	if cluster != "prod-cluster" || client != "mon" || ns != "ns1" || db != "mydb" {
		t.Fatalf("got %q %q %q %q", cluster, client, ns, db)
	}
}

func TestNotificationSentLine(t *testing.T) {
	ev := notify.Event{
		Stats:          postgres.ConnectionStats{Total: 90, Active: 10, Idle: 80},
		Threshold:      "total",
		ThresholdValue: 85,
		MaxConnections: 100,
		Level:          "attention",
		Cluster:        "c1",
		Client:         "cli",
		Database:       "db1",
	}
	line := NotificationSentLine(ev)
	for _, sub := range []string{"total=90", "cluster=c1", "85%", "attention"} {
		if !strings.Contains(line, sub) {
			t.Fatalf("line %q missing %q", line, sub)
		}
	}
}

func TestSendEvents_dryRun(t *testing.T) {
	cfg := &config.Config{DryRun: true}
	s := &fakeSender{}
	sent, _ := SendEvents(context.Background(), []notify.Sender{s}, cfg, []notify.Event{{Message: "x", Threshold: "test"}})
	if len(sent) != 0 || s.n != 0 {
		t.Fatalf("dry-run sent=%v sender.n=%d", sent, s.n)
	}
}

func TestSendEvents_delivers(t *testing.T) {
	cfg := &config.Config{}
	s := &fakeSender{}
	sent, _ := SendEvents(context.Background(), []notify.Sender{s}, cfg, []notify.Event{
		{Message: "alert", Threshold: "idle", Stats: postgres.ConnectionStats{Total: 1}},
	})
	if !sent["idle"] || s.n != 1 {
		t.Fatalf("sent=%v n=%d", sent, s.n)
	}
}

func TestApplyHysteresisFilter_blocksUntilConfirmed(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{ConfirmAlert: 3}
	st := &memStore{states: []string{"ok", "alert"}}
	events := []notify.Event{{Threshold: "total", Message: "m"}}
	out := ApplyHysteresisFilter(ctx, st, cfg, "c", "cl", "d", "alert", events)
	if len(out) != 0 {
		t.Fatalf("want filtered out, got %d", len(out))
	}
	st.states = []string{"alert", "alert"}
	out = ApplyHysteresisFilter(ctx, st, cfg, "c", "cl", "d", "alert", events)
	if len(out) != 1 {
		t.Fatalf("want pass-through after confirm, got %d", len(out))
	}
}

func TestApplyHysteresisFilter_longQueryAlwaysPasses(t *testing.T) {
	cfg := &config.Config{ConfirmAlert: 5}
	st := &memStore{}
	events := []notify.Event{{Threshold: "long_query", Message: "lq"}}
	out := ApplyHysteresisFilter(context.Background(), st, cfg, "c", "cl", "d", "attention", events)
	if len(out) != 1 {
		t.Fatalf("got %d events", len(out))
	}
}

func TestApplyLongQueryCooldownFilter_skipsRecent(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{LongQueryMinSeconds: 60, LongQueryCooldownSeconds: 3600}
	st := &memStore{hasLQ: true, lastLQ: time.Now()}
	events := []notify.Event{{Threshold: "long_query"}, {Threshold: "idle", Message: "i"}}
	out := ApplyLongQueryCooldownFilter(ctx, st, cfg, "c", "cl", "d", events)
	if len(out) != 1 || out[0].Threshold != "idle" {
		t.Fatalf("out = %+v", out)
	}
}

func TestDoRunCheck_levelMode(t *testing.T) {
	ctx := context.Background()
	q := &fakeQuerier{
		queryRow: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "max_connections"):
				return fakeRow{scan: func(dest ...any) error {
					*(dest[0].(*int)) = 100
					return nil
				}}
			case strings.Contains(sql, "query_start"):
				return fakeRow{scan: func(dest ...any) error { *(dest[0].(*int)) = 0; return nil }}
			case strings.Contains(sql, "backend_start"):
				return fakeRow{scan: func(dest ...any) error { *(dest[0].(*int)) = 0; return nil }}
			default:
				return fakeRow{scan: func(dest ...any) error {
					*(dest[0].(*int)) = 80
					*(dest[1].(*int)) = 10
					*(dest[2].(*int)) = 90
					return nil
				}}
			}
		},
	}
	cfg := &config.Config{Client: "c", ThresholdLevels: "75,85,95"}
	res, err := DoRunCheck(ctx, q, cfg, "", "c", "", "db")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) == 0 {
		t.Fatal("expected level mode event")
	}
}

func TestApplyThresholdDefaults_explicit(t *testing.T) {
	ctx := context.Background()
	q := &fakeQuerier{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*int)) = 200
				return nil
			}}
		},
	}
	cfg := &config.Config{ThresholdTotal: 150}
	if err := ApplyThresholdDefaults(ctx, q, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestLogDryRunStats_smoke(t *testing.T) {
	LogDryRunStats("cl", "c", "db", RunCheckResult{
		Stats: postgres.ConnectionStats{Total: 1, Active: 1, Idle: 0}, MaxConn: 100,
	})
}

func TestTrySendResolutionNotification_sends(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{ConfirmOk: 2}
	st := &memStore{states: []string{"ok", "ok", "alert"}}
	s := &fakeSender{}
	res := RunCheckResult{Stats: postgres.ConnectionStats{Total: 5, Active: 1, Idle: 4}, MaxConn: 100}
	TrySendResolutionNotification(ctx, st, cfg, []notify.Sender{s}, res, "cl", "c", "ns", "db")
	if s.n != 1 {
		t.Fatalf("sender calls = %d", s.n)
	}
}

func TestNotifyConnectFailure_noSenders(t *testing.T) {
	NotifyConnectFailure(context.Background(), nil, &config.Config{}, "", "", "", "", errors.New("fail"))
}

func TestNotifyConnectFailure_tooManyClients(t *testing.T) {
	s := &fakeSender{}
	err533 := &pgconn.PgError{Code: "53300"}
	NotifyConnectFailure(context.Background(), []notify.Sender{s}, &config.Config{}, "", "c", "", "db", err533)
	if s.n != 1 {
		t.Fatalf("n=%d", s.n)
	}
}

func TestCollectEvents_forceNotification(t *testing.T) {
	ctx := context.Background()
	q := statsQuerierMock(100, 80, 10, 90, 0, 0)
	cfg := &config.Config{ForceNotification: true, ThresholdLevels: "75,85,95"}
	res, err := DoRunCheck(ctx, q, cfg, "", "c", "", "db")
	if err != nil {
		t.Fatal(err)
	}
	foundTest := false
	for _, e := range res.Events {
		if e.Threshold == "test" {
			foundTest = true
		}
	}
	if !foundTest {
		t.Fatal("missing force-notification event")
	}
}

func TestDoRunCheck_staleThreshold(t *testing.T) {
	ctx := context.Background()
	q := statsQuerierMock(100, 80, 10, 90, 5, 0)
	cfg := &config.Config{
		StaleAge:        60,
		ThresholdStale:  3,
		ThresholdLevels: "75,85,95",
	}
	res, err := DoRunCheck(ctx, q, cfg, "", "c", "", "db")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range res.Events {
		if e.Threshold == "stale" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected stale event")
	}
}

func TestDoRunCheck_idleThreshold(t *testing.T) {
	ctx := context.Background()
	q := statsQuerierMock(100, 10, 10, 20, 0, 0)
	cfg := &config.Config{ThresholdIdle: 5, ThresholdTotal: 1000, ThresholdActive: 1000}
	res, err := DoRunCheck(ctx, q, cfg, "", "c", "", "db")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range res.Events {
		if e.Threshold == "idle" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected idle event")
	}
}

func TestMakeRunFunc_insertsAndRuns(t *testing.T) {
	ctx := context.Background()
	q := statsQuerierMock(100, 80, 10, 90, 0, 0)
	cfg := &config.Config{
		ThresholdLevels: "75,85,95",
		DryRun:          true,
	}
	st := &memStore{}
	fn := MakeRunFunc(ctx, q, cfg, nil, st, "cl", "c", "ns", "db")
	out := fn()
	if out.QueryFailed || out.DeliveryFailed {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestMakeRunFunc_queryFailed(t *testing.T) {
	ctx := context.Background()
	q := &fakeQuerier{
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{scan: func(...any) error { return errors.New("query boom") }}
		},
	}
	fn := MakeRunFunc(ctx, q, &config.Config{DryRun: true}, nil, nil, "", "c", "", "db")
	out := fn()
	if !out.QueryFailed {
		t.Fatal("want QueryFailed")
	}
	if out.DeliveryFailed {
		t.Fatal("QueryFailed path must not set DeliveryFailed")
	}
}

func TestNotificationSentLine_cases(t *testing.T) {
	cases := []notify.Event{
		{Threshold: "test", Stats: postgres.ConnectionStats{}},
		{Threshold: "connect_failure", Stats: postgres.ConnectionStats{}},
		{Threshold: "too_many_clients", Stats: postgres.ConnectionStats{}},
		{Threshold: "resolution", Stats: postgres.ConnectionStats{}},
		{Threshold: "total", ThresholdValue: 10, MaxConnections: 100, Level: "alert", Stats: postgres.ConnectionStats{Total: 10}},
	}
	for _, ev := range cases {
		if NotificationSentLine(ev) == "" {
			t.Fatalf("empty line for %q", ev.Threshold)
		}
	}
}

func TestApplyLongQueryCooldownFilter_noStore(t *testing.T) {
	cfg := &config.Config{LongQueryMinSeconds: 10}
	events := []notify.Event{{Threshold: "long_query"}}
	out := ApplyLongQueryCooldownFilter(context.Background(), nil, cfg, "c", "cl", "d", events)
	if len(out) != 1 {
		t.Fatalf("got %d", len(out))
	}
}

func TestApplyHysteresisFilter_nilStore(t *testing.T) {
	cfg := &config.Config{ConfirmAlert: 3}
	events := []notify.Event{{Threshold: "total"}}
	out := ApplyHysteresisFilter(context.Background(), nil, cfg, "c", "cl", "d", "alert", events)
	if len(out) != 1 {
		t.Fatalf("got %d", len(out))
	}
}

func statsQuerierMock(maxConn, active, idle, total, stale, longQ int) *fakeQuerier {
	return &fakeQuerier{
		queryRow: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "max_connections"):
				return fakeRow{scan: func(dest ...any) error {
					*(dest[0].(*int)) = maxConn
					return nil
				}}
			case strings.Contains(sql, "query_start"):
				return fakeRow{scan: func(dest ...any) error {
					*(dest[0].(*int)) = longQ
					return nil
				}}
			case strings.Contains(sql, "backend_start"):
				return fakeRow{scan: func(dest ...any) error {
					*(dest[0].(*int)) = stale
					return nil
				}}
			default:
				return fakeRow{scan: func(dest ...any) error {
					*(dest[0].(*int)) = active
					*(dest[1].(*int)) = idle
					*(dest[2].(*int)) = total
					return nil
				}}
			}
		},
	}
}
