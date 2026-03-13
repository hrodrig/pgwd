package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Loki sends log entries to Loki's push API.
type Loki struct {
	URL         string
	Labels      map[string]string // e.g. app=pgwd, env=prod
	OrgID       string            // X-Scope-OrgID header (multi-tenancy)
	BearerToken string            // Authorization: Bearer <token>
	Client      *http.Client
}

// lokiPushBody matches Loki's /loki/api/v1/push JSON.
type lokiPushBody struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [[nanosecond_timestamp, line], ...]
}

// PushPayload returns the JSON body that Send posts to Loki. Useful for debugging and tests.
func (l *Loki) PushPayload(ev Event) ([]byte, error) {
	labels := make(map[string]string)
	for k, v := range l.Labels {
		labels[k] = v
	}
	if labels["app"] == "" {
		labels["app"] = "pgwd"
	}
	labels["threshold"] = ev.Threshold
	labels["level"] = eventLevel(ev)
	if ev.Namespace != "" {
		labels["namespace"] = ev.Namespace
	}
	if ev.Database != "" {
		labels["database"] = ev.Database
	}
	if ev.Cluster != "" {
		labels["cluster"] = ev.Cluster
	}

	prefix := "pgwd:"
	if ev.Cluster != "" || ev.Database != "" {
		var parts []string
		if ev.Cluster != "" {
			parts = append(parts, fmt.Sprintf("cluster=%s", ev.Cluster))
		}
		if ev.Database != "" {
			parts = append(parts, fmt.Sprintf("database=%s", ev.Database))
		}
		prefix = fmt.Sprintf("pgwd [%s]:", strings.Join(parts, " "))
	}
	line := fmt.Sprintf("%s %s | total=%d active=%d idle=%d", prefix, ev.Message, ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle)
	if ev.MaxConnections > 0 {
		line += fmt.Sprintf(" max_connections=%d", ev.MaxConnections)
		if ev.MaxConnectionsIsOverride {
			line += " (test override)"
		}
	}
	if ev.Threshold == "test" {
		line += " (delivery check)"
	} else if ev.Threshold == "connect_failure" {
		line += " (connection failed)"
	} else if ev.Threshold == "too_many_clients" {
		line += " (too many clients — DB saturated)"
	} else {
		line += fmt.Sprintf(" (limit %s=%d)", ev.Threshold, ev.ThresholdValue)
	}
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	body := lokiPushBody{
		Streams: []lokiStream{{
			Stream: labels,
			Values: [][]string{{ts, line}},
		}},
	}
	return json.Marshal(body)
}

// Send pushes a log line to Loki.
func (l *Loki) Send(ctx context.Context, ev Event) error {
	client := l.Client
	if client == nil {
		client = http.DefaultClient
	}

	raw, err := l.PushPayload(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if l.OrgID != "" {
		req.Header.Set("X-Scope-OrgID", l.OrgID)
	}
	if l.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+l.BearerToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("loki push returned %s", resp.Status)
	}
	return nil
}

// eventLevel returns the severity level for Loki labels. Uses ev.Level when set, else derives from threshold.
func eventLevel(ev Event) string {
	if ev.Level != "" {
		return ev.Level
	}
	return thresholdToLevel(ev.Threshold)
}

// thresholdToLevel maps threshold to severity level for Loki labels (attention, alert, danger).
func thresholdToLevel(threshold string) string {
	switch threshold {
	case "too_many_clients", "connect_failure":
		return "danger"
	case "total", "active", "idle", "stale":
		return "attention"
	case "test":
		return "attention"
	default:
		return "attention"
	}
}

// ParseLokiLabels parses "k1=v1,k2=v2" into a map.
func ParseLokiLabels(s string) map[string]string {
	m := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}
