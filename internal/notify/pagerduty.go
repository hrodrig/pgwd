package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDuty sends events to PagerDuty Events API v2.
type PagerDuty struct {
	RoutingKey string
	Severity   string // default severity when event mapping does not apply
	Source     string
	EventsURL  string       // default pagerDutyEventsURL; override in tests
	Client     *http.Client // optional override for tests
}

type pagerDutyEnvelope struct {
	RoutingKey  string           `json:"routing_key"`
	EventAction string           `json:"event_action"`
	Payload     pagerDutyPayload `json:"payload"`
}

type pagerDutyPayload struct {
	Summary       string         `json:"summary"`
	Source        string         `json:"source"`
	Severity      string         `json:"severity"`
	Timestamp     string         `json:"timestamp"`
	CustomDetails map[string]any `json:"custom_details"`
}

// Send posts a trigger event to PagerDuty.
func (p *PagerDuty) Send(ctx context.Context, ev Event) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	summary := ev.Message
	if summary == "" {
		summary = summaryTitle(ev)
	}
	source := p.Source
	if source == "" {
		source = "pgwd"
	}
	body, err := json.Marshal(pagerDutyEnvelope{
		RoutingKey:  p.RoutingKey,
		EventAction: "trigger",
		Payload: pagerDutyPayload{
			Summary:       summary,
			Source:        source,
			Severity:      pagerDutySeverity(ev, p.Severity),
			Timestamp:     ts,
			CustomDetails: pagerDutyDetails(ev),
		},
	})
	if err != nil {
		return fmt.Errorf("pagerduty payload: %w", err)
	}
	url := p.EventsURL
	if url == "" {
		url = pagerDutyEventsURL
	}
	if err := postJSONWithRetryClient(ctx, p.Client, url, body, nil); err != nil {
		return fmt.Errorf("pagerduty: %w", err)
	}
	return nil
}

func pagerDutySeverity(ev Event, defaultSeverity string) string {
	if ev.Level == "danger" || ev.Threshold == "too_many_clients" || ev.Threshold == "connect_failure" {
		return "critical"
	}
	if ev.Level == "alert" {
		return "warning"
	}
	if ev.Level == "attention" || ev.Threshold == "resolution" || ev.Threshold == "test" {
		return "info"
	}
	if defaultSeverity != "" {
		return defaultSeverity
	}
	return "warning"
}

func pagerDutyDetails(ev Event) map[string]any {
	details := map[string]any{
		"total":           ev.Stats.Total,
		"active":          ev.Stats.Active,
		"idle":            ev.Stats.Idle,
		"threshold":       ev.Threshold,
		"threshold_value": ev.ThresholdValue,
		"level":           eventLevel(ev),
	}
	if ev.MaxConnections > 0 {
		details["max_connections"] = ev.MaxConnections
		if ev.MaxConnectionsIsOverride {
			details["max_connections_test_override"] = true
		}
	}
	if ev.Database != "" {
		details["database"] = ev.Database
	}
	if ev.Client != "" {
		details["client"] = ev.Client
	}
	if ev.Cluster != "" {
		details["cluster"] = ev.Cluster
	}
	if ev.Namespace != "" {
		details["namespace"] = ev.Namespace
	}
	return details
}
