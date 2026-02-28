package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/kube"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/postgres"
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
	flag.StringVar(&cfg.DBURL, "db-url", cfg.DBURL, "PostgreSQL connection URL (PGWD_DB_URL)")
	flag.IntVar(&cfg.ThresholdTotal, "threshold-total", cfg.ThresholdTotal, "Alert when total connections >= N (PGWD_THRESHOLD_TOTAL)")
	flag.IntVar(&cfg.ThresholdActive, "threshold-active", cfg.ThresholdActive, "Alert when active connections >= N (PGWD_THRESHOLD_ACTIVE)")
	flag.IntVar(&cfg.ThresholdIdle, "threshold-idle", cfg.ThresholdIdle, "Alert when idle connections >= N (PGWD_THRESHOLD_IDLE)")
	flag.IntVar(&cfg.StaleAge, "stale-age", cfg.StaleAge, "Consider connection stale if open longer than N seconds (PGWD_STALE_AGE)")
	flag.IntVar(&cfg.ThresholdStale, "threshold-stale", cfg.ThresholdStale, "Alert when stale connections (open > stale-age) >= N (PGWD_THRESHOLD_STALE)")
	flag.StringVar(&cfg.SlackWebhook, "slack-webhook", cfg.SlackWebhook, "Slack Incoming Webhook URL (PGWD_SLACK_WEBHOOK)")
	flag.StringVar(&cfg.LokiURL, "loki-url", cfg.LokiURL, "Loki push API URL, e.g. http://localhost:3100/loki/api/v1/push (PGWD_LOKI_URL)")
	flag.StringVar(&cfg.LokiLabels, "loki-labels", cfg.LokiLabels, "Loki labels, e.g. job=pgwd,env=prod (PGWD_LOKI_LABELS)")
	flag.IntVar(&cfg.Interval, "interval", cfg.Interval, "Run every N seconds; 0 = run once (PGWD_INTERVAL)")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Only print, do not send notifications (PGWD_DRY_RUN)")
	flag.BoolVar(&cfg.ForceNotification, "force-notification", cfg.ForceNotification, "Always send a test notification to validate delivery/format (PGWD_FORCE_NOTIFICATION)")
	flag.IntVar(&cfg.DefaultThresholdPercent, "default-threshold-percent", cfg.DefaultThresholdPercent, "When total/active threshold are 0, set to this % of max_connections (1-100, default 80) (PGWD_DEFAULT_THRESHOLD_PERCENT)")
	flag.StringVar(&cfg.KubePostgres, "kube-postgres", cfg.KubePostgres, "Connect via kubectl port-forward: namespace/type/name (e.g. default/svc/postgres) (PGWD_KUBE_POSTGRES)")
	flag.IntVar(&cfg.KubeLocalPort, "kube-local-port", cfg.KubeLocalPort, "Local port for kube port-forward (default 5432) (PGWD_KUBE_LOCAL_PORT)")
	flag.StringVar(&cfg.KubePasswordVar, "kube-password-var", cfg.KubePasswordVar, "Pod env var for password when URL has DISCOVER_MY_PASSWORD (default POSTGRES_PASSWORD) (PGWD_KUBE_PASSWORD_VAR)")
	flag.StringVar(&cfg.KubePasswordContainer, "kube-password-container", cfg.KubePasswordContainer, "Container name in pod for password discovery (PGWD_KUBE_PASSWORD_CONTAINER)")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Cluster name for notifications (PGWD_CLUSTER); when -kube-postgres is set, detected from kubeconfig if unset")
	flag.StringVar(&cfg.Client, "client", cfg.Client, "Client/service/pod name for notifications (PGWD_CLIENT); when -kube-postgres is set, derived from resource (e.g. svc/name) if unset")
	flag.BoolVar(&cfg.NotifyOnConnectFailure, "notify-on-connect-failure", cfg.NotifyOnConnectFailure, "Send an alert to notifiers when Postgres connection fails (infrastructure alert) (PGWD_NOTIFY_ON_CONNECT_FAILURE)")
	flag.IntVar(&cfg.TestMaxConnections, "test-max-connections", cfg.TestMaxConnections, "Override server max_connections for defaults and display (for testing alerts; 0 = use server) (PGWD_TEST_MAX_CONNECTIONS)")
	flag.Parse()
	return *showVersionFlag
}

func validateConfig(cfg *config.Config) {
	if cfg.DBURL == "" {
		log.Fatal("missing database URL: set PGWD_DB_URL or -db-url")
	}
	if cfg.ThresholdStale > 0 && cfg.StaleAge <= 0 {
		log.Fatal("when using threshold-stale, stale-age must be > 0 (PGWD_STALE_AGE or -stale-age)")
	}
	if !cfg.HasAnyNotifier() && !cfg.DryRun {
		log.Fatal("no notifier configured: set PGWD_SLACK_WEBHOOK and/or PGWD_LOKI_URL (or -slack-webhook / -loki-url), or use -dry-run")
	}
	if cfg.ForceNotification && !cfg.HasAnyNotifier() {
		log.Fatal("force-notification requires at least one notifier (slack-webhook or loki-url)")
	}
	if cfg.NotifyOnConnectFailure && !cfg.HasAnyNotifier() {
		log.Fatal("notify-on-connect-failure requires at least one notifier (slack-webhook or loki-url)")
	}
	if cfg.KubePostgres != "" && cfg.DBURL == "" {
		log.Fatal("kube-postgres requires PGWD_DB_URL or -db-url (use host localhost and the same port as -kube-local-port)")
	}
}

func setupKube(ctx context.Context, cfg *config.Config) {
	if cfg.KubePostgres == "" {
		return
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
		podName, err := kube.ResolvePod(ctx, namespace, resource)
		if err != nil {
			log.Fatalf("kube resolve pod: %v", err)
		}
		password, err = kube.GetPasswordFromPod(ctx, namespace, podName, cfg.KubePasswordContainer, cfg.KubePasswordVar)
		if err != nil {
			log.Fatal("kube: could not get password from pod (check namespace, pod name, container, and env var)")
		}
	}
	finalURL, err := kube.ReplaceDBURLForKube(cfg.DBURL, password, cfg.KubeLocalPort)
	if err != nil {
		log.Fatal("kube: failed to build DB URL (check -db-url format)")
	}
	cfg.DBURL = finalURL
	cleanup, err := kube.StartPortForward(ctx, namespace, resource, cfg.KubeLocalPort)
	if err != nil {
		log.Fatalf("kube port-forward: %v", err)
	}
	defer cleanup()
}

func runContextStrings(ctx context.Context, cfg *config.Config) (cluster, client, namespace, database string) {
	if cfg.Cluster != "" {
		cluster = cfg.Cluster
	} else if cfg.KubePostgres != "" {
		cluster = kube.ClusterName(ctx)
	}
	if cfg.Client != "" {
		client = cfg.Client
	} else if cfg.KubePostgres != "" {
		if _, res, err := kube.ParseKubePostgres(cfg.KubePostgres); err == nil {
			client = res
		}
	}
	if client == "" {
		if h, err := os.Hostname(); err == nil {
			client = h
		}
	}
	if cfg.KubePostgres != "" {
		if ns, _, err := kube.ParseKubePostgres(cfg.KubePostgres); err == nil {
			namespace = ns
		}
	}
	if u, err := url.Parse(cfg.DBURL); err == nil && u.Path != "" {
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
			URL:    cfg.LokiURL,
			Labels: notify.ParseLokiLabels(cfg.LokiLabels),
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
	for _, s := range senders {
		if sendErr := s.Send(ctx, ev); sendErr != nil {
			log.Printf("notify (connect failure): %v", sendErr)
		}
	}
}

func applyThresholdDefaults(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	percent := cfg.DefaultThresholdPercent
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	maxConnForDefaults, maxConnErr := postgres.MaxConnections(ctx, pool)
	if cfg.TestMaxConnections > 0 {
		maxConnForDefaults = cfg.TestMaxConnections
	}
	if maxConnForDefaults > 0 {
		defaultThreshold := (maxConnForDefaults * percent) / 100
		if defaultThreshold < 1 {
			defaultThreshold = 1
		}
		if cfg.ThresholdTotal == 0 {
			cfg.ThresholdTotal = defaultThreshold
		}
		if cfg.ThresholdActive == 0 {
			cfg.ThresholdActive = defaultThreshold
		}
	}
	if !cfg.HasAnyThreshold() && !cfg.DryRun && !cfg.ForceNotification {
		if maxConnErr != nil {
			return fmt.Errorf("no thresholds set and could not default from server (total/active default to default-threshold-percent of max_connections). Set -threshold-total and/or -threshold-active, or use -dry-run or -force-notification: %w", maxConnErr)
		}
		if maxConnForDefaults == 0 {
			return fmt.Errorf("no thresholds set and could not default from server (server returned max_connections=0). Set -threshold-total and/or -threshold-active, or use -dry-run or -force-notification")
		}
		return fmt.Errorf("no thresholds set. Set -threshold-total and/or -threshold-active, or use -dry-run or -force-notification")
	}
	return nil
}

func baseEvent(stats postgres.ConnectionStats, maxConn int, override bool, cluster, client, ns, db string) notify.Event {
	return notify.Event{
		Stats:                    stats,
		MaxConnections:           maxConn,
		MaxConnectionsIsOverride: override,
		Cluster:                  cluster,
		Client:                   client,
		Namespace:                ns,
		Database:                 db,
	}
}

func collectEvents(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, stats postgres.ConnectionStats, maxConn int, cluster, client, ns, db string) []notify.Event {
	var events []notify.Event
	override := cfg.TestMaxConnections > 0
	ev := baseEvent(stats, maxConn, override, cluster, client, ns, db)

	if cfg.ThresholdStale > 0 && cfg.StaleAge > 0 {
		staleCount, err := postgres.StaleCount(ctx, pool, cfg.StaleAge)
		if err != nil {
			log.Printf("stale count: %v", err)
		} else if staleCount >= cfg.ThresholdStale {
			e := ev
			e.Threshold = "stale"
			e.ThresholdValue = cfg.ThresholdStale
			e.Message = fmt.Sprintf("Stale connections (open > %ds): %d >= %d", cfg.StaleAge, staleCount, cfg.ThresholdStale)
			events = append(events, e)
		}
	}
	if cfg.ThresholdTotal > 0 && stats.Total >= cfg.ThresholdTotal {
		e := ev
		e.Threshold = "total"
		e.ThresholdValue = cfg.ThresholdTotal
		e.Message = fmt.Sprintf("Total connections %d >= %d", stats.Total, cfg.ThresholdTotal)
		events = append(events, e)
	}
	if cfg.ThresholdActive > 0 && stats.Active >= cfg.ThresholdActive {
		e := ev
		e.Threshold = "active"
		e.ThresholdValue = cfg.ThresholdActive
		e.Message = fmt.Sprintf("Active connections %d >= %d", stats.Active, cfg.ThresholdActive)
		events = append(events, e)
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

func sendEvents(ctx context.Context, senders []notify.Sender, cfg *config.Config, events []notify.Event) {
	for _, ev := range events {
		if cfg.DryRun {
			log.Printf("[dry-run] would send: %s", ev.Message)
			continue
		}
		for _, s := range senders {
			if err := s.Send(ctx, ev); err != nil {
				log.Printf("notify: %v", err)
			}
		}
	}
}

func makeRunFunc(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, senders []notify.Sender, cluster, client, ns, db string) func() {
	return func() {
		stats, err := postgres.Stats(ctx, pool)
		if err != nil {
			log.Printf("stats: %v", err)
			return
		}
		maxConn, _ := postgres.MaxConnections(ctx, pool)
		if cfg.TestMaxConnections > 0 {
			maxConn = cfg.TestMaxConnections
		}
		if cfg.DryRun {
			if maxConn > 0 {
				log.Printf("total=%d active=%d idle=%d max_connections=%d", stats.Total, stats.Active, stats.Idle, maxConn)
			} else {
				log.Printf("total=%d active=%d idle=%d", stats.Total, stats.Active, stats.Idle)
			}
		}
		events := collectEvents(ctx, pool, cfg, stats, maxConn, cluster, client, ns, db)
		sendEvents(ctx, senders, cfg, events)
	}
}

func main() {
	handleVersion()

	cfg := config.FromEnv()
	if parseFlags(&cfg) {
		printVersion()
		os.Exit(0)
	}
	validateConfig(&cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	setupKube(ctx, &cfg)
	runCluster, runClient, runNamespace, runDatabase := runContextStrings(ctx, &cfg)
	senders := buildSenders(&cfg)

	pool, err := postgres.Pool(ctx, cfg.DBURL)
	if err != nil {
		notifyConnectFailure(ctx, senders, &cfg, runCluster, runClient, runNamespace, runDatabase, err)
		log.Fatal("postgres connect failed (check database URL, connectivity, and credentials)")
	}
	defer pool.Close()

	if err := applyThresholdDefaults(ctx, pool, &cfg); err != nil {
		notifyConnectFailure(ctx, senders, &cfg, runCluster, runClient, runNamespace, runDatabase, err)
		log.Fatal(err)
	}
	run := makeRunFunc(ctx, pool, &cfg, senders, runCluster, runClient, runNamespace, runDatabase)
	run()
	if cfg.Interval <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
