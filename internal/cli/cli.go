// Package cli implements the pgwd command-line entry flow (config, kube, monitor loop).
package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hrodrig/pgwd/contrib"
	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/httpsrv"
	"github.com/hrodrig/pgwd/internal/kube"
	"github.com/hrodrig/pgwd/internal/metricsexport"
	"github.com/hrodrig/pgwd/internal/metricsstore"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/openbsd"
	"github.com/hrodrig/pgwd/internal/postgres"
	"github.com/hrodrig/pgwd/internal/run"
	"github.com/hrodrig/pgwd/internal/store"
	"github.com/hrodrig/pgwd/internal/validator"
)

// Process exit codes (see SPECIFICATIONS.md § Exit codes).
const (
	ExitConnectFailure = 2 // Postgres connection failure (or too many clients)
	ExitQueryError     = 3 // Postgres query error during stats collection
	ExitStrictNotify   = 4 // -strict and notifier delivery failed
)

// exitFunc is os.Exit; tests may replace it to avoid killing the test process.
var exitFunc = os.Exit

func init() {
	run.SetKubeHelpers(kube.ClusterName, kube.ParseKubePostgres)
}

// Build metadata injected at link time from cmd/pgwd via SetBuildInfo.
var (
	version   = "dev"
	commit    = ""
	buildDate = ""
	branch    = ""
)

// SetBuildInfo sets release identity strings (called from cmd/pgwd before Run).
func SetBuildInfo(v, c, built, b string) {
	version, commit, buildDate, branch = v, c, built, b
}

// printVersion writes the release identity line to stdout.
func printVersion() {
	c := commit
	if c == "" {
		c = "unknown"
	}
	built := buildDate
	if built == "" {
		built = "unknown"
	}
	br := branch
	if br == "" {
		br = "unknown"
	}
	fmt.Printf("pgwd %s (branch %s, commit %s, built %s)\n", version, br, c, built)
}

// handleVersion checks os.Args for "version"/"-version"/"--version"; prints version and exits if matched.
func handleVersion() {
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "-version" || os.Args[1] == "--version") {
		printVersion()
		os.Exit(0)
	}
}

// handlePrintSampleConfig writes the annotated example config to stdout and exits when requested.
func handlePrintSampleConfig() {
	for _, arg := range os.Args[1:] {
		if arg == "--print-sample-config" || arg == "-print-sample-config" {
			fmt.Print(contrib.SampleConf())
			os.Exit(0)
		}
	}
}

// parseFlags registers pgwd CLI flags on cfg, parses os.Args, and returns whether
// -version was set. Call after loading config from file/env so flag values override
// those sources (see config.FinalizeAfterFlags). When true, the caller should
// printVersion and exit without starting the monitor.
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
	flag.BoolVar(&cfg.PagerDutyEnabled, "notifications-pagerduty-enabled", cfg.PagerDutyEnabled, "Enable PagerDuty Events v2 notifications (PGWD_NOTIFICATIONS_PAGERDUTY_ENABLED)")
	flag.StringVar(&cfg.PagerDutyRoutingKey, "notifications-pagerduty-routing-key", cfg.PagerDutyRoutingKey, "PagerDuty Events v2 routing key (PGWD_NOTIFICATIONS_PAGERDUTY_ROUTING_KEY)")
	flag.StringVar(&cfg.PagerDutySeverity, "notifications-pagerduty-severity", cfg.PagerDutySeverity, "PagerDuty default severity (PGWD_NOTIFICATIONS_PAGERDUTY_SEVERITY; default warning)")
	flag.StringVar(&cfg.PagerDutySource, "notifications-pagerduty-source", cfg.PagerDutySource, "PagerDuty event source (PGWD_NOTIFICATIONS_PAGERDUTY_SOURCE; default pgwd)")
	flag.BoolVar(&cfg.TeamsEnabled, "notifications-teams-enabled", cfg.TeamsEnabled, "Enable Microsoft Teams notifications (PGWD_NOTIFICATIONS_TEAMS_ENABLED)")
	flag.StringVar(&cfg.TeamsWebhook, "notifications-teams-webhook", cfg.TeamsWebhook, "Microsoft Teams incoming webhook URL (PGWD_NOTIFICATIONS_TEAMS_WEBHOOK)")
	flag.BoolVar(&cfg.GenericEnabled, "notifications-generic-enabled", cfg.GenericEnabled, "Enable generic webhook notifications (PGWD_NOTIFICATIONS_GENERIC_ENABLED)")
	flag.StringVar(&cfg.GenericWebhookURL, "notifications-generic-webhook-url", cfg.GenericWebhookURL, "Generic webhook target URL (PGWD_NOTIFICATIONS_GENERIC_WEBHOOK_URL)")
	flag.StringVar(&cfg.GenericJSONKey, "notifications-generic-json-key", cfg.GenericJSONKey, "Generic webhook JSON field for message text (PGWD_NOTIFICATIONS_GENERIC_JSON_KEY; default text)")
	flag.StringVar(&cfg.GenericHeadersJSON, "notifications-generic-headers", cfg.GenericHeadersJSON, "Generic webhook HTTP headers as JSON object (PGWD_NOTIFICATIONS_GENERIC_HEADERS)")
	flag.StringVar(&cfg.GenericExtraFieldsJSON, "notifications-generic-extra-fields", cfg.GenericExtraFieldsJSON, "Generic webhook extra JSON fields as JSON object (PGWD_NOTIFICATIONS_GENERIC_EXTRA_FIELDS)")
	flag.StringVar(&cfg.GenericBodyTemplate, "notifications-generic-body-template", cfg.GenericBodyTemplate, "Generic webhook Go template for custom JSON body (PGWD_NOTIFICATIONS_GENERIC_BODY_TEMPLATE)")
	flag.StringVar(&cfg.GenericHMACSecret, "notifications-generic-hmac-secret", cfg.GenericHMACSecret, "Generic webhook HMAC-SHA256 signing secret (PGWD_NOTIFICATIONS_GENERIC_HMAC_SECRET)")
	flag.StringVar(&cfg.GenericHMACHeader, "notifications-generic-hmac-header", cfg.GenericHMACHeader, "Generic webhook HMAC signature header (PGWD_NOTIFICATIONS_GENERIC_HMAC_HEADER; default X-Pgwd-Signature)")
	flag.IntVar(&cfg.RetryMaxAttempts, "notifications-retry-max-attempts", cfg.RetryMaxAttempts, "Notifier HTTP retry max attempts (PGWD_NOTIFICATIONS_RETRY_MAX_ATTEMPTS; default 3)")
	retryInitialBackoff := flag.String("notifications-retry-initial-backoff", "", "Notifier HTTP retry initial backoff, e.g. 1s (PGWD_NOTIFICATIONS_RETRY_INITIAL_BACKOFF; default 1s)")
	retryMaxBackoff := flag.String("notifications-retry-max-backoff", "", "Notifier HTTP retry max backoff, e.g. 10s (PGWD_NOTIFICATIONS_RETRY_MAX_BACKOFF; default 10s)")
	flag.IntVar(&cfg.Interval, "interval", cfg.Interval, "Run every N seconds; 0 = run once (PGWD_INTERVAL)")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Only print, do not send notifications (PGWD_DRY_RUN)")
	flag.BoolVar(&cfg.Strict, "strict", cfg.Strict, "Exit 4 when notifier delivery fails for a threshold event (PGWD_STRICT)")
	flag.BoolVar(&cfg.EnableCollector, "enable-collector", cfg.EnableCollector, "Send anonymous usage telemetry on daemon startup (PGWD_ENABLE_COLLECTOR; default false)")
	flag.BoolVar(&cfg.EnableUpdateCheck, "enable-update-check", cfg.EnableUpdateCheck, "Check GitHub for newer pgwd releases on daemon startup (PGWD_ENABLE_UPDATE_CHECK; default true)")
	flag.BoolVar(&cfg.ForceNotification, "force-notification", cfg.ForceNotification, "Always send a test notification to validate delivery/format (PGWD_FORCE_NOTIFICATION)")
	flag.IntVar(&cfg.DefaultThresholdPercent, "db-default-threshold-percent", cfg.DefaultThresholdPercent, "When one of total/active is 0, set it to this % of max_connections (1-100, default 80) (PGWD_DB_DEFAULT_THRESHOLD_PERCENT)")
	flag.StringVar(&cfg.ThresholdLevels, "db-threshold-levels", cfg.ThresholdLevels, "When both total and active are 0: comma-separated percentages for 3-tier alerts, e.g. 75,85,95 (attention/alert/danger). Only highest level fires. (PGWD_DB_THRESHOLD_LEVELS)")
	flag.StringVar(&cfg.KubePostgres, "kube-postgres", cfg.KubePostgres, "Connect via port-forward (client-go): namespace/type/name (e.g. default/svc/postgres) (PGWD_KUBE_POSTGRES)")
	flag.StringVar(&cfg.KubeLoki, "kube-loki", cfg.KubeLoki, "Connect to Loki via port-forward when Loki is inside the cluster: namespace/type/name (e.g. monitoring/svc/loki) (PGWD_KUBE_LOKI)")
	flag.StringVar(&cfg.KubeContext, "kube-context", cfg.KubeContext, "Kubectl context to use (empty = current context) (PGWD_KUBE_CONTEXT)")
	flag.IntVar(&cfg.KubeLocalPort, "kube-local-port", cfg.KubeLocalPort, "Local port for kube port-forward (default 5432) (PGWD_KUBE_LOCAL_PORT)")
	flag.IntVar(&cfg.KubeLokiLocalPort, "kube-loki-local-port", cfg.KubeLokiLocalPort, "Local port for Loki port-forward (default 3100) (PGWD_KUBE_LOKI_LOCAL_PORT)")
	flag.IntVar(&cfg.KubeLokiRemotePort, "kube-loki-remote-port", cfg.KubeLokiRemotePort, "Remote port on the Loki service (default 3100) (PGWD_KUBE_LOKI_REMOTE_PORT)")
	flag.StringVar(&cfg.Client, "client", cfg.Client, "Client name for this monitor instance — REQUIRED (PGWD_CLIENT); identifies which monitor sent the alert")
	flag.BoolVar(&cfg.NotifyOnConnectFailure, "notify-on-connect-failure", cfg.NotifyOnConnectFailure, "Send an alert to notifiers when Postgres connection fails (infrastructure alert) (PGWD_NOTIFY_ON_CONNECT_FAILURE)")
	flag.IntVar(&cfg.TestMaxConnections, "test-max-connections", cfg.TestMaxConnections, "Override server max_connections for defaults and display (for testing alerts; 0 = use server) (PGWD_TEST_MAX_CONNECTIONS)")
	flag.BoolVar(&cfg.ValidateK8sAccess, "validate-k8s-access", cfg.ValidateK8sAccess, "Validate cluster connectivity and list pods, then exit. Use -kube-context to select context. (PGWD_VALIDATE_K8S_ACCESS)")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: info (default) or debug. Debug = verbose dry-run stats every interval (PGWD_LOG_LEVEL)")
	flag.StringVar(&cfg.ExportMetricsFormat, "export-metrics-format", cfg.ExportMetricsFormat, "Export persisted metrics from the configured metrics store; format: csv (file). Then exit (PGWD_EXPORT_METRICS_FORMAT)")
	flag.StringVar(&cfg.ExportMetricsDestination, "export-metrics-destination", cfg.ExportMetricsDestination, "Output path for -export-metrics-format csv (PGWD_EXPORT_METRICS_DESTINATION)")
	flag.Parse()
	if *retryInitialBackoff != "" {
		if d, err := time.ParseDuration(*retryInitialBackoff); err == nil {
			cfg.RetryInitialBackoff = d
		}
	}
	if *retryMaxBackoff != "" {
		if d, err := time.ParseDuration(*retryMaxBackoff); err == nil {
			cfg.RetryMaxBackoff = d
		}
	}
	return *showVersionFlag
}

// validateConfig runs validator.Validate on the merged config (file, env, flags).
// On failure it logs the error and exits; on success it returns so main can start
// kube port-forwards and the monitor loop.
func validateConfig(cfg *config.Config) {
	if err := validator.Validate(cfg); err != nil {
		log.Fatal(err)
	}
}

// exportMetricsAndExit runs a one-shot export of persisted check history from the
// configured metrics store (sqlite.path or metrics_store) to dest using format
// (e.g. csv). Requires both -export-metrics-format and -export-metrics-destination;
// logs the row count and exits 0 on success, or log.Fatal on error.
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

// setupKube configures Kubernetes access when -kube-postgres is set. If KubePostgres
// is empty, it returns a no-op cleanup and leaves cfg unchanged. Otherwise it
// optionally loads the DB password from a Secret (kube.password_from_secret),
// rejects the removed DISCOVER_MY_PASSWORD placeholder, rewrites cfg.DBURL to
// localhost, and starts client-go port-forward. Returns a cleanup that stops the
// forward; defer it from main.
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
	secret := kube.PasswordFromSecret{
		Namespace: cfg.KubePasswordFromSecret.Namespace,
		Name:      cfg.KubePasswordFromSecret.Name,
		Key:       cfg.KubePasswordFromSecret.Key,
	}
	if secret.Namespace == "" && secret.Name != "" {
		secret.Namespace = namespace
	}
	finalURL, err := kube.ResolveKubeDBURL(ctx, cfg.KubeContext, cfg.DBURL, secret, cfg.KubeLocalPort)
	if err != nil {
		log.Fatalf("kube-postgres: %v", err)
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

// LogStartupBanner logs version, build metadata, platform, and log level at daemon start.
func LogStartupBanner(cfg *config.Config) {
	logStartupBanner(cfg)
}

func logStartupBanner(cfg *config.Config) {
	c := commit
	if c == "" {
		c = "unknown"
	}
	built := buildDate
	if built == "" {
		built = "unknown"
	}
	br := branch
	if br == "" {
		br = "unknown"
	}
	log.Printf("pgwd: starting %s (branch %s, commit %s, built %s) %s/%s log_level=%s",
		version, br, c, built, runtime.GOOS, runtime.GOARCH, cfg.LogLevel)
}

// LogConfigTrace logs config file load status and env/CLI usage (debug).
func LogConfigTrace(path string, configLoaded bool, hasCLIArgs bool) {
	logConfigTrace(path, configLoaded, hasCLIArgs)
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

// Run is the pgwd entry point: load config, optional kube port-forwards, run checks.
func Run() {
	handleVersion()
	handlePrintSampleConfig()

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

	if cfg.Interval > 0 {
		startCollector(&cfg)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !cfg.UsesDatabases() {
		defer setupKube(ctx, &cfg)()
		defer setupKubeLoki(ctx, &cfg)()
	}
	if cfg.KubePostgres == "" && cfg.KubeLoki == "" {
		openbsd.ApplyPledge()
	}

	senders := run.BuildSenders(&cfg)
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

// loadAndParseConfig reads YAML (if present), applies defaults and env, parses CLI
// flags, and handles -version early exit. Returns cfg, whether the file loaded,
// and the config path used.
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

// MaybeLogConfigTrace emits LogConfigTrace when log-level is debug.
func MaybeLogConfigTrace(cfg *config.Config, path string, loaded bool, hasCLIArgs bool) {
	maybeLogConfigTrace(cfg, path, loaded, hasCLIArgs)
}

func maybeLogConfigTrace(cfg *config.Config, path string, loaded bool, hasCLIArgs bool) {
	if cfg.LogLevel == "debug" {
		logConfigTrace(path, loaded, hasCLIArgs)
	}
}

// ApplyDBURLOverride clears databases when -db-url is set for a one-shot run.
func ApplyDBURLOverride(cfg *config.Config) {
	applyDBURLOverride(cfg)
}

func applyDBURLOverride(cfg *config.Config) {
	if cfg.DBURL != "" && cfg.Interval <= 0 && cfg.UsesDatabases() {
		cfg.Databases = nil
	}
}

// openStoreIfConfigured opens the metrics store (SQLite, PostgreSQL, or MySQL) when
// configured in cfg. Returns nil when no store driver/path/dsn is set; log.Fatal on
// open errors.
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

// setupHTTPIfConfigured starts the HTTP server (health + /metrics) when http.listen
// is set. Returns a no-op cleanup when disabled, or a shutdown function to defer.
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

// exitStrictIf exits with ExitStrictNotify when -strict is set and delivery failed.
func exitStrictIf(cfg *config.Config, deliveryFailed bool) {
	if cfg.Strict && deliveryFailed {
		log.Printf("pgwd: strict mode: notifier delivery failed")
		exitFunc(ExitStrictNotify)
	}
}

// exitConnectFailureIf exits with ExitConnectFailure for single-target connect failure.
func exitConnectFailureIf(targets []config.DatabaseTarget) {
	if len(targets) == 1 {
		log.Printf("postgres connect failed (check database URL, connectivity, and credentials)")
		exitFunc(ExitConnectFailure)
	}
}

// exitQueryErrorIf exits with ExitQueryError on one-shot single-target stats query failure.
// Daemon mode (interval > 0) logs and continues so a transient query error does not kill the process.
func exitQueryErrorIf(cfg *config.Config, targets []config.DatabaseTarget, queryFailed bool) {
	if !queryFailed || cfg.Interval > 0 || len(targets) != 1 {
		return
	}
	log.Printf("postgres stats query failed")
	exitFunc(ExitQueryError)
}

// handleConnectFailure notifies (if configured) then exits 2 for single-target, or logs for multi-DB.
func handleConnectFailure(ctx context.Context, senders []notify.Sender, targetCfg *config.Config, cluster, client, ns, db string, connectErr error, targets []config.DatabaseTarget) {
	exitStrictIf(targetCfg, run.NotifyConnectFailure(ctx, senders, targetCfg, cluster, client, ns, db, connectErr))
	if len(targets) == 1 {
		exitConnectFailureIf(targets)
		return
	}
	log.Printf("postgres connect failed [%s]: %v", client, connectErr)
}

// runOneTarget connects to one database target, applies threshold defaults, and
// runs the first check. On connect or threshold errors it notifies (if configured)
// and returns; single-target mode exits the process on connect failure.
func runOneTarget(ctx context.Context, t config.DatabaseTarget, cfg *config.Config, senders []notify.Sender, st store.MetricsStorer, targets []config.DatabaseTarget) {
	targetCfg := cfg.ConfigForTarget(t)
	runCluster, runClient, runNamespace, runDatabase := run.RunContextStrings(ctx, targetCfg, t.URL)

	pool, err := postgres.Pool(ctx, t.URL)
	if err != nil {
		handleConnectFailure(ctx, senders, targetCfg, runCluster, runClient, runNamespace, runDatabase, err, targets)
		return
	}
	defer pool.Close()

	// pgxpool.New is lazy; Ping forces a real connection so connect failures map to exit 2.
	if err := pool.Ping(ctx); err != nil {
		handleConnectFailure(ctx, senders, targetCfg, runCluster, runClient, runNamespace, runDatabase, err, targets)
		return
	}

	if err := run.ApplyThresholdDefaults(ctx, pool, targetCfg); err != nil {
		exitStrictIf(targetCfg, run.NotifyConnectFailure(ctx, senders, targetCfg, runCluster, runClient, runNamespace, runDatabase, err))
		log.Printf("threshold config error [%s]: %v", t.Client, err)
		exitQueryErrorIf(targetCfg, targets, true)
		return
	}
	outcome := run.MakeRunFunc(ctx, pool, targetCfg, senders, st, runCluster, runClient, runNamespace, runDatabase)()
	exitStrictIf(targetCfg, outcome.DeliveryFailed)
	exitQueryErrorIf(targetCfg, targets, outcome.QueryFailed)
}

// runTickerLoop repeats runOneTarget for every target every cfg.Interval seconds
// until ctx is cancelled (SIGINT/SIGTERM).
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
