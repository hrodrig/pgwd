package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	text := fmt.Sprintf(
		":warning: *pgwd* – Threshold exceeded\n*%s*\nConnections: total=%d, active=%d, idle=%d (limit %s=%d)",
		ev.Message, ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle, ev.Threshold, ev.ThresholdValue,
	)
	body := map[string]any{
		"text": text,
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
