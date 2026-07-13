package validator

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
)

func TestValidateDatabases(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"no databases", &config.Config{}, false, ""},
		{"databases with url", &config.Config{Databases: []config.DatabaseTarget{{URL: "postgres://x/db"}}}, false, ""},
		{"databases missing url", &config.Config{Databases: []config.DatabaseTarget{{URL: ""}}}, true, "databases[0] missing url"},
		{"databases + kube-postgres", &config.Config{Databases: []config.DatabaseTarget{{URL: "x"}}, KubePostgres: "default/svc/pg"}, true, "kube-postgres is not supported with databases"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDatabases(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDatabases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateDatabases() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateClient(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"has client", &config.Config{Client: "x"}, false, ""},
		{"no client single-db", &config.Config{Client: ""}, true, "client is required"},
		{"no client databases", &config.Config{Client: "", Databases: []config.DatabaseTarget{{URL: "x"}}}, true, "client is required when using databases"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateClient() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateDBURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"has url", &config.Config{DBURL: "postgres://x"}, false, ""},
		{"no url single-db", &config.Config{DBURL: ""}, true, "missing database URL"},
		{"no url but databases", &config.Config{DBURL: "", Databases: []config.DatabaseTarget{{URL: "x"}}}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDBURL(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDBURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateDBURL() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateStale(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"no stale threshold", &config.Config{}, false, ""},
		{"stale with age", &config.Config{ThresholdStale: 5, StaleAge: 60}, false, ""},
		{"stale without age", &config.Config{ThresholdStale: 5, StaleAge: 0}, true, "db-stale-age must be > 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStale(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStale() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateStale() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateNotifiers(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"dry-run no notifier", &config.Config{DryRun: true}, false, ""},
		{"has slack", &config.Config{SlackWebhook: "https://x"}, false, ""},
		{"has pagerduty", &config.Config{PagerDutyRoutingKey: "rk"}, false, ""},
		{"no notifier no dry-run", &config.Config{}, true, "no notifier"},
		{"force-notification no notifier", &config.Config{ForceNotification: true, DryRun: true}, true, "force-notification requires"},
		{"notify-on-connect-failure no notifier", &config.Config{NotifyOnConnectFailure: true, DryRun: true}, false, ""},
		{"pagerduty enabled missing key", &config.Config{PagerDutyEnabled: true, DryRun: true}, true, "routing_key is required"},
		{"teams enabled missing webhook", &config.Config{TeamsEnabled: true, DryRun: true}, true, "webhook_url is required"},
		{"generic enabled missing url", &config.Config{GenericEnabled: true, DryRun: true}, true, "webhook_url is required"},
		{"invalid generic template", &config.Config{GenericEnabled: true, GenericWebhookURL: "https://x", GenericBodyTemplate: "{{", DryRun: true}, true, "body_template"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotifiers(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotifiers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateNotifiers() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateKubePostgres(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"no kube-postgres", &config.Config{}, false, ""},
		{"kube-postgres with url", &config.Config{KubePostgres: "default/svc/pg", DBURL: "postgres://localhost:5432/db"}, false, ""},
		{"kube-postgres without url", &config.Config{KubePostgres: "default/svc/pg", DBURL: ""}, true, "kube-postgres requires"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKubePostgres(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKubePostgres() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateKubePostgres() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateKubePostgresFormat(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{"empty", &config.Config{}, false},
		{"valid", &config.Config{KubePostgres: "default/svc/postgres"}, false},
		{"invalid", &config.Config{KubePostgres: "bad"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKubePostgresFormat(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKubePostgresFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateKubeLoki(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"no kube-loki", &config.Config{}, false, ""},
		{"kube-loki valid", &config.Config{KubeLoki: "monitoring/svc/loki", KubeLokiLocalPort: 3100, KubeLokiRemotePort: 3100}, false, ""},
		{"kube-loki and loki-url both", &config.Config{KubeLoki: "m/svc/l", LokiURL: "http://x"}, true, "use -kube-loki OR"},
		{"kube-loki local port 0", &config.Config{KubeLoki: "m/svc/l", KubeLokiLocalPort: 0, KubeLokiRemotePort: 3100}, true, "kube-loki-local-port"},
		{"kube-loki remote port invalid", &config.Config{KubeLoki: "m/svc/l", KubeLokiLocalPort: 3100, KubeLokiRemotePort: 0}, true, "kube-loki-remote-port"},
		{"kube-loki invalid format", &config.Config{KubeLoki: "bad", KubeLokiLocalPort: 3100, KubeLokiRemotePort: 3100}, true, "namespace/type/name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKubeLoki(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKubeLoki() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("ValidateKubeLoki() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidateLongQueryAlerts(t *testing.T) {
	if err := ValidateLongQueryAlerts(&config.Config{}); err != nil {
		t.Errorf("no long query: %v", err)
	}
	err := ValidateLongQueryAlerts(&config.Config{LongQueryMinSeconds: 60})
	if err == nil || !strings.Contains(err.Error(), "metrics store") {
		t.Fatalf("want metrics store error: %v", err)
	}
	err = ValidateLongQueryAlerts(&config.Config{
		LongQueryMinSeconds: 60, SqlitePath: "/tmp/x.db", LongQueryCooldownSeconds: 0,
	})
	if err == nil || !strings.Contains(err.Error(), "cooldown") {
		t.Fatalf("want cooldown error: %v", err)
	}
	err = ValidateLongQueryAlerts(&config.Config{
		LongQueryMinSeconds: 60, SqlitePath: "/tmp/x.db", LongQueryCooldownSeconds: 120, LongQueryMinCount: 0,
	})
	if err == nil || !strings.Contains(err.Error(), "min_count") {
		t.Fatalf("want min_count error: %v", err)
	}
	if err := ValidateLongQueryAlerts(&config.Config{
		LongQueryMinSeconds: 60, SqlitePath: "/tmp/x.db", LongQueryCooldownSeconds: 120, LongQueryMinCount: 1,
	}); err != nil {
		t.Errorf("valid: %v", err)
	}
}

func TestValidateMetricsStore(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains string
	}{
		{"no store", &config.Config{}, false, ""},
		{"sqlite path only", &config.Config{SqlitePath: "/tmp/x.db"}, false, ""},
		{"sqlite driver no path", &config.Config{MetricsStoreDriver: "sqlite"}, true, "sqlite.path is required"},
		{"postgres no dsn", &config.Config{MetricsStoreDriver: "postgres"}, true, "metrics_store.dsn"},
		{"postgres with dsn", &config.Config{MetricsStoreDriver: "postgres", MetricsStoreDSN: "postgres://h/p"}, false, ""},
		{"mysql with dsn", &config.Config{MetricsStoreDriver: "mysql", MetricsStoreDSN: "u@tcp(h:3306)/d"}, false, ""},
		{"unsupported driver", &config.Config{MetricsStoreDriver: "oracle"}, true, "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetricsStore(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestValidate_Integration(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{"valid minimal", &config.Config{Client: "x", DBURL: "postgres://localhost/db", DryRun: true}, false},
		{"valid with notifier", &config.Config{Client: "x", DBURL: "postgres://x", SlackWebhook: "https://x"}, false},
		{"missing client", &config.Config{DBURL: "x", DryRun: true}, true},
		{"missing dburl", &config.Config{Client: "x", DryRun: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWarnNotifierTLS(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w

	cfg := &config.Config{
		SlackWebhook:      "http://hooks.example.com/slack",
		LokiURL:           "http://127.0.0.1:3100/push",
		TeamsEnabled:      true,
		TeamsWebhook:      "http://teams.example.com/hook",
		GenericEnabled:    true,
		GenericWebhookURL: "https://hooks.example.com/generic",
	}
	WarnNotifierTLS(cfg)
	_ = w.Close()
	os.Stderr = old

	b, _ := io.ReadAll(r)
	out := string(b)
	if !strings.Contains(out, "Slack") || !strings.Contains(out, "Teams") {
		t.Fatalf("stderr = %q", out)
	}
	if strings.Contains(out, "Loki") {
		t.Fatalf("loopback Loki should not warn: %q", out)
	}
	if strings.Contains(out, "generic") {
		t.Fatalf("https generic should not warn: %q", out)
	}
}
