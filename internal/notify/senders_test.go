package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/postgres"
)

func TestPagerDutySeverity(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		def  string
		want string
	}{
		{"danger level", Event{Level: "danger", Threshold: "total"}, "warning", "critical"},
		{"too_many_clients", Event{Threshold: "too_many_clients"}, "warning", "critical"},
		{"connect_failure", Event{Threshold: "connect_failure"}, "warning", "critical"},
		{"alert level", Event{Level: "alert", Threshold: "total"}, "warning", "warning"},
		{"attention", Event{Level: "attention", Threshold: "total"}, "warning", "info"},
		{"test", Event{Threshold: "test"}, "warning", "info"},
		{"default severity", Event{Threshold: "total"}, "critical", "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pagerDutySeverity(tt.ev, tt.def); got != tt.want {
				t.Errorf("pagerDutySeverity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPagerDutyDetails(t *testing.T) {
	ev := Event{
		Stats:          postgres.ConnectionStats{Total: 10, Active: 3, Idle: 7},
		Threshold:      "total",
		ThresholdValue: 50,
		MaxConnections: 100,
		Database:       "mydb",
		Client:         "monitor-a",
		Cluster:        "prod",
		Namespace:      "default",
	}
	d := pagerDutyDetails(ev)
	if d["total"] != 10 || d["active"] != 3 || d["idle"] != 7 {
		t.Errorf("connection stats: %v", d)
	}
	if d["threshold"] != "total" || d["threshold_value"] != 50 {
		t.Errorf("threshold fields: %v", d)
	}
	if d["database"] != "mydb" || d["cluster"] != "prod" {
		t.Errorf("context: %v", d)
	}
}

func TestPagerDutySend_Success(t *testing.T) {
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

	pd := &PagerDuty{
		RoutingKey: "rk-test",
		Severity:   "warning",
		Source:     "pgwd-test",
		EventsURL:  srv.URL,
		Client:     srv.Client(),
	}
	err := pd.Send(context.Background(), Event{
		Stats:     postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold: "test",
		Message:   "pagerduty test",
		Cluster:   "prod",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody.RoutingKey != "rk-test" || gotBody.EventAction != "trigger" {
		t.Errorf("envelope: %+v", gotBody)
	}
	if gotBody.DedupKey != "pgwd:_:prod:_:test" {
		t.Errorf("dedup_key = %q, want pgwd:_:prod:_:test", gotBody.DedupKey)
	}
	if gotBody.Payload.Source != "pgwd-test" || gotBody.Payload.Severity != "info" {
		t.Errorf("payload: %+v", gotBody.Payload)
	}
	if gotBody.Payload.CustomDetails["cluster"] != "prod" {
		t.Errorf("custom_details: %v", gotBody.Payload.CustomDetails)
	}
}

func TestTeamsSend_Success(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode: %v", err)
		}
		gotText = payload["text"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	teams := &Teams{WebhookURL: srv.URL, Client: srv.Client()}
	err := teams.Send(context.Background(), Event{
		Stats:     postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold: "test",
		Message:   "hello teams",
		Cluster:   "prod",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotText, "hello teams") {
		t.Errorf("text missing message: %q", gotText)
	}
	if !strings.Contains(gotText, "Cluster: prod") {
		t.Errorf("text missing cluster: %q", gotText)
	}
}

func TestGenericWebhookSend_DefaultPayload(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	gw := &GenericWebhook{
		WebhookURL:  srv.URL,
		JSONKey:     "message",
		ExtraFields: map[string]string{"source": "pgwd"},
		Client:      srv.Client(),
	}
	if err := gw.Send(context.Background(), Event{Threshold: "test", Message: "generic test"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got["source"] != "pgwd" {
		t.Errorf("extra field source = %q", got["source"])
	}
	if !strings.Contains(got["message"], "generic test") {
		t.Errorf("message = %q", got["message"])
	}
}

func TestGenericWebhookSend_CustomHeaders(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	gw := &GenericWebhook{
		WebhookURL: srv.URL,
		Headers:    map[string]string{"Authorization": "Bearer jwt-token"},
		Client:     srv.Client(),
	}
	if err := gw.Send(context.Background(), Event{Threshold: "test", Message: "auth"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAuth != "Bearer jwt-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestGenericWebhookSend_HMAC(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Custom-Sig")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	gw := &GenericWebhook{
		WebhookURL: srv.URL,
		HMACSecret: "secret",
		HMACHeader: "X-Custom-Sig",
		Client:     srv.Client(),
	}
	if err := gw.Send(context.Background(), Event{Threshold: "test", Message: "signed"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("signature = %q", gotSig)
	}
}

func TestGenericWebhookSend_BodyTemplate(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 1, InitialBackoff: 0, MaxBackoff: 0})

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	gw := &GenericWebhook{
		WebhookURL:   srv.URL,
		BodyTemplate: `{"text":"{{.Message}}","event":"{{.EventType}}","total":{{.Total}}}`,
		Client:       srv.Client(),
	}
	ev := Event{
		Stats:     postgres.ConnectionStats{Total: 9},
		Threshold: "test",
		Message:   "tmpl",
	}
	if err := gw.Send(context.Background(), ev); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got["text"] != "tmpl" || got["event"] != "test" {
		t.Errorf("payload = %v", got)
	}
	if got["total"].(float64) != 9 {
		t.Errorf("total = %v", got["total"])
	}
}

func TestGenericWebhookSend_InvalidTemplateJSON(t *testing.T) {
	gw := &GenericWebhook{
		WebhookURL:   "http://example.com",
		BodyTemplate: `not json {{.Message}}`,
	}
	err := gw.Send(context.Background(), Event{Threshold: "test", Message: "x"})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("err = %v", err)
	}
}

func TestSummaryText(t *testing.T) {
	ev := Event{
		Stats:     postgres.ConnectionStats{Total: 1, Active: 0, Idle: 1},
		Threshold: "test",
		Message:   "msg",
		Database:  "db1",
	}
	got := SummaryText(ev, "2025-01-01 00:00:00")
	if !strings.Contains(got, "Test notification") {
		t.Errorf("missing title: %q", got)
	}
	if !strings.Contains(got, "Database: db1") {
		t.Errorf("missing database: %q", got)
	}
}
