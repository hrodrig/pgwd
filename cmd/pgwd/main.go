// Package main is the entry point for pgwd (Postgres Watch Dog), a Go CLI that
// checks PostgreSQL connection counts (active/idle) and notifies via Slack and/or
// Loki when thresholds are exceeded. It can also alert on stale connections.
// See the README and github.com/hrodrig/pgwd for usage and install.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hrodrig/pgwd/internal/checker"
	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/httpsrv"
	"github.com/hrodrig/pgwd/internal/kube"
	"github.com/hrodrig/pgwd/internal/metricsexport"
	"github.com/hrodrig/pgwd/internal/metricsstore"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/openbsd"
	"github.com/hrodrig/pgwd/internal/postgres"
	"github.com/hrodrig/pgwd/internal/store"
	"github.com/hrodrig/pgwd/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Set at build time via -ldflags (see Makefile).
var (
	Version   string = "dev"
	Commit    string = ""
	BuildDate string = ""
)

func printVersion() {
	commit := Commit
	if commit == "" {
		commit = "unknown"
	}
	built := BuildDate
	if built == "" {
		built = "unknown"
	}
	fmt.Printf("pgwd %s (commit %s, built %s)\n", Version, commit, built)
}

// handleVersion checks os.Args for "version"/"-version"/"--version"; prints version and exits if matched.
func handleVersion() {
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "-version" || os.Args[1] == "--version") {
		printVersion()
		os.Exit(0)
	}
}

func parseFlags(cfg *config.Config) (showVersion bool) {
	showVersionFlag := flag.Bool("version", false, "print version and exit")
	configPath := config.ConfigPath()
	flag.StringVar(&configPath, "config", configPath, "Config file path (PGWD_CONFIG); default /etc/pgwd/pgwd.conf")
	flag.StringVar(&cfg.DBURL, "db-url", cfg.DBURL, "PostgreSQL connection URL (PGWD_DB_URL)")
	flag.IntVar(&cfg.ThresholdTotal, "db-threshold-total", cfg.ThresholdTotal, "Alert when total connections >= N (PGWD_DB_THRESHOLD_TOTAL). Deprecated: use -db-threshold-levels; will be removed in v1.0.0.")
	flag.IntVar(&cfg.ThresholdActive, "db-threshold-active", cfg.ThresholdActive, "Alert when active connections >= N (PGWD_DB_THRESHOLD_ACTIVE). Deprecated: use -db-threshold-levels; will be removed in v1.0.0.")
	flag.IntVar(&cfg.ThresholdIdle, "db-threshold-idle", cfg.ThresholdIdle, "Alert when idle connections >= N (PGWD_DB_THRESHOLD_IDLE)")
	flag.IntVar(&cfg.StaleAge, "db-stale-age", cfg.StaleAge, "Consider connection stale if open longer than N seconds (PGWD_DB_STALE_AGE)")
	flag.IntVar(&cfg.ThresholdStale, "db-threshold-stale", cfg.ThresholdStale, "Alert when stale connections (open > stale-age) >= N (PGWD_DB_THRESHOLD_STALE)")
	flag.IntVar(&cfg.LongQueryMinSeconds, "db-long-query-min-seconds", cfg.LongQueryMinSeconds, "Alert on active queries running longer than N seconds; 0=off. Requires metrics store; uses cooldown (PGWD_DB_LONG_QUERY_MIN_SECONDS)")
	flag.IntVar(&cfg.LongQueryCooldownSeconds, "db-long-query-cooldown-seconds", cfg.LongQueryCooldownSeconds, "Min seconds between long_query notifications per target; default 3600 when min-seconds set (PGWD_DB_LONG_QUERY_COOLDOWN_SECONDS)")
	flag.IntVar(&cfg.LongQueryMinCount, "db-long-query-min-count", cfg.LongQueryMinCount, "Alert when count of long-running queries >= N; default 1 (PGWD_DB_LONG_QUERY_MIN_COUNT)")
	flag.StringVar(&cfg.SlackWebhook, "notifications-slack-webhook", cfg.SlackWebhook, "Slack Incoming Webhook URL (PGWD_NOTIFICATIONS_SLACK_WEBHOOK)")
	flag.StringVar(&cfg.LokiURL, "notifications-loki-url", cfg.LokiURL, "Loki push API URL, e.g. http://localhost:3100/loki/api/v1/push (PGWD_NOTIFICATIONS_LOKI_URL)")
	flag.StringVar(&cfg.LokiLabels, "notifications-loki-labels", cfg.LokiLabels, "Loki labels, e.g. app=pgwd,env=prod (PGWD_NOTIFICATIONS_LOKI_LABELS)")
	flag.StringVar(&cfg.LokiOrgID, "notifications-loki-org-id", cfg.LokiOrgID, "Loki X-Scope-OrgID header (multi-tenancy); for 401 Unauthorized (PGWD_NOTIFICATIONS_LOKI_ORG_ID)")
	flag.StringVar(&cfg.LokiBearerToken, "notifications-loki-bearer-token", cfg.LokiBearerToken, "Loki Authorization: Bearer token (PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN)")
	flag.IntVar(&cfg.Interval, "interval", cfg.Interval, "Run every N seconds; 0 = run once (PGWD_INTERVAL)")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Only print, do not send notifications (PGWD_DRY_RUN)")
	flag.BoolVar(&cfg.ForceNotification, "force-notification", cfg.ForceNotification, "Always send a test notification to validate delivery/format (PGWD_FORCE_NOTIFICATION)")
	flag.IntVar(&cfg.DefaultThresholdPercent, "db-default-threshold-percent", cfg.DefaultThresholdPercent, "When one of total/active is 0, set it to this % of max_connections (1-100, default 80) (PGWD_DB_DEFAULT_THRESHOLD_PERCENT)")
	flag.StringVar(&cfg.ThresholdLevels, "db-threshold-levels", cfg.ThresholdLevels, "When both total and active are 0: comma-separated percentages for 3-tier alerts, e.g. 75,85,95 (attention/alert/danger). Only highest level fires. (PGWD_DB_THRESHOLD_LEVELS)")
	flag.StringVar(&cfg.KubePostgres, "kube-postgres", cfg.KubePostgres, "Connect via port-forward (client-go): namespace/type/name (e.g. default/svc/postgres) (PGWD_KUBE_POSTGRES)")
	flag.StringVar(&cfg.KubeLoki, "kube-loki", cfg.KubeLoki, "Connect to Loki via port-forward when Loki is inside the cluster: namespace/type/name (e.g. monitoring/svc/loki) (PGWD_KUBE_LOKI)")
	flag.StringVar(&cfg.KubeContext, "kube-context", cfg.KubeContext, "Kubectl context to use (empty = current context) (PGWD_KUBE_CONTEXT)")
	flag.IntVar(&cfg.KubeLocalPort, "kube-local-port", cfg.KubeLocalPort, "Local port for kube port-forward (default 5432) (PGWD_KUBE_LOCAL_PORT)")
	flag.IntVar(&cfg.KubeLokiLocalPort, "kube-loki-local-port", cfg.KubeLokiLocalPort, "Local port for Loki port-forward (default 3100) (PGWD_KUBE_LOKI_LOCAL_PORT)")
	flag.IntVar(&cfg.KubeLokiRemotePort, "kube-loki-remote-port", cfg.KubeLokiRemotePort, "Remote port on the Loki service (default 3100) (PGWD_KUBE_LOKI_REMOTE_PORT)")
	flag.StringVar(&cfg.KubePasswordVar, "kube-password-var", cfg.KubePasswordVar, "Pod env var for password when URL has DISCOVER_MY_PASSWORD (default POSTGRES_PASSWORD) (PGWD_KUBE_PASSWORD_VAR)")
	flag.StringVar(&cfg.KubePasswordContainer, "kube-password-container", cfg.KubePasswordContainer, "Container name in pod for password discovery (PGWD_KUBE_PASSWORD_CONTAINER)")
	flag.StringVar(&cfg.Client, "client", cfg.Client, "Client name for this monitor instance — REQUIRED (PGWD_CLIENT); identifies which monitor sent the alert")
	flag.BoolVar(&cfg.NotifyOnConnectFailure, "notify-on-connect-failure", cfg.NotifyOnConnectFailure, "Send an alert to notifiers when Postgres connection fails (infrastructure alert) (PGWD_NOTIFY_ON_CONNECT_FAILURE)")
	flag.IntVar(&cfg.TestMaxConnections, "test-max-connections", cfg.TestMaxConnections, "Override server max_connections for defaults and display (for testing alerts; 0 = use server) (PGWD_TEST_MAX_CONNECTIONS)")
	flag.BoolVar(&cfg.ValidateK8sAccess, "validate-k8s-access", cfg.ValidateK8sAccess, "Validate cluster connectivity and list pods, then exit. Use -kube-context to select context. (PGWD_VALIDATE_K8S_ACCESS)")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: info (default) or debug. Debug = verbose dry-run stats every interval (PGWD_LOG_LEVEL)")
	flag.StringVar(&cfg.ExportMetricsFormat, "export-metrics-format", cfg.ExportMetricsFormat, "Export persisted metrics from the configured metrics store; format: csv (file). Then exit (PGWD_EXPORT_METRICS_FORMAT)")
	flag.StringVar(&cfg.ExportMetricsDestination, "export-metrics-destination", cfg.ExportMetricsDestination, "Output path for -export-metrics-format csv (PGWD_EXPORT_METRICS_DESTINATION)")
	flag.Parse()
	return *showVersionFlag
}

func validateConfig(cfg *config.Config) {
	if err := validator.Validate(cfg); err != nil {
		log.Fatal(err)
	}
}

func exportMetricsAndExit(cfg *config.Config) {
	format := strings.TrimSpace(cfg.ExportMetricsFormat)
	dest := strings.TrimSpace(cfg.ExportMetricsDestination)
	if format == "" || dest == "" {
		log.Fatal("pgwd: -export-metrics-format and -export-metrics-destination must both be set (e.g. -export-metrics-format csv -export-metrics-destination /path/out.csv)")
	}
	ctx := context.Background()
	n, err := metricsexport.Export(ctx, format, dest, cfg)
	if err != nil {
		log.Fatalf("pgwd: metrics export: %v", err)
	}
	log.Printf("pgwd: exported %d metrics row(s) via format=%q to %s", n, format, dest)
	os.Exit(0)
}

// setupKube starts port-forward and updates cfg.DBURL when -kube-postgres is set.
// Returns a cleanup function that must be called on exit (e.g. defer in main).
func setupKube(ctx context.Context, cfg *config.Config) (cleanup func()) {
	if cfg.KubePostgres == "" {
		return func() {}
	}
	if err := kube.RequireKubectl(); err != nil {
		log.Fatalf("kube-postgres: %v", err)
	}
	namespace, resource, err := kube.ParseKubePostgres(cfg.KubePostgres)
	if err != nil {
		log.Fatalf("kube-postgres: %v", err)
	}
	if cfg.KubeLocalPort < 1 || cfg.KubeLocalPort > 65535 {
		log.Fatal("kube-local-port must be between 1 and 65535")
	}
	password := ""
	if kube.URLContainsDiscoverPassword(cfg.DBURL) {
		podName, err := kube.ResolvePod(ctx, cfg.KubeContext, namespace, resource)
		if err != nil {
			log.Fatalf("kube resolve pod: %v", err)
		}
		password, err = kube.GetPasswordFromPod(ctx, cfg.KubeContext, namespace, podName, cfg.KubePasswordContainer, cfg.KubePasswordVar)
		if err != nil {
			log.Fatal("kube: could not get password from pod (check namespace, pod name, container, and env var)")
		}
	}
	finalURL, err := kube.ReplaceDBURLForKube(cfg.DBURL, password, cfg.KubeLocalPort)
	if err != nil {
		log.Fatal("kube: failed to build DB URL (check -db-url format)")
	}
	cfg.DBURL = finalURL
	cleanup, err = kube.StartPortForward(ctx, cfg.KubeContext, namespace, resource, cfg.KubeLocalPort)
	if err != nil {
		log.Fatalf("kube port-forward: %v", err)
	}
	return cleanup
}

// setupKubeLoki starts port-forward to Loki when -kube-loki is set and LokiURL is empty.
// Sets cfg.LokiURL to localhost:port. Returns a cleanup function. Call it on exit.
func setupKubeLoki(ctx context.Context, cfg *config.Config) (cleanup func()) {
	if cfg.KubeLoki == "" || cfg.LokiURL != "" {
		return func() {}
	}
	if err := kube.RequireKubectl(); err != nil {
		log.Fatalf("kube-loki: %v", err)
	}
	namespace, resource, err := kube.ParseKubePostgres(cfg.KubeLoki)
	if err != nil {
		log.Fatalf("kube-loki: %v", err)
	}
	cfg.LokiURL = fmt.Sprintf("http://127.0.0.1:%d/loki/api/v1/push", cfg.KubeLokiLocalPort)
	cleanup, err = kube.StartPortForwardTo(ctx, cfg.KubeContext, namespace, resource, cfg.KubeLokiLocalPort, cfg.KubeLokiRemotePort)
	if err != nil {
		log.Fatalf("kube-loki port-forward: %v", err)
	}
	return cleanup
}

func runContextStrings(ctx context.Context, cfg *config.Config, dbURL string) (cluster, client, namespace, database string) {
	if cfg.KubePostgres != "" {
		cluster = kube.ClusterName(ctx, cfg.KubeContext)
	}
	client = cfg.Client
	if cfg.KubePostgres != "" {
		if ns, _, err := kube.ParseKubePostgres(cfg.KubePostgres); err == nil {
			namespace = ns
		}
	}
	if u, err := url.Parse(dbURL); err == nil && u.Path != "" {
		database = strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
	}
	return cluster, client, namespace, database
}

func buildSenders(cfg *config.Config) []notify.Sender {
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
	return senders
}

func notifyConnectFailure(ctx context.Context, senders []notify.Sender, cfg *config.Config, cluster, client, ns, db string, connectErr error) {
	if len(senders) == 0 {
		return
	}
	log.Printf("Sending notification…")
	// Connection failure is urgent: always notify when senders exist, even in dry-run (infrastructure failure must be visible).
	tooManyClients := connectErr != nil && (strings.Contains(connectErr.Error(), "too many clients") || strings.Contains(connectErr.Error(), "53300"))
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

func applyThresholdDefaults(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	maxConn, maxConnErr := postgres.MaxConnections(ctx, pool)
	if cfg.TestMaxConnections > 0 {
		maxConn = cfg.TestMaxConnections
	}
	if !cfg.UsesLevelMode() && maxConn > 0 {
		checker.ApplySingleThresholdDefaults(cfg, maxConn)
	}
	if err := checker.ValidateThresholdConfig(cfg, maxConn, maxConnErr); err != nil {
		return err
	}
	return nil
}

func collectEvents(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, stats postgres.ConnectionStats, maxConn int, cluster, client, ns, db string) []notify.Event {
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

func collectStaleEvent(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, ev notify.Event) *notify.Event {
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

func collectLongQueryEvent(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, ev notify.Event) *notify.Event {
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

// sendEvents delivers each event to all senders. Returns thresholds for which at least one send succeeded (dry-run: empty).
func sendEvents(ctx context.Context, senders []notify.Sender, cfg *config.Config, events []notify.Event) map[string]bool {
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
			log.Printf("Notification sent: %s", ev.Message)
		}
	}
	return sent
}

// runCheckResult is returned by doRunCheck for store integration.
type runCheckResult struct {
	Stats              postgres.ConnectionStats
	MaxConn            int
	Events             []notify.Event
	StaleCount         int // from target StaleAge (for alerts)
	StaleCountForStore int // from SqliteStaleAge if >0, else StaleCount (for store)
}

func doRunCheck(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, cluster, client, ns, db string) (runCheckResult, error) {
	var res runCheckResult
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
	// Stale count for store: use sqlite.stale_age if set, else same as alerts
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

func applyHysteresisFilter(ctx context.Context, st store.MetricsStorer, cfg *config.Config, client, cluster, db, state string, events []notify.Event) []notify.Event {
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

func applyLongQueryCooldownFilter(ctx context.Context, st store.MetricsStorer, cfg *config.Config, client, cluster, db string, events []notify.Event) []notify.Event {
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

func trySendResolutionNotification(ctx context.Context, st store.MetricsStorer, cfg *config.Config, senders []notify.Sender, res runCheckResult, cluster, client, ns, db string) {
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
	sendEvents(ctx, senders, cfg, []notify.Event{ev})
}

func makeRunFunc(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, senders []notify.Sender, st store.MetricsStorer, cluster, client, ns, db string) func() {
	return func() {
		res, err := doRunCheck(ctx, pool, cfg, cluster, client, ns, db)
		if err != nil {
			log.Printf("stats: %v", err)
			return
		}
		if cfg.DryRun && cfg.LogLevel == "debug" {
			logDryRunStats(cluster, client, db, res)
		}
		statePre, _ := checker.StateAndThresholdFromEvents(res.Events)
		res.Events = applyHysteresisFilter(ctx, st, cfg, client, cluster, db, statePre, res.Events)
		res.Events = applyLongQueryCooldownFilter(ctx, st, cfg, client, cluster, db, res.Events)
		state, thr := checker.StateAndThresholdFromEvents(res.Events)
		sent := sendEvents(ctx, senders, cfg, res.Events)
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
				trySendResolutionNotification(ctx, st, cfg, senders, res, cluster, client, ns, db)
			}
		}
	}
}

func logDryRunStats(cluster, client, database string, res runCheckResult) {
	// target = cluster (if k8s) / client (monitor id) / database (Postgres DB name)
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

func logStartupBanner(cfg *config.Config) {
	commit := Commit
	if commit == "" {
		commit = "unknown"
	}
	built := BuildDate
	if built == "" {
		built = "unknown"
	}
	log.Printf("pgwd: starting %s (commit %s, built %s) %s/%s log_level=%s",
		Version, commit, built, runtime.GOOS, runtime.GOARCH, cfg.LogLevel)
}

func logConfigTrace(path string, configLoaded bool, hasCLIArgs bool) {
	if path != "" {
		if configLoaded {
			log.Printf("pgwd: config file %s: loaded", path)
		} else {
			log.Printf("pgwd: config file %s: not found", path)
		}
	}
	envCount := 0
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PGWD_") {
			envCount++
		}
	}
	if configLoaded {
		if envCount > 0 {
			log.Printf("pgwd: PGWD_* env: %d var(s) set, ignored (config file is source)", envCount)
		} else {
			log.Printf("pgwd: PGWD_* env: not set")
		}
	} else {
		if envCount == 0 {
			log.Printf("pgwd: PGWD_* env: not set")
		} else {
			log.Printf("pgwd: PGWD_* env: %d var(s) set", envCount)
		}
	}
	if !hasCLIArgs {
		log.Printf("pgwd: CLI params: not used")
	} else {
		log.Printf("pgwd: CLI params: used")
	}
}

func main() {
	handleVersion()

	cfg, loaded, configPath := loadAndParseConfig()
	applyDBURLOverride(&cfg)
	if strings.TrimSpace(cfg.ExportMetricsFormat) != "" {
		exportMetricsAndExit(&cfg)
	}
	if cfg.ValidateK8sAccess {
		if err := kube.ValidateKubernetesAccess(context.Background(), cfg.KubeContext); err != nil {
			log.Fatalf("validate-k8s-access: %v", err)
		}
		os.Exit(0)
	}
	validateConfig(&cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !cfg.UsesDatabases() {
		defer setupKube(ctx, &cfg)()
		defer setupKubeLoki(ctx, &cfg)()
	}
	if cfg.KubePostgres == "" && cfg.KubeLoki == "" {
		openbsd.ApplyPledge()
	}

	senders := buildSenders(&cfg)
	targets := cfg.Targets()
	st := openStoreIfConfigured(&cfg)
	if st != nil {
		defer st.Close()
	}
	httpCleanup := setupHTTPIfConfigured(&cfg, st)
	defer httpCleanup()
	maybeLogConfigTrace(&cfg, configPath, loaded, len(os.Args) > 1)

	for _, t := range targets {
		runOneTarget(ctx, t, &cfg, senders, st, targets)
	}
	if cfg.Interval <= 0 {
		return
	}
	runTickerLoop(ctx, &cfg, senders, st, targets)
}

func loadAndParseConfig() (config.Config, bool, string) {
	path := config.ConfigPath()
	cfg, loaded, err := config.FromFile(path)
	if err != nil {
		log.Fatalf("pgwd: config file %s: %v", path, err)
	}
	if !loaded {
		config.ApplyDefaults(&cfg)
		config.ApplyEnv(&cfg)
	} else {
		config.ApplyDefaults(&cfg)
	}
	if parseFlags(&cfg) {
		printVersion()
		os.Exit(0)
	}
	config.FinalizeAfterFlags(&cfg)
	logStartupBanner(&cfg)
	return cfg, loaded, path
}

func maybeLogConfigTrace(cfg *config.Config, path string, loaded bool, hasCLIArgs bool) {
	if cfg.LogLevel == "debug" {
		logConfigTrace(path, loaded, hasCLIArgs)
	}
}

func applyDBURLOverride(cfg *config.Config) {
	if cfg.DBURL != "" && cfg.Interval <= 0 && cfg.UsesDatabases() {
		cfg.Databases = nil
	}
}

func openStoreIfConfigured(cfg *config.Config) store.MetricsStorer {
	switch metricsstore.Driver(cfg) {
	case metricsstore.DriverSQLite:
		if cfg.SqlitePath == "" {
			return nil
		}
		st, err := store.Open(cfg.SqlitePath, cfg.SqliteMaxMetrics)
		if err != nil {
			log.Fatalf("sqlite metrics store: %v", err)
		}
		return st
	case metricsstore.DriverPostgres, metricsstore.DriverMySQL:
		st, err := store.OpenSQLMetrics(metricsstore.Driver(cfg), cfg.MetricsStoreDSN, cfg.SqliteMaxMetrics)
		if err != nil {
			log.Fatalf("metrics SQL store: %v", err)
		}
		return st
	default:
		return nil
	}
}

func setupHTTPIfConfigured(cfg *config.Config, st store.MetricsStorer) func() {
	if cfg.HTTPListen == "" {
		return func() {}
	}
	httpSrv := httpsrv.New(cfg, st)
	if err := httpSrv.Start(); err != nil {
		log.Fatalf("http: %v", err)
	}
	log.Printf("pgwd: HTTP listening on %s (%s%s)", cfg.HTTPListen, cfg.HTTPBasePath, cfg.HTTPHealthPath)
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Stop(shutdownCtx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
	}
}

func runOneTarget(ctx context.Context, t config.DatabaseTarget, cfg *config.Config, senders []notify.Sender, st store.MetricsStorer, targets []config.DatabaseTarget) {
	targetCfg := cfg.ConfigForTarget(t)
	runCluster, runClient, runNamespace, runDatabase := runContextStrings(ctx, targetCfg, t.URL)

	pool, err := postgres.Pool(ctx, t.URL)
	if err != nil {
		notifyConnectFailure(ctx, senders, targetCfg, runCluster, runClient, runNamespace, runDatabase, err)
		if len(targets) == 1 {
			log.Fatal("postgres connect failed (check database URL, connectivity, and credentials)")
		}
		log.Printf("postgres connect failed [%s]: %v", t.Client, err)
		return
	}
	defer pool.Close()

	if err := applyThresholdDefaults(ctx, pool, targetCfg); err != nil {
		notifyConnectFailure(ctx, senders, targetCfg, runCluster, runClient, runNamespace, runDatabase, err)
		log.Printf("threshold config error [%s]: %v", t.Client, err)
		return
	}
	run := makeRunFunc(ctx, pool, targetCfg, senders, st, runCluster, runClient, runNamespace, runDatabase)
	run()
}

func runTickerLoop(ctx context.Context, cfg *config.Config, senders []notify.Sender, st store.MetricsStorer, targets []config.DatabaseTarget) {
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, t := range targets {
				runOneTarget(ctx, t, cfg, senders, st, targets)
			}
		}
	}
}
