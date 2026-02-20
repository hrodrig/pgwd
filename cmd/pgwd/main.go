package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hrodrig/pgwd/internal/config"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Pool(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("postgres connect: %v", err)
	}
	defer pool.Close()

	// Apply sensible defaults from server max_connections when thresholds are not set (0)
	percent := cfg.DefaultThresholdPercent
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	if maxConn, err := postgres.MaxConnections(ctx, pool); err == nil && maxConn > 0 {
		defaultThreshold := (maxConn * percent) / 100
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

	run := func() {
		stats, err := postgres.Stats(ctx, pool)
		if err != nil {
			log.Printf("stats: %v", err)
			return
		}

		if cfg.DryRun {
			maxConn, _ := postgres.MaxConnections(ctx, pool)
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
					Stats:          stats,
					Threshold:      "stale",
					ThresholdValue: cfg.ThresholdStale,
					Message:        fmt.Sprintf("Stale connections (open > %ds): %d >= %d", cfg.StaleAge, staleCount, cfg.ThresholdStale),
				})
			}
		}
		if cfg.ThresholdTotal > 0 && stats.Total >= cfg.ThresholdTotal {
			events = append(events, notify.Event{
				Stats:          stats,
				Threshold:      "total",
				ThresholdValue: cfg.ThresholdTotal,
				Message:        fmt.Sprintf("Total connections %d >= %d", stats.Total, cfg.ThresholdTotal),
			})
		}
		if cfg.ThresholdActive > 0 && stats.Active >= cfg.ThresholdActive {
			events = append(events, notify.Event{
				Stats:          stats,
				Threshold:      "active",
				ThresholdValue: cfg.ThresholdActive,
				Message:        fmt.Sprintf("Active connections %d >= %d", stats.Active, cfg.ThresholdActive),
			})
		}
		if cfg.ThresholdIdle > 0 && stats.Idle >= cfg.ThresholdIdle {
			events = append(events, notify.Event{
				Stats:          stats,
				Threshold:      "idle",
				ThresholdValue: cfg.ThresholdIdle,
				Message:        fmt.Sprintf("Idle connections %d >= %d", stats.Idle, cfg.ThresholdIdle),
			})
		}
		if cfg.ForceNotification {
			events = append(events, notify.Event{
				Stats:          stats,
				Threshold:      "test",
				ThresholdValue: 0,
				Message:        fmt.Sprintf("Test notification — delivery check (force-notification). Current: total=%d active=%d idle=%d", stats.Total, stats.Active, stats.Idle),
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
