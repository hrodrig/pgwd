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

func main() {
	// "pgwd version" or "pgwd -version" / "--version": print version and exit
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "-version" || os.Args[1] == "--version") {
		printVersion()
		os.Exit(0)
	}

	cfg := config.FromEnv()

	// CLI overrides (same names as env, without prefix in flags for brevity)
	showVersion := flag.Bool("version", false, "print version and exit")
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

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Optional: Kubernetes port-forward and password discovery
	if cfg.KubePostgres != "" {
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

	// Run context for notifications (health-check style: cluster, client, namespace)
	var runCluster, runClient, runNamespace string
	if cfg.Cluster != "" {
		runCluster = cfg.Cluster
	} else if cfg.KubePostgres != "" {
		runCluster = kube.ClusterName(ctx)
	}
	if cfg.Client != "" {
		runClient = cfg.Client
	} else if cfg.KubePostgres != "" {
		if _, res, err := kube.ParseKubePostgres(cfg.KubePostgres); err == nil {
			runClient = res
		}
	}
	if runClient == "" {
		if h, err := os.Hostname(); err == nil {
			runClient = h
		}
	}
	if cfg.KubePostgres != "" {
		if ns, _, err := kube.ParseKubePostgres(cfg.KubePostgres); err == nil {
			runNamespace = ns
		}
	}
	var runDatabase string
	if u, err := url.Parse(cfg.DBURL); err == nil && u.Path != "" {
		runDatabase = strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
	}

	// Build senders early so we can notify on connection failure if requested
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

	pool, err := postgres.Pool(ctx, cfg.DBURL)
	if err != nil {
		// Notify on failure when requested (infrastructure alert) or when force-notification (validate channel/format)
		if len(senders) > 0 && !cfg.DryRun && (cfg.NotifyOnConnectFailure || cfg.ForceNotification) {
			ev := notify.Event{
				Stats:          postgres.ConnectionStats{},
				Threshold:      "connect_failure",
				ThresholdValue: 0,
				Message:        "pgwd could not connect to Postgres. Check database URL, connectivity, credentials, or infrastructure.",
				Cluster:        runCluster,
				Client:         runClient,
				Namespace:      runNamespace,
				Database:       runDatabase,
			}
			for _, s := range senders {
				if sendErr := s.Send(ctx, ev); sendErr != nil {
					log.Printf("notify (connect failure): %v", sendErr)
				}
			}
		}
		log.Fatal("postgres connect failed (check database URL, connectivity, and credentials)")
	}
	defer pool.Close()

	// Apply sensible defaults from server max_connections (or override) when thresholds are not set (0)
	percent := cfg.DefaultThresholdPercent
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	maxConnForDefaults, _ := postgres.MaxConnections(ctx, pool)
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
		log.Fatal("at least one of: threshold (PGWD_THRESHOLD_* / -threshold-*), -dry-run, or -force-notification required (total/active default to default-threshold-percent of max_connections when unset)")
	}

	run := func() {
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

		var events []notify.Event
		if cfg.ThresholdStale > 0 && cfg.StaleAge > 0 {
			staleCount, err := postgres.StaleCount(ctx, pool, cfg.StaleAge)
			if err != nil {
				log.Printf("stale count: %v", err)
			} else if staleCount >= cfg.ThresholdStale {
				events = append(events, notify.Event{
					Stats:                    stats,
					Threshold:                "stale",
					ThresholdValue:           cfg.ThresholdStale,
					Message:                  fmt.Sprintf("Stale connections (open > %ds): %d >= %d", cfg.StaleAge, staleCount, cfg.ThresholdStale),
					MaxConnections:           maxConn,
					MaxConnectionsIsOverride: cfg.TestMaxConnections > 0,
					Cluster:                  runCluster,
					Client:                   runClient,
					Namespace:                runNamespace,
					Database:                 runDatabase,
				})
			}
		}
		if cfg.ThresholdTotal > 0 && stats.Total >= cfg.ThresholdTotal {
			events = append(events, notify.Event{
				Stats:                    stats,
				Threshold:                "total",
				ThresholdValue:           cfg.ThresholdTotal,
				Message:                  fmt.Sprintf("Total connections %d >= %d", stats.Total, cfg.ThresholdTotal),
				MaxConnections:           maxConn,
				MaxConnectionsIsOverride: cfg.TestMaxConnections > 0,
				Cluster:                  runCluster,
				Client:                   runClient,
				Namespace:                runNamespace,
				Database:                 runDatabase,
			})
		}
		if cfg.ThresholdActive > 0 && stats.Active >= cfg.ThresholdActive {
			events = append(events, notify.Event{
				Stats:                    stats,
				Threshold:                "active",
				ThresholdValue:           cfg.ThresholdActive,
				Message:                  fmt.Sprintf("Active connections %d >= %d", stats.Active, cfg.ThresholdActive),
				MaxConnections:           maxConn,
				MaxConnectionsIsOverride: cfg.TestMaxConnections > 0,
				Cluster:                  runCluster,
				Client:                   runClient,
				Namespace:                runNamespace,
				Database:                 runDatabase,
			})
		}
		if cfg.ThresholdIdle > 0 && stats.Idle >= cfg.ThresholdIdle {
			events = append(events, notify.Event{
				Stats:                    stats,
				Threshold:                "idle",
				ThresholdValue:           cfg.ThresholdIdle,
				Message:                  fmt.Sprintf("Idle connections %d >= %d", stats.Idle, cfg.ThresholdIdle),
				MaxConnections:           maxConn,
				MaxConnectionsIsOverride: cfg.TestMaxConnections > 0,
				Cluster:                  runCluster,
				Client:                   runClient,
				Namespace:                runNamespace,
				Database:                 runDatabase,
			})
		}
		if cfg.ForceNotification {
			events = append(events, notify.Event{
				Stats:                    stats,
				Threshold:                "test",
				ThresholdValue:           0,
				Message:                  "Test notification — delivery check (force-notification).",
				MaxConnections:           maxConn,
				MaxConnectionsIsOverride: cfg.TestMaxConnections > 0,
				Cluster:                  runCluster,
				Client:                   runClient,
				Namespace:                runNamespace,
				Database:                 runDatabase,
			})
		}

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
