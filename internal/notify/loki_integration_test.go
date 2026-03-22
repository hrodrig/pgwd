package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/pgwd/internal/postgres"
)

// lokiQueryResponse matches Loki's /loki/api/v1/query_range JSON.
type lokiQueryResponse struct {
	Data lokiQueryData `json:"data"`
}

type lokiQueryData struct {
	Result []lokiStreamResult `json:"result"`
}

type lokiStreamResult struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [[ts_ns, line], ...]
}

// queryLokiRaw fetches the full query_range response from Loki.
func queryLokiRaw(ctx context.Context, queryBase string, logql string) (*lokiQueryResponse, error) {
	u, err := url.Parse(queryBase)
	if err != nil {
		return nil, err
	}
	basePath := strings.TrimSuffix(u.Path, "/push")
	if basePath == "" || basePath == u.Path {
		basePath = "/loki/api/v1"
	}
	queryURL := fmt.Sprintf("%s://%s%s/query_range?query=%s",
		u.Scheme, u.Host, basePath, url.QueryEscape(logql))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki query returned %s", resp.Status)
	}
	var out lokiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// queryLoki fetches log lines from Loki's query_range API.
func queryLoki(ctx context.Context, queryBase string, logql string) ([]string, error) {
	out, err := queryLokiRaw(ctx, queryBase, logql)
	if err != nil {
		return nil, err
	}
	return extractLinesFromResp(out), nil
}

func extractLinesFromResp(resp *lokiQueryResponse) []string {
	var lines []string
	for _, r := range resp.Data.Result {
		for _, v := range r.Values {
			if len(v) >= 2 {
				lines = append(lines, v[1])
			}
		}
	}
	return lines
}

func TestLoki_Integration(t *testing.T) {
	pushURL := getEnvSkip(t, "PGWD_TEST_LOKI_URL", "Loki push URL for integration test (e.g. http://localhost:3100/loki/api/v1/push)")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send a test event
	loki := &Loki{URL: pushURL, Labels: map[string]string{"app": "pgwd", "env": "test"}}
	ev := Event{
		Stats:          postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold:      "test",
		ThresholdValue: 0,
		Message:        "Test notification",
		MaxConnections: 20,
	}
	if err := loki.Send(ctx, ev); err != nil {
		t.Fatalf("Loki.Send: %v", err)
	}

	// Allow Loki to ingest
	time.Sleep(500 * time.Millisecond)

	// Query Loki (queryLokiRaw derives query_range URL from push URL)
	resp, err := queryLokiRaw(ctx, pushURL, `{app="pgwd"}`)
	if err != nil {
		t.Fatalf("queryLokiRaw: %v", err)
	}
	lines := extractLinesFromResp(resp)
	if len(lines) == 0 {
		t.Fatal("no log lines found in Loki for app=pgwd")
	}

	// Show raw response (same format as e2e-kube)
	respJSON, _ := json.Marshal(resp)
	t.Logf("Verifying log reached Loki...")
	t.Logf("--- Loki query response (raw) ---")
	t.Logf("%s", string(respJSON))
	t.Logf("--- end ---")

	if !hasDeliveryCheckLine(lines) {
		t.Errorf("expected log line with pgwd:, total=5, active=2, idle=3, max_connections=20, delivery check; got lines: %v", lines)
	}
}

// TestLoki_Integration_ShowPayload sends a test event and prints the push payload and query response.
// Run with: PGWD_TEST_LOKI_URL=... PGWD_TEST_LOKI_VERBOSE=1 go test -v -run TestLoki_Integration_ShowPayload
func TestLoki_Integration_ShowPayload(t *testing.T) {
	pushURL := getEnvSkip(t, "PGWD_TEST_LOKI_URL", "Loki push URL")
	if os.Getenv("PGWD_TEST_LOKI_VERBOSE") == "" {
		t.Skip("set PGWD_TEST_LOKI_VERBOSE=1 to run (prints payload and response)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	loki := &Loki{URL: pushURL, Labels: map[string]string{"app": "pgwd", "env": "test"}}
	ev := Event{
		Stats:          postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold:      "test",
		ThresholdValue: 0,
		Message:        "Test notification",
		MaxConnections: 20,
		Namespace:      "mynamespace", // example: set when running in K8s
	}

	// Build and print payload (before sending)
	payload, err := loki.PushPayload(ev)
	if err != nil {
		t.Fatalf("PushPayload: %v", err)
	}
	var payloadPretty map[string]interface{}
	_ = json.Unmarshal(payload, &payloadPretty)
	payloadJSON, _ := json.MarshalIndent(payloadPretty, "", "  ")
	t.Logf("--- Payload sent to Loki (POST %s) ---\n%s", pushURL, string(payloadJSON))

	if err := loki.Send(ctx, ev); err != nil {
		t.Fatalf("Loki.Send: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Query and print response
	resp, err := queryLokiRaw(ctx, pushURL, `{app="pgwd"}`)
	if err != nil {
		t.Fatalf("queryLokiRaw: %v", err)
	}
	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	t.Logf("--- Response from Loki (GET query_range?query={app=\"pgwd\"}) ---\n%s", string(respJSON))
}

// hasDeliveryCheckLine returns true if any line contains the expected delivery-check fields.
func hasDeliveryCheckLine(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, "pgwd:") &&
			strings.Contains(line, "total=5") &&
			strings.Contains(line, "active=2") &&
			strings.Contains(line, "idle=3") &&
			strings.Contains(line, "max_connections=20") &&
			strings.Contains(line, "delivery check") {
			return true
		}
	}
	return false
}

func getEnvSkip(t *testing.T, key, desc string) string {
	t.Helper()
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		t.Skipf("%s not set (e.g. %s=http://localhost:3100/loki/api/v1/push). Start Loki: docker compose -f testing/compose-loki.yaml up -d", key, key)
	}
	return v
}
