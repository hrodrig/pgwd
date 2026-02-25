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

// Send posts a Slack message when a threshold is exceeded.
func (s *Slack) Send(ctx context.Context, ev Event) error {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	ts := time.Now().Format("2006-01-02 15:04:05")
	var b strings.Builder
	if ev.Threshold == "test" {
		b.WriteString(":white_check_mark: *pgwd* – Test notification\n")
	} else if ev.Threshold == "connect_failure" {
		b.WriteString(":warning: *pgwd* – Connection failure\n")
	} else {
		b.WriteString(":warning: *pgwd* – Threshold exceeded\n")
	}
	b.WriteString("*" + ev.Message + "*\n")
	// Health-check style: always Time; then Client (host/service), Database, Cluster, Namespace when present
	b.WriteString(fmt.Sprintf("• *Time*: %s\n", ts))
	if ev.Client != "" {
		b.WriteString(fmt.Sprintf("• *Client*: %s\n", ev.Client))
	}
	if ev.Database != "" {
		b.WriteString(fmt.Sprintf("• *Database*: %s\n", ev.Database))
	}
	if ev.Cluster != "" {
		b.WriteString(fmt.Sprintf("• *Cluster*: %s\n", ev.Cluster))
	}
	if ev.Namespace != "" {
		b.WriteString(fmt.Sprintf("• *Namespace*: %s\n", ev.Namespace))
	}
	connLine := fmt.Sprintf("• *Connections*: total=%d, active=%d, idle=%d", ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle)
	if ev.Threshold == "test" {
		connLine += " (delivery check)"
	} else if ev.Threshold == "connect_failure" {
		connLine += " (connection failed)"
	} else {
		connLine += fmt.Sprintf(" (limit %s=%d)", ev.Threshold, ev.ThresholdValue)
	}
	b.WriteString(connLine)

	// Attachment color = vertical bar in Slack: green (OK), red (error), yellow (others)
	var color string
	switch ev.Threshold {
	case "test":
		color = "good" // green
	case "connect_failure":
		color = "danger" // red
	default:
		color = "warning" // yellow (threshold exceeded, etc.)
	}

	body := map[string]any{
		"attachments": []map[string]any{
			{
				"color":    color,
				"text":     b.String(),
				"fallback": ev.Message,
			},
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
