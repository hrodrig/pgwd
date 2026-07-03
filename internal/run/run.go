// Package run implements the per-check orchestration: event collection, hysteresis,
// notification delivery, and resolution alerts. Extracted from cmd/pgwd for test coverage.
package run

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/hrodrig/pgwd/internal/checker"
	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/postgres"
	"github.com/hrodrig/pgwd/internal/store"
	"github.com/jackc/pgx/v5/pgconn"
)

// RunContextStrings derives alert and metrics-store identity fields for one check run.
func RunContextStrings(ctx context.Context, cfg *config.Config, dbURL string) (cluster, client, namespace, database string) {
	if cfg.KubePostgres != "" {
		cluster = kubeClusterName(ctx, cfg.KubeContext)
	}
	client = cfg.Client
	if cfg.KubePostgres != "" {
		if ns, _, err := parseKubePostgres(cfg.KubePostgres); err == nil {
			namespace = ns
		}
	}
	if u, err := url.Parse(dbURL); err == nil && u.Path != "" {
		database = strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
	}
	return cluster, client, namespace, database
}

// kubeClusterName and parseKubePostgres are thin wrappers so run tests avoid importing kube
// with client-go side effects in table tests (ClusterName still used from kube in production).
var (
	kubeClusterName   = func(ctx context.Context, kubeContext string) string { return "" }
	parseKubePostgres = func(input string) (namespace, resource string, err error) { return "", "", nil }
)

// SetKubeHelpers wires Kubernetes helpers from internal/kube (call from cmd/pgwd init).
func SetKubeHelpers(clusterName func(context.Context, string) string, parsePostgres func(string) (string, string, error)) {
	if clusterName != nil {
		kubeClusterName = clusterName
	}
	if parsePostgres != nil {
		parseKubePostgres = parsePostgres
	}
}

// BuildSenders constructs notification backends configured in cfg.
func BuildSenders(cfg *config.Config) []notify.Sender {
	notify.ApplyRetryConfig(NotifyRetryFromConfig(cfg))
	var senders []notify.Sender
	if cfg.SlackWebhook != "" {
		senders = append(senders, &notify.Slack{WebhookURL: cfg.SlackWebhook})
	}
	if cfg.LokiURL != "" {
		senders = append(senders, &notify.Loki{
			URL:         cfg.LokiURL,
			Labels:      notify.ParseLokiLabels(cfg.LokiLabels),
			OrgID:       cfg.LokiOrgID,
			BearerToken: cfg.LokiBearerToken,
		})
	}
	if cfg.PagerDutyActive() {
		senders = append(senders, &notify.PagerDuty{
			RoutingKey: cfg.PagerDutyRoutingKey,
			Severity:   cfg.PagerDutySeverity,
			Source:     cfg.PagerDutySource,
		})
	}
	if cfg.TeamsActive() {
		senders = append(senders, &notify.Teams{WebhookURL: cfg.TeamsWebhook})
	}
	if cfg.GenericActive() {
		senders = append(senders, &notify.GenericWebhook{
			WebhookURL:   cfg.GenericWebhookURL,
			JSONKey:      cfg.GenericJSONKey,
			Headers:      cfg.GenericHeaders,
			ExtraFields:  cfg.GenericExtraFields,
			BodyTemplate: cfg.GenericBodyTemplate,
			HMACSecret:   cfg.GenericHMACSecret,
			HMACHeader:   cfg.GenericHMACHeader,
		})
	}
	return senders
}

// NotifyRetryFromConfig maps cfg retry fields to notify.RetryConfig with defaults.
func NotifyRetryFromConfig(cfg *config.Config) notify.RetryConfig {
	rc := notify.RetryConfig{
		MaxAttempts:    cfg.RetryMaxAttempts,
		InitialBackoff: cfg.RetryInitialBackoff,
		MaxBackoff:     cfg.RetryMaxBackoff,
	}
	if rc.MaxAttempts <= 0 {
		rc.MaxAttempts = notify.DefaultRetryConfig.MaxAttempts
	}
	if rc.InitialBackoff <= 0 {
		rc.InitialBackoff = notify.DefaultRetryConfig.InitialBackoff
	}
	if rc.MaxBackoff <= 0 {
		rc.MaxBackoff = notify.DefaultRetryConfig.MaxBackoff
	}
	return rc
}

// IsTooManyClientsError reports whether err is Postgres SQLSTATE 53300.
func IsTooManyClientsError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "53300"
}

// NotifyConnectFailure sends an infrastructure alert when Postgres cannot be reached.
func NotifyConnectFailure(ctx context.Context, senders []notify.Sender, cfg *config.Config, cluster, client, ns, db string, connectErr error) {
	if len(senders) == 0 {
		return
	}
	log.Printf("Sending notification…")
	tooManyClients := IsTooManyClientsError(connectErr)
	ev := notify.Event{
		Stats:          postgres.ConnectionStats{},
		Threshold:      "connect_failure",
		ThresholdValue: 0,
		Message:        "pgwd could not connect to Postgres. Check database URL, connectivity, credentials, or infrastructure.",
		Cluster:        cluster,
		Client:         client,
		Namespace:      ns,
		Database:       db,
	}
	if tooManyClients {
		ev.Threshold = "too_many_clients"
		ev.Message = "Postgres rejected connection: too many clients already (max_connections exceeded). Database is saturated — urgent."
	}
	sent := 0
	for _, s := range senders {
		if sendErr := s.Send(ctx, ev); sendErr != nil {
			log.Printf("notify (connect failure): %v", sendErr)
		} else {
			sent++
		}
	}
	if sent > 0 {
		log.Printf("Notification sent")
	}
}

// ApplyThresholdDefaults reads max_connections, fills zero thresholds, and validates config.
func ApplyThresholdDefaults(ctx context.Context, pool postgres.Querier, cfg *config.Config) error {
	maxConn, maxConnErr := postgres.MaxConnections(ctx, pool)
	if cfg.TestMaxConnections > 0 {
		maxConn = cfg.TestMaxConnections
	}
	if !cfg.UsesLevelMode() && maxConn > 0 {
		checker.ApplySingleThresholdDefaults(cfg, maxConn)
	}
	return checker.ValidateThresholdConfig(cfg, maxConn, maxConnErr)
}

// RunCheckResult is returned by DoRunCheck for store integration.
type RunCheckResult struct {
	Stats              postgres.ConnectionStats
	MaxConn            int
	Events             []notify.Event
	StaleCount         int
	StaleCountForStore int
}

// DoRunCheck reads connection stats from Postgres and collects threshold events.
func DoRunCheck(ctx context.Context, pool postgres.Querier, cfg *config.Config, cluster, client, ns, db string) (RunCheckResult, error) {
	var res RunCheckResult
	stats, err := postgres.Stats(ctx, pool)
	if err != nil {
		return res, err
	}
	res.Stats = stats
	res.MaxConn, _ = postgres.MaxConnections(ctx, pool)
	if cfg.TestMaxConnections > 0 {
		res.MaxConn = cfg.TestMaxConnections
	}
	if cfg.StaleAge > 0 {
		res.StaleCount, _ = postgres.StaleCount(ctx, pool, cfg.StaleAge)
	}
	staleAgeForStore := cfg.SqliteStaleAge
	if staleAgeForStore <= 0 {
		staleAgeForStore = cfg.StaleAge
	}
	if staleAgeForStore > 0 {
		res.StaleCountForStore, _ = postgres.StaleCount(ctx, pool, staleAgeForStore)
	} else {
		res.StaleCountForStore = res.StaleCount
	}
	res.Events = collectEvents(ctx, pool, cfg, stats, res.MaxConn, cluster, client, ns, db)
	return res, nil
}

func collectEvents(ctx context.Context, pool postgres.Querier, cfg *config.Config, stats postgres.ConnectionStats, maxConn int, cluster, client, ns, db string) []notify.Event {
	var events []notify.Event
	ev := checker.BaseEvent(stats, maxConn, cfg.TestMaxConnections > 0, cluster, client, ns, db)

	if cfg.ThresholdStale > 0 && cfg.StaleAge > 0 {
		if e := collectStaleEvent(ctx, pool, cfg, ev); e != nil {
			events = append(events, *e)
		}
	}
	if cfg.LongQueryMinSeconds > 0 {
		if e := collectLongQueryEvent(ctx, pool, cfg, ev); e != nil {
			events = append(events, *e)
		}
	}
	if cfg.UsesLevelMode() && maxConn > 0 {
		if e := checker.CollectLevelModeEvent(ev, cfg, stats, maxConn); e != nil {
			events = append(events, *e)
		}
	} else {
		events = append(events, checker.CollectExplicitThresholdEvents(ev, cfg, stats, maxConn)...)
	}
	if cfg.ThresholdIdle > 0 && stats.Idle >= cfg.ThresholdIdle {
		e := ev
		e.Threshold = "idle"
		e.ThresholdValue = cfg.ThresholdIdle
		e.Message = fmt.Sprintf("Idle connections %d >= %d", stats.Idle, cfg.ThresholdIdle)
		events = append(events, e)
	}
	if cfg.ForceNotification {
		e := ev
		e.Threshold = "test"
		e.ThresholdValue = 0
		e.Message = "Test notification — delivery check (force-notification)."
		events = append(events, e)
	}
	return events
}

func collectStaleEvent(ctx context.Context, pool postgres.Querier, cfg *config.Config, ev notify.Event) *notify.Event {
	staleCount, err := postgres.StaleCount(ctx, pool, cfg.StaleAge)
	if err != nil {
		log.Printf("stale count: %v", err)
		return nil
	}
	if staleCount < cfg.ThresholdStale {
		return nil
	}
	e := ev
	e.Threshold = "stale"
	e.ThresholdValue = cfg.ThresholdStale
	e.Message = fmt.Sprintf("Stale connections (open > %ds): %d >= %d", cfg.StaleAge, staleCount, cfg.ThresholdStale)
	return &e
}

func collectLongQueryEvent(ctx context.Context, pool postgres.Querier, cfg *config.Config, ev notify.Event) *notify.Event {
	n, err := postgres.LongQueryCount(ctx, pool, cfg.LongQueryMinSeconds)
	if err != nil {
		log.Printf("long query count: %v", err)
		return nil
	}
	if n < cfg.LongQueryMinCount {
		return nil
	}
	e := ev
	e.Threshold = "long_query"
	e.ThresholdValue = cfg.LongQueryMinCount
	e.Level = "attention"
	e.Message = fmt.Sprintf("Long-running queries (active, runtime > %ds): %d >= %d", cfg.LongQueryMinSeconds, n, cfg.LongQueryMinCount)
	return &e
}

// SendEvents delivers each event to all senders. Returns thresholds for which at least one send succeeded.
func SendEvents(ctx context.Context, senders []notify.Sender, cfg *config.Config, events []notify.Event) map[string]bool {
	sent := make(map[string]bool)
	for _, ev := range events {
		if cfg.DryRun {
			log.Printf("[dry-run] would send: %s", ev.Message)
			continue
		}
		n := 0
		for _, s := range senders {
			if err := s.Send(ctx, ev); err != nil {
				log.Printf("notify: %v", err)
			} else {
				n++
			}
		}
		if n > 0 {
			sent[ev.Threshold] = true
			log.Printf("Notification sent: %s", notificationSentLine(ev))
		}
	}
	return sent
}

func notificationSentLine(ev notify.Event) string {
	line := fmt.Sprintf("%s %s | total=%d active=%d idle=%d",
		notificationSentPrefix(ev), notificationSentMessage(ev),
		ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle)
	line += notificationSentMaxConnSuffix(ev)
	line += notificationSentThresholdSuffix(ev)
	return line
}

func notificationSentPrefix(ev notify.Event) string {
	if ev.Cluster == "" && ev.Database == "" && ev.Client == "" {
		return "pgwd:"
	}
	parts := make([]string, 0, 3)
	if ev.Cluster != "" {
		parts = append(parts, fmt.Sprintf("cluster=%s", ev.Cluster))
	}
	if ev.Database != "" {
		parts = append(parts, fmt.Sprintf("database=%s", ev.Database))
	}
	if ev.Client != "" {
		parts = append(parts, fmt.Sprintf("client=%s", ev.Client))
	}
	return fmt.Sprintf("pgwd [%s]:", strings.Join(parts, " "))
}

func notificationSentMessage(ev notify.Event) string {
	switch ev.Threshold {
	case "total":
		return fmt.Sprintf("Total connections %d >= %d", ev.Stats.Total, ev.ThresholdValue)
	case "active":
		return fmt.Sprintf("Active connections %d >= %d", ev.Stats.Active, ev.ThresholdValue)
	default:
		return ev.Message
	}
}

func notificationSentMaxConnSuffix(ev notify.Event) string {
	if ev.MaxConnections <= 0 {
		return ""
	}
	s := fmt.Sprintf(" max_connections=%d", ev.MaxConnections)
	if ev.MaxConnectionsIsOverride {
		s += " (test override)"
	}
	return s
}

func notificationSentThresholdSuffix(ev notify.Event) string {
	switch ev.Threshold {
	case "test":
		return " (delivery check)"
	case "connect_failure":
		return " (connection failed)"
	case "too_many_clients":
		return " (too many clients — DB saturated)"
	case "resolution":
		return " (returned to normal)"
	default:
		extra := ""
		if ev.Level != "" && ev.MaxConnections > 0 && ev.ThresholdValue > 0 &&
			(ev.Threshold == "total" || ev.Threshold == "active") {
			pct := ev.ThresholdValue * 100 / ev.MaxConnections
			extra = fmt.Sprintf(", %d%%, %s", pct, ev.Level)
		}
		return fmt.Sprintf(" (limit %s=%d%s)", ev.Threshold, ev.ThresholdValue, extra)
	}
}

// ApplyHysteresisFilter suppresses threshold alerts until the same state has been seen consecutively.
func ApplyHysteresisFilter(ctx context.Context, st store.MetricsStorer, cfg *config.Config, client, cluster, db, state string, events []notify.Event) []notify.Event {
	if st == nil || cfg.ConfirmAlert <= 1 || state == "ok" {
		return events
	}
	last, _ := st.LastStates(ctx, client, cluster, db, cfg.ConfirmAlert-1)
	if len(last) >= cfg.ConfirmAlert-1 && checker.AllStringsEqual(last, state) {
		return events
	}
	var filtered []notify.Event
	for _, e := range events {
		if e.Threshold == "connect_failure" || e.Threshold == "too_many_clients" || e.Threshold == "long_query" {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// ApplyLongQueryCooldownFilter drops long_query events within the cooldown window.
func ApplyLongQueryCooldownFilter(ctx context.Context, st store.MetricsStorer, cfg *config.Config, client, cluster, db string, events []notify.Event) []notify.Event {
	if cfg.LongQueryMinSeconds <= 0 || st == nil {
		return events
	}
	cd, ok := st.(store.AlertCooldownRecorder)
	if !ok {
		return events
	}
	var out []notify.Event
	for _, e := range events {
		if e.Threshold != "long_query" {
			out = append(out, e)
			continue
		}
		last, has, err := cd.LastLongQueryAlert(ctx, client, cluster, db)
		if err != nil {
			log.Printf("long_query cooldown read: %v", err)
			out = append(out, e)
			continue
		}
		if has && time.Since(last) < time.Duration(cfg.LongQueryCooldownSeconds)*time.Second {
			continue
		}
		out = append(out, e)
	}
	return out
}

// TrySendResolutionNotification sends a resolution event when state returns to ok.
func TrySendResolutionNotification(ctx context.Context, st store.MetricsStorer, cfg *config.Config, senders []notify.Sender, res RunCheckResult, cluster, client, ns, db string) {
	if st == nil || cfg.ConfirmOk < 1 {
		return
	}
	last, _ := st.LastStates(ctx, client, cluster, db, cfg.ConfirmOk+1)
	if len(last) < cfg.ConfirmOk+1 {
		return
	}
	for i := 0; i < cfg.ConfirmOk && i < len(last); i++ {
		if last[i] != "ok" {
			return
		}
	}
	if last[cfg.ConfirmOk] == "ok" || last[cfg.ConfirmOk] == "" {
		return
	}
	ev := notify.Event{
		Stats:          res.Stats,
		Threshold:      "resolution",
		ThresholdValue: 0,
		Message:        fmt.Sprintf("PostgreSQL connections returned to normal. total=%d active=%d idle=%d", res.Stats.Total, res.Stats.Active, res.Stats.Idle),
		Level:          "ok",
		MaxConnections: res.MaxConn,
		Cluster:        cluster,
		Client:         client,
		Namespace:      ns,
		Database:       db,
	}
	SendEvents(ctx, senders, cfg, []notify.Event{ev})
}

// MakeRunFunc returns the per-interval check closure for one target.
func MakeRunFunc(ctx context.Context, pool postgres.Querier, cfg *config.Config, senders []notify.Sender, st store.MetricsStorer, cluster, client, ns, db string) func() {
	return func() {
		res, err := DoRunCheck(ctx, pool, cfg, cluster, client, ns, db)
		if err != nil {
			log.Printf("stats: %v", err)
			return
		}
		if cfg.DryRun && cfg.LogLevel == "debug" {
			LogDryRunStats(cluster, client, db, res)
		}
		statePre, _ := checker.StateAndThresholdFromEvents(res.Events)
		res.Events = ApplyHysteresisFilter(ctx, st, cfg, client, cluster, db, statePre, res.Events)
		res.Events = ApplyLongQueryCooldownFilter(ctx, st, cfg, client, cluster, db, res.Events)
		state, thr := checker.StateAndThresholdFromEvents(res.Events)
		sent := SendEvents(ctx, senders, cfg, res.Events)
		if sent["long_query"] && st != nil {
			if cd, ok := st.(store.AlertCooldownRecorder); ok {
				if err := cd.SetLongQueryAlert(ctx, client, cluster, db, time.Now()); err != nil {
					log.Printf("long_query cooldown write: %v", err)
				}
			}
		}
		if st != nil {
			rec := store.Record{
				Client: client, Cluster: cluster, Namespace: ns, Database: db,
				Total: res.Stats.Total, Active: res.Stats.Active, Idle: res.Stats.Idle, Stale: res.StaleCountForStore,
				MaxConnections: res.MaxConn, State: state, Threshold: thr,
			}
			if err := st.Insert(ctx, rec); err != nil {
				log.Printf("store insert: %v", err)
			} else if state == "ok" {
				TrySendResolutionNotification(ctx, st, cfg, senders, res, cluster, client, ns, db)
			}
		}
	}
}

// LogDryRunStats logs connection counts when -dry-run and log-level=debug.
func LogDryRunStats(cluster, client, database string, res RunCheckResult) {
	parts := []string{client}
	if database != "" {
		parts = append(parts, database)
	}
	target := strings.Join(parts, "/")
	if cluster != "" {
		target = cluster + "/" + target
	}
	if res.MaxConn > 0 {
		log.Printf("[%s] total=%d active=%d idle=%d max_connections=%d", target, res.Stats.Total, res.Stats.Active, res.Stats.Idle, res.MaxConn)
	} else {
		log.Printf("[%s] total=%d active=%d idle=%d", target, res.Stats.Total, res.Stats.Active, res.Stats.Idle)
	}
}

// NotificationSentLine exposes the log line formatter for tests.
func NotificationSentLine(ev notify.Event) string {
	return notificationSentLine(ev)
}
