package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"text/template"
)

// GenericWebhook sends events to a custom HTTP endpoint with optional JWT headers and HMAC signing.
type GenericWebhook struct {
	WebhookURL   string
	JSONKey      string
	Headers      map[string]string
	ExtraFields  map[string]string
	BodyTemplate string
	HMACSecret   string
	HMACHeader   string
	Client       *http.Client // optional override for tests

	tmplOnce sync.Once
	tmpl     *template.Template
	tmplErr  error
}

type genericTemplateData struct {
	Message   string
	Threshold string
	Level     string
	Total     int
	Active    int
	Idle      int
	MaxConn   int
	Cluster   string
	Client    string
	Database  string
	Namespace string
	EventType string
}

// Send posts a JSON payload to the configured webhook URL.
func (g *GenericWebhook) Send(ctx context.Context, ev Event) error {
	body, err := g.buildBody(ev)
	if err != nil {
		return err
	}
	header := g.HMACHeader
	if header == "" {
		header = "X-Pgwd-Signature"
	}
	headers := cloneStringMap(g.Headers)
	if err := postJSONWithRetryClient(ctx, g.Client, g.WebhookURL, body, func(req *http.Request) {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if g.HMACSecret != "" {
			req.Header.Set(header, signHMACBody(body, g.HMACSecret))
		}
	}); err != nil {
		return fmt.Errorf("generic webhook: %w", err)
	}
	return nil
}

func (g *GenericWebhook) buildBody(ev Event) ([]byte, error) {
	if g.BodyTemplate != "" {
		return g.renderTemplateBody(ev)
	}
	key := g.JSONKey
	if key == "" {
		key = "text"
	}
	payload := map[string]string{key: SummaryText(ev, "")}
	for k, v := range g.ExtraFields {
		payload[k] = v
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("generic webhook payload: %w", err)
	}
	return raw, nil
}

func (g *GenericWebhook) renderTemplateBody(ev Event) ([]byte, error) {
	g.tmplOnce.Do(func() {
		g.tmpl, g.tmplErr = template.New("generic").Parse(g.BodyTemplate)
	})
	if g.tmplErr != nil {
		return nil, fmt.Errorf("generic webhook body_template: %w", g.tmplErr)
	}
	data := genericTemplateData{
		Message:   jsonEscapeString(ev.Message),
		Threshold: jsonEscapeString(ev.Threshold),
		Level:     jsonEscapeString(eventLevel(ev)),
		Total:     ev.Stats.Total,
		Active:    ev.Stats.Active,
		Idle:      ev.Stats.Idle,
		MaxConn:   ev.MaxConnections,
		Cluster:   jsonEscapeString(ev.Cluster),
		Client:    jsonEscapeString(ev.Client),
		Database:  jsonEscapeString(ev.Database),
		Namespace: jsonEscapeString(ev.Namespace),
		EventType: jsonEscapeString(ev.Threshold),
	}
	var buf bytes.Buffer
	if err := g.tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("generic webhook body_template execute: %w", err)
	}
	raw := buf.Bytes()
	if !json.Valid(raw) {
		return nil, fmt.Errorf("generic webhook body_template rendered invalid JSON")
	}
	return raw, nil
}

// jsonEscapeString returns s escaped for embedding inside a JSON string literal
// (no surrounding quotes), so body_template values with " or \ stay valid JSON.
func jsonEscapeString(s string) string {
	b, err := json.Marshal(s)
	if err != nil || len(b) < 2 {
		return ""
	}
	return string(b[1 : len(b)-1])
}

func signHMACBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
