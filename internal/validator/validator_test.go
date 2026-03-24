package validator

import (
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
)

func TestValidateDatabases(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
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
		name      string
		cfg       *config.Config
		wantErr   bool
		contains  string
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
		{"no notifier no dry-run", &config.Config{}, true, "no notifier"},
		{"force-notification no notifier", &config.Config{ForceNotification: true, DryRun: true}, true, "force-notification requires"},
		{"notify-on-connect-failure no notifier", &config.Config{NotifyOnConnectFailure: true, DryRun: true}, true, "notify-on-connect-failure requires"},
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
