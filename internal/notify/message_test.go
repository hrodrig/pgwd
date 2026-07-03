package notify

import (
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/postgres"
)

func TestSummaryText_allTitles(t *testing.T) {
	cases := []struct {
		threshold string
		level     string
		want      string
	}{
		{"test", "", "Test notification"},
		{"connect_failure", "", "Connection failure"},
		{"too_many_clients", "", "too many clients"},
		{"resolution", "", "Resolved"},
		{"total", "attention", "Attention"},
		{"total", "alert", "Alert"},
		{"total", "danger", "Danger"},
		{"idle", "", "Threshold exceeded"},
	}
	for _, tc := range cases {
		ev := Event{
			Threshold: tc.threshold,
			Level:     tc.level,
			Message:   "msg",
			Stats:     postgres.ConnectionStats{Total: 1, Active: 0, Idle: 1},
		}
		got := SummaryText(ev, "2025-01-01 00:00:00")
		if !strings.Contains(got, tc.want) {
			t.Errorf("threshold=%q level=%q: got %q want substring %q", tc.threshold, tc.level, got, tc.want)
		}
	}
}

func TestSummaryText_emptyTimestampUsesNow(t *testing.T) {
	ev := Event{Threshold: "test", Message: "m", Stats: postgres.ConnectionStats{}}
	got := SummaryText(ev, "")
	if got == "" || !strings.Contains(got, "Time:") {
		t.Fatalf("got %q", got)
	}
}
