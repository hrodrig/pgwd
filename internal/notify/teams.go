package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Teams sends events to a Microsoft Teams incoming webhook.
type Teams struct {
	WebhookURL string
	Client     *http.Client // optional override for tests
}

// Send posts a plain-text summary to Teams.
func (t *Teams) Send(ctx context.Context, ev Event) error {
	text := SummaryText(ev, "")
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("teams payload: %w", err)
	}
	if err := postJSONWithRetryClient(ctx, t.Client, t.WebhookURL, body, nil); err != nil {
		return fmt.Errorf("teams webhook: %w", err)
	}
	return nil
}
