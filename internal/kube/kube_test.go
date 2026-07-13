package kube

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestValidateKubernetesAccess_InvalidContext(t *testing.T) {
	ctx := context.Background()
	// Use a context that does not exist; should fail (load kubeconfig or list pods)
	err := ValidateKubernetesAccess(ctx, "pgwd-test-nonexistent-context-xyz")
	if err == nil {
		t.Skip("succeeded (cluster/context may exist); cannot assert failure")
	}
	if !strings.Contains(err.Error(), "kubeconfig") && !strings.Contains(err.Error(), "list pods") && !strings.Contains(err.Error(), "context") {
		t.Logf("expected error about kubeconfig/context/pods, got: %v", err)
	}
}

func TestParseKubePostgres(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNS    string
		wantRes   string
		wantError bool
	}{
		{"valid svc", "default/svc/postgres", "default", "svc/postgres", false},
		{"valid pod", "ns/pod/postgres-0", "ns", "pod/postgres-0", false},
		{"valid custom ns", "my-ns/svc/my-db", "my-ns", "svc/my-db", false},
		{"case insensitive type", "ns/SVC/name", "ns", "svc/name", false},
		{"one part", "only-one", "", "", true},
		{"two parts", "two/parts", "", "", true},
		{"invalid type deployment", "ns/deployment/name", "", "", true},
		{"empty namespace", "/svc/name", "", "", true},
		{"empty name", "ns/svc/", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, res, err := ParseKubePostgres(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if ns != tt.wantNS {
				t.Errorf("namespace: got %q, want %q", ns, tt.wantNS)
			}
			if res != tt.wantRes {
				t.Errorf("resource: got %q, want %q", res, tt.wantRes)
			}
		})
	}
}

func TestResolveKubeDBURL_RejectDiscover(t *testing.T) {
	ctx := context.Background()
	secret := PasswordFromSecret{}
	_, err := ResolveKubeDBURL(ctx, "", "postgres://user:DISCOVER_MY_PASSWORD@host/db", secret, 15432)
	if err == nil {
		t.Fatal("expected error for DISCOVER_MY_PASSWORD")
	}
	if !errors.Is(err, errDiscoverPasswordRemoved) {
		t.Errorf("error = %v, want errDiscoverPasswordRemoved", err)
	}
}

func TestReplaceDBURLForKube(t *testing.T) {
	tests := []struct {
		name        string
		dbURL       string
		newPassword string
		localPort   int
		wantHost    string
		wantPass    string
		wantQuery   string
		wantError   bool
	}{
		{
			name:        "replace password and host",
			dbURL:       "postgres://user:DISCOVER_MY_PASSWORD@host:5432/db",
			newPassword: "secret",
			localPort:   15432,
			wantHost:    "127.0.0.1:15432",
			wantPass:    "secret",
		},
		{
			name:        "no password replacement",
			dbURL:       "postgres://user:pass@host:5432/db",
			newPassword: "",
			localPort:   15432,
			wantHost:    "127.0.0.1:15432",
			wantPass:    "pass",
		},
		{
			name:      "invalid URL",
			dbURL:     "://bad",
			wantError: true,
		},
		{
			name:        "query params preserved",
			dbURL:       "postgres://user:old@host/db?sslmode=disable",
			newPassword: "new",
			localPort:   15432,
			wantHost:    "127.0.0.1:15432",
			wantPass:    "new",
			wantQuery:   "sslmode=disable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReplaceDBURLForKube(tt.dbURL, tt.newPassword, tt.localPort)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			u, err := url.Parse(got)
			if err != nil {
				t.Fatalf("result is not a valid URL: %v", err)
			}
			if u.Host != tt.wantHost {
				t.Errorf("host: got %q, want %q", u.Host, tt.wantHost)
			}
			pass, _ := u.User.Password()
			if pass != tt.wantPass {
				t.Errorf("password: got %q, want %q", pass, tt.wantPass)
			}
			if tt.wantQuery != "" && u.RawQuery != tt.wantQuery {
				t.Errorf("query: got %q, want %q", u.RawQuery, tt.wantQuery)
			}
		})
	}
}

func TestRequireKubectl(t *testing.T) {
	if err := RequireKubectl(); err != nil {
		t.Errorf("RequireKubectl() = %v, want nil", err)
	}
}

func TestResolvePod_PodPrefix(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		resource  string
		wantPod   string
		wantError bool
	}{
		{"pod prefix returns name", "pod/my-pod", "my-pod", false},
		{"invalid prefix", "invalid-prefix/name", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod, err := ResolvePod(ctx, "", "ns", tt.resource)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.resource)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pod != tt.wantPod {
				t.Errorf("got %q, want %q", pod, tt.wantPod)
			}
		})
	}
}

func TestClusterName_NonexistentContext(t *testing.T) {
	ctx := context.Background()
	got := ClusterName(ctx, "pgwd-test-nonexistent-context-xyz")
	if got != "" {
		t.Errorf("expected empty string for nonexistent context, got %q", got)
	}
}
