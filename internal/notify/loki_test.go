package notify

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hrodrig/pgwd/internal/postgres"
)

func TestLoki_PushPayload_includes_database_and_cluster_labels(t *testing.T) {
	loki := &Loki{URL: "http://localhost:3100/loki/api/v1/push", Labels: map[string]string{"app": "pgwd"}}
	ev := Event{
		Stats:          postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold:      "test",
		Message:        "Test notification",
		MaxConnections: 20,
		Database:       "myapp",
		Cluster:        "prod",
	}
	raw, err := loki.PushPayload(ev)
	if err != nil {
		t.Fatalf("PushPayload: %v", err)
	}
	var body lokiPushBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(body.Streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(body.Streams))
	}
	labels := body.Streams[0].Stream
	if labels["database"] != "myapp" {
		t.Errorf("labels[%q] = %q, want myapp", "database", labels["database"])
	}
	if labels["cluster"] != "prod" {
		t.Errorf("labels[%q] = %q, want prod", "cluster", labels["cluster"])
	}
}

func TestLoki_PushPayload_omits_empty_database_and_cluster(t *testing.T) {
	loki := &Loki{URL: "http://localhost:3100/loki/api/v1/push", Labels: map[string]string{"app": "pgwd"}}
	ev := Event{
		Stats:          postgres.ConnectionStats{Total: 5, Active: 2, Idle: 3},
		Threshold:      "test",
		Message:        "Test notification",
		MaxConnections: 20,
		// Database and Cluster empty
	}
	raw, err := loki.PushPayload(ev)
	if err != nil {
		t.Fatalf("PushPayload: %v", err)
	}
	var body lokiPushBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	labels := body.Streams[0].Stream
	if _, ok := labels["database"]; ok {
		t.Errorf("database label should be omitted when empty, got %q", labels["database"])
	}
	if _, ok := labels["cluster"]; ok {
		t.Errorf("cluster label should be omitted when empty, got %q", labels["cluster"])
	}
}

func TestParseLokiLabels(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"single", "job=pgwd", map[string]string{"job": "pgwd"}},
		{"two", "job=pgwd,env=prod", map[string]string{"job": "pgwd", "env": "prod"}},
		{"spaces", " job = pgwd , env = prod ", map[string]string{"job": "pgwd", "env": "prod"}},
		{"empty value", "key=", map[string]string{"key": ""}},
		{"value with equals", "k=a=b", map[string]string{"k": "a=b"}},
		{"no equals", "justkey", map[string]string{}},
		{"comma only", ",,,", map[string]string{}},
		{"many", "a=1,b=2,c=3", map[string]string{"a": "1", "b": "2", "c": "3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLokiLabels(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLokiLabels(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
