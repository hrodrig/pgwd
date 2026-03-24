package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/postgres"
)

// ---------------------------------------------------------------------------
// Slack.Send
// ---------------------------------------------------------------------------

func TestSlackSend_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	err := s.Send(context.Background(), Event{
		Stats:     postgres.ConnectionStats{Total: 10, Active: 3, Idle: 7},
		Threshold: "test",
		Message:   "unit test",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
}

func TestSlackSend_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	err := s.Send(context.Background(), Event{Threshold: "test", Message: "fail"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "slack webhook returned") {
		t.Errorf("error = %q, want substring %q", err, "slack webhook returned")
	}
}

func TestSlackSend_InvalidURL(t *testing.T) {
	s := &Slack{WebhookURL: "://bad"}
	err := s.Send(context.Background(), Event{Threshold: "test", Message: "x"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestSlackSend_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	err := s.Send(ctx, Event{Threshold: "test", Message: "cancelled"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Loki.Send
// ---------------------------------------------------------------------------

func TestLokiSend_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	l := &Loki{URL: srv.URL, Client: srv.Client()}
	err := l.Send(context.Background(), Event{
		Stats:     postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold: "test",
		Message:   "unit test",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
}

func TestLokiSend_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	l := &Loki{URL: srv.URL, Client: srv.Client()}
	err := l.Send(context.Background(), Event{Threshold: "test", Message: "fail"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "loki push returned") {
		t.Errorf("error = %q, want substring %q", err, "loki push returned")
	}
}

func TestLokiSend_OrgIDHeader(t *testing.T) {
	var gotOrgID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrgID = r.Header.Get("X-Scope-OrgID")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	l := &Loki{URL: srv.URL, OrgID: "tenant-42", Client: srv.Client()}
	if err := l.Send(context.Background(), Event{Threshold: "test", Message: "org"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotOrgID != "tenant-42" {
		t.Errorf("X-Scope-OrgID = %q, want %q", gotOrgID, "tenant-42")
	}
}

func TestLokiSend_BearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	l := &Loki{URL: srv.URL, BearerToken: "secret-token", Client: srv.Client()}
	if err := l.Send(context.Background(), Event{Threshold: "test", Message: "auth"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-token")
	}
}

func TestLokiSend_InvalidURL(t *testing.T) {
	l := &Loki{URL: "://bad"}
	err := l.Send(context.Background(), Event{Threshold: "test", Message: "x"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// ---------------------------------------------------------------------------
// slackHeader
// ---------------------------------------------------------------------------

func TestSlackHeader(t *testing.T) {
	tests := []struct {
		name     string
		ev       Event
		wantSub  string
		wantSubs []string
	}{
		{
			name:    "test threshold",
			ev:      Event{Threshold: "test", Message: "m"},
			wantSub: ":white_check_mark:",
		},
		{
			name:    "connect_failure",
			ev:      Event{Threshold: "connect_failure", Message: "m"},
			wantSub: ":warning:",
		},
		{
			name:    "too_many_clients",
			ev:      Event{Threshold: "too_many_clients", Message: "m"},
			wantSub: ":rotating_light:",
		},
		{
			name:    "resolution",
			ev:      Event{Threshold: "resolution", Message: "m"},
			wantSub: ":green_circle:",
		},
		{
			name:    "level attention",
			ev:      Event{Threshold: "total", Level: "attention", Message: "m"},
			wantSub: ":large_yellow_circle:",
		},
		{
			name:    "level alert",
			ev:      Event{Threshold: "total", Level: "alert", Message: "m"},
			wantSub: ":large_orange_circle:",
		},
		{
			name:    "level danger",
			ev:      Event{Threshold: "total", Level: "danger", Message: "m"},
			wantSub: ":red_circle:",
		},
		{
			name:    "default no level",
			ev:      Event{Threshold: "total", Message: "m"},
			wantSub: ":warning:",
		},
		{
			name: "context fields present",
			ev: Event{
				Threshold: "test", Message: "m",
				Client: "pod/myapp", Cluster: "prod",
				Database: "mydb", Namespace: "default",
			},
			wantSubs: []string{"*Client*: pod/myapp", "*Cluster*: prod", "*Database*: mydb", "*Namespace*: default"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slackHeader(tt.ev, "2025-01-01 00:00:00")
			if tt.wantSub != "" && !strings.Contains(got, tt.wantSub) {
				t.Errorf("slackHeader = %q, want substring %q", got, tt.wantSub)
			}
			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("slackHeader missing %q in:\n%s", sub, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// slackColor
// ---------------------------------------------------------------------------

func TestSlackColor(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want string
	}{
		{"test", Event{Threshold: "test"}, "good"},
		{"resolution", Event{Threshold: "resolution"}, "good"},
		{"connect_failure", Event{Threshold: "connect_failure"}, "danger"},
		{"too_many_clients", Event{Threshold: "too_many_clients"}, "danger"},
		{"attention level", Event{Threshold: "total", Level: "attention"}, "#FFD700"},
		{"alert level", Event{Threshold: "total", Level: "alert"}, "#FF8C00"},
		{"danger level", Event{Threshold: "total", Level: "danger"}, "#CC0000"},
		{"default", Event{Threshold: "total"}, "warning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slackColor(tt.ev); got != tt.want {
				t.Errorf("slackColor = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// slackConnLine
// ---------------------------------------------------------------------------

func TestSlackConnLine(t *testing.T) {
	tests := []struct {
		name     string
		ev       Event
		wantSubs []string
	}{
		{
			name: "basic stats no max",
			ev: Event{
				Stats:     postgres.ConnectionStats{Total: 10, Active: 3, Idle: 7},
				Threshold: "total", ThresholdValue: 50,
			},
			wantSubs: []string{"total=10", "active=3", "idle=7", "(limit total=50)"},
		},
		{
			name: "with max_connections",
			ev: Event{
				Stats:          postgres.ConnectionStats{Total: 10, Active: 3, Idle: 7},
				Threshold:      "total",
				ThresholdValue: 50,
				MaxConnections: 100,
			},
			wantSubs: []string{"max_connections=100"},
		},
		{
			name: "with max_connections override",
			ev: Event{
				Stats:                    postgres.ConnectionStats{Total: 10},
				Threshold:                "total",
				MaxConnections:           100,
				MaxConnectionsIsOverride: true,
			},
			wantSubs: []string{"max_connections=100", "(test override)"},
		},
		{
			name:     "test threshold suffix",
			ev:       Event{Threshold: "test"},
			wantSubs: []string{"(delivery check)"},
		},
		{
			name:     "connect_failure suffix",
			ev:       Event{Threshold: "connect_failure"},
			wantSubs: []string{"(connection failed)"},
		},
		{
			name:     "too_many_clients suffix",
			ev:       Event{Threshold: "too_many_clients"},
			wantSubs: []string{"(too many clients"},
		},
		{
			name:     "resolution suffix",
			ev:       Event{Threshold: "resolution"},
			wantSubs: []string{"(returned to normal)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slackConnLine(tt.ev)
			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("slackConnLine missing %q in %q", sub, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildLokiLine
// ---------------------------------------------------------------------------

func TestBuildLokiLine(t *testing.T) {
	tests := []struct {
		name     string
		ev       Event
		wantSubs []string
		wantNot  []string
	}{
		{
			name: "all context fields",
			ev: Event{
				Stats:     postgres.ConnectionStats{Total: 10, Active: 5, Idle: 5},
				Threshold: "total", ThresholdValue: 50,
				Message: "threshold exceeded",
				Cluster: "prod", Database: "mydb", Client: "pod/app",
			},
			wantSubs: []string{
				"pgwd [cluster=prod database=mydb client=pod/app]:",
				"threshold exceeded",
				"total=10", "active=5", "idle=5",
				"(limit total=50)",
			},
		},
		{
			name: "no context fields",
			ev: Event{
				Stats:     postgres.ConnectionStats{Total: 1, Active: 0, Idle: 1},
				Threshold: "test",
				Message:   "hello",
			},
			wantSubs: []string{"pgwd: hello", "total=1", "active=0", "idle=1", "(delivery check)"},
			wantNot:  []string{"cluster=", "database=", "client="},
		},
		{
			name: "partial context (cluster only)",
			ev: Event{
				Stats:     postgres.ConnectionStats{Total: 3, Active: 1, Idle: 2},
				Threshold: "active", ThresholdValue: 5,
				Message: "active high", Cluster: "staging",
			},
			wantSubs: []string{"pgwd [cluster=staging]:", "active high"},
			wantNot:  []string{"database=", "client="},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLokiLine(tt.ev)
			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("buildLokiLine missing %q in:\n%s", sub, got)
				}
			}
			for _, sub := range tt.wantNot {
				if strings.Contains(got, sub) {
					t.Errorf("buildLokiLine should not contain %q in:\n%s", sub, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// thresholdToLevel
// ---------------------------------------------------------------------------

func TestThresholdToLevel(t *testing.T) {
	tests := []struct {
		threshold string
		want      string
	}{
		{"too_many_clients", "danger"},
		{"connect_failure", "danger"},
		{"resolution", "ok"},
		{"total", "attention"},
		{"active", "attention"},
		{"idle", "attention"},
		{"stale", "attention"},
		{"test", "attention"},
		{"unknown_threshold", "attention"},
	}
	for _, tt := range tests {
		t.Run(tt.threshold, func(t *testing.T) {
			if got := thresholdToLevel(tt.threshold); got != tt.want {
				t.Errorf("thresholdToLevel(%q) = %q, want %q", tt.threshold, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// eventLevel
// ---------------------------------------------------------------------------

func TestEventLevel(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want string
	}{
		{
			name: "explicit level returned as-is",
			ev:   Event{Threshold: "total", Level: "alert"},
			want: "alert",
		},
		{
			name: "empty level delegates to thresholdToLevel",
			ev:   Event{Threshold: "connect_failure"},
			want: "danger",
		},
		{
			name: "empty level for resolution",
			ev:   Event{Threshold: "resolution"},
			want: "ok",
		},
		{
			name: "custom level value",
			ev:   Event{Threshold: "total", Level: "custom"},
			want: "custom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventLevel(tt.ev); got != tt.want {
				t.Errorf("eventLevel = %q, want %q", got, tt.want)
			}
		})
	}
}
