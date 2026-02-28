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
	URL    string
	Labels map[string]string // e.g. job=pgwd, level=warning
	Client *http.Client
}

// lokiPushBody matches Loki's /loki/api/v1/push JSON.
type lokiPushBody struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [[nanosecond_timestamp, line], ...]
}

// Send pushes a log line to Loki.
func (l *Loki) Send(ctx context.Context, ev Event) error {
	client := l.Client
	if client == nil {
		client = http.DefaultClient
	}

	labels := make(map[string]string)
	for k, v := range l.Labels {
		labels[k] = v
	}
	if labels["job"] == "" {
		labels["job"] = "pgwd"
	}
	labels["threshold"] = ev.Threshold

	line := fmt.Sprintf("pgwd: %s | total=%d active=%d idle=%d", ev.Message, ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle)
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
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
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
