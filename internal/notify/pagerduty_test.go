package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hrodrig/pgwd/internal/postgres"
)

func TestPagerDutyDedupKey(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want string
	}{
		{
			name: "connections threshold",
			ev:   Event{Threshold: "total", Client: "c", Cluster: "cl", Database: "d"},
			want: "pgwd:c:cl:d:connections",
		},
		{
			name: "resolution shares connections key",
			ev:   Event{Threshold: "resolution", Client: "c", Cluster: "cl", Database: "d"},
			want: "pgwd:c:cl:d:connections",
		},
		{
			name: "long_query",
			ev:   Event{Threshold: "long_query", Client: "c", Cluster: "cl", Database: "d"},
			want: "pgwd:c:cl:d:long_query",
		},
		{
			name: "connect_failure",
			ev:   Event{Threshold: "connect_failure", Client: "c", Cluster: "cl", Database: "d"},
			want: "pgwd:c:cl:d:connect",
		},
		{
			name: "too_many_clients",
			ev:   Event{Threshold: "too_many_clients", Client: "c", Cluster: "cl", Database: "d"},
			want: "pgwd:c:cl:d:connect",
		},
		{
			name: "test",
			ev:   Event{Threshold: "test", Client: "c", Cluster: "cl", Database: "d"},
			want: "pgwd:c:cl:d:test",
		},
		{
			name: "empty segments become underscore",
			ev:   Event{Threshold: "active"},
			want: "pgwd:_:_:_:connections",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pagerDutyDedupKey(tt.ev); got != tt.want {
				t.Fatalf("pagerDutyDedupKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPagerDutyEventAction(t *testing.T) {
	if got := pagerDutyEventAction(Event{Threshold: "resolution"}); got != "resolve" {
		t.Fatalf("resolution action = %q, want resolve", got)
	}
	if got := pagerDutyEventAction(Event{Threshold: "total"}); got != "trigger" {
		t.Fatalf("total action = %q, want trigger", got)
	}
}

func TestPagerDuty_Send_triggerIncludesDedupKey(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var gotBody pagerDutyEnvelope
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	pd := &PagerDuty{RoutingKey: "rk", EventsURL: srv.URL, Client: srv.Client()}
	err := pd.Send(context.Background(), Event{
		Stats:     postgres.ConnectionStats{Total: 90, Active: 80, Idle: 10},
		Threshold: "total",
		Level:     "alert",
		Message:   "high connections",
		Client:    "c",
		Cluster:   "cl",
		Database:  "d",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody.EventAction != "trigger" {
		t.Errorf("event_action = %q, want trigger", gotBody.EventAction)
	}
	if gotBody.DedupKey != "pgwd:c:cl:d:connections" {
		t.Errorf("dedup_key = %q, want pgwd:c:cl:d:connections", gotBody.DedupKey)
	}
}

func TestPagerDuty_Send_resolutionIsResolveAction(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var gotBody pagerDutyEnvelope
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	pd := &PagerDuty{RoutingKey: "rk", EventsURL: srv.URL, Client: srv.Client()}
	err := pd.Send(context.Background(), Event{
		Threshold: "resolution",
		Message:   "returned to normal",
		Client:    "c",
		Cluster:   "cl",
		Database:  "d",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody.EventAction != "resolve" {
		t.Errorf("event_action = %q, want resolve", gotBody.EventAction)
	}
	if gotBody.DedupKey != "pgwd:c:cl:d:connections" {
		t.Errorf("dedup_key = %q, want pgwd:c:cl:d:connections", gotBody.DedupKey)
	}
}
