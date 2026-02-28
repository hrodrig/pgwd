package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Slack sends events to Slack via Incoming Webhook.
type Slack struct {
	WebhookURL string
	Client     *http.Client
}

func slackHeader(ev Event, ts string) string {
	var h string
	switch ev.Threshold {
	case "test":
		h = ":white_check_mark: *pgwd* – Test notification\n"
	case "connect_failure":
		h = ":warning: *pgwd* – Connection failure\n"
	case "too_many_clients":
		h = ":rotating_light: *pgwd* – URGENT: too many clients (DB saturated)\n"
	default:
		h = ":warning: *pgwd* – Threshold exceeded\n"
	}
	h += "*" + ev.Message + "*\n"
	h += fmt.Sprintf("• *Time*: %s\n", ts)
	if ev.Client != "" {
		h += fmt.Sprintf("• *Client*: %s\n", ev.Client)
	}
	if ev.Database != "" {
		h += fmt.Sprintf("• *Database*: %s\n", ev.Database)
	}
	if ev.Cluster != "" {
		h += fmt.Sprintf("• *Cluster*: %s\n", ev.Cluster)
	}
	if ev.Namespace != "" {
		h += fmt.Sprintf("• *Namespace*: %s\n", ev.Namespace)
	}
	return h
}

func slackConnLine(ev Event) string {
	line := fmt.Sprintf("• *Connections*: total=%d active=%d idle=%d", ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle)
	if ev.MaxConnections > 0 {
		line += fmt.Sprintf(" max_connections=%d", ev.MaxConnections)
		if ev.MaxConnectionsIsOverride {
			line += " (test override)"
		}
	}
	switch ev.Threshold {
	case "test":
		line += " (delivery check)"
	case "connect_failure":
		line += " (connection failed)"
	case "too_many_clients":
		line += " (too many clients — DB saturated)"
	default:
		line += fmt.Sprintf(" (limit %s=%d)", ev.Threshold, ev.ThresholdValue)
	}
	return line
}

func slackColor(ev Event) string {
	switch ev.Threshold {
	case "test":
		return "good"
	case "connect_failure", "too_many_clients":
		return "danger"
	default:
		return "warning"
	}
}

// Send posts a Slack message when a threshold is exceeded.
func (s *Slack) Send(ctx context.Context, ev Event) error {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	var b strings.Builder
	b.WriteString(slackHeader(ev, ts))
	b.WriteString(slackConnLine(ev))
	body := map[string]any{
		"attachments": []map[string]any{
			{"color": slackColor(ev), "text": b.String(), "fallback": ev.Message},
		},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(raw))
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
		return fmt.Errorf("slack webhook returned %s", resp.Status)
	}
	return nil
}
