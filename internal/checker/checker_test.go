package checker

import (
	"errors"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/postgres"
)

var defaultLevels = []int{75, 85, 95}

func TestLevelFromPercent(t *testing.T) {
	tests := []struct {
		percent int
		levels  []int
		want    int
	}{
		{0, defaultLevels, 0},
		{50, defaultLevels, 0},
		{74, defaultLevels, 0},
		{75, defaultLevels, 1},
		{80, defaultLevels, 1},
		{84, defaultLevels, 1},
		{85, defaultLevels, 2},
		{90, defaultLevels, 2},
		{94, defaultLevels, 2},
		{95, defaultLevels, 3},
		{99, defaultLevels, 3},
		{100, defaultLevels, 3},
		{100, []int{50, 75, 90}, 3},
		{49, []int{50, 75, 90}, 0},
		{50, []int{50, 75, 90}, 1},
		{75, []int{50, 75, 90}, 2},
		{90, []int{50, 75, 90}, 3},
	}
	for _, tt := range tests {
		got := LevelFromPercent(tt.percent, tt.levels)
		if got != tt.want {
			t.Errorf("LevelFromPercent(%d, %v) = %d, want %d", tt.percent, tt.levels, got, tt.want)
		}
	}
}

func TestLevelToLabel(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{0, "attention"},
		{1, "attention"},
		{2, "alert"},
		{3, "danger"},
		{4, "danger"},
		{5, "danger"},
		{99, "attention"},
	}
	for _, tt := range tests {
		got := LevelToLabel(tt.level)
		if got != tt.want {
			t.Errorf("LevelToLabel(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestTitle(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{"", ""},
		{"a", "A"},
		{"hello", "Hello"},
		{"HELLO", "HELLO"},
	}
	for _, tt := range tests {
		got := Title(tt.s)
		if got != tt.want {
			t.Errorf("Title(%q) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestAllStringsEqual(t *testing.T) {
	tests := []struct {
		sl   []string
		v    string
		want bool
	}{
		{nil, "x", true},
		{[]string{}, "x", true},
		{[]string{"a"}, "a", true},
		{[]string{"a", "a", "a"}, "a", true},
		{[]string{"a", "b"}, "a", false},
		{[]string{"a", "a", "b"}, "a", false},
		{[]string{""}, "", true},
	}
	for _, tt := range tests {
		got := AllStringsEqual(tt.sl, tt.v)
		if got != tt.want {
			t.Errorf("AllStringsEqual(%v, %q) = %v, want %v", tt.sl, tt.v, got, tt.want)
		}
	}
}

func TestStateAndThresholdFromEvents(t *testing.T) {
	mkEv := func(threshold, level string) notify.Event {
		return notify.Event{Threshold: threshold, Level: level}
	}
	tests := []struct {
		name           string
		events         []notify.Event
		wantState      string
		wantThreshold  string
	}{
		{"empty", nil, "ok", ""},
		{"empty slice", []notify.Event{}, "ok", ""},
		{"single attention", []notify.Event{mkEv("total", "attention")}, "attention", "total"},
		{"connect_failure wins", []notify.Event{mkEv("total", "attention"), mkEv("connect_failure", "")}, "connect_failure", "connect_failure"},
		{"too_many_clients wins", []notify.Event{mkEv("too_many_clients", "")}, "connect_failure", "too_many_clients"},
		{"danger wins", []notify.Event{mkEv("total", "alert"), mkEv("active", "danger")}, "danger", "active"},
		{"alert over attention", []notify.Event{mkEv("total", "attention"), mkEv("active", "alert")}, "alert", "active"},
		{"attention default", []notify.Event{mkEv("idle", "")}, "attention", "idle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, threshold := StateAndThresholdFromEvents(tt.events)
			if state != tt.wantState || threshold != tt.wantThreshold {
				t.Errorf("StateAndThresholdFromEvents() = (%q, %q), want (%q, %q)",
					state, threshold, tt.wantState, tt.wantThreshold)
			}
		})
	}
}

func TestApplySingleThresholdDefaults(t *testing.T) {
	tests := []struct {
		name       string
		percent    int
		maxConn    int
		initTotal  int
		initActive int
		wantTotal  int
		wantActive int
	}{
		{"80% of 100", 80, 100, 0, 0, 80, 80},
		{"50% of 200", 50, 200, 0, 0, 100, 100},
		{"percent clamped to 1", 0, 100, 0, 0, 1, 1},
		{"percent clamped to 100", 150, 100, 0, 0, 100, 100},
		{"partial: total set", 80, 100, 50, 0, 50, 80},
		{"partial: active set", 80, 100, 0, 60, 80, 60},
		{"both set: no change", 80, 100, 90, 70, 90, 70},
		{"maxConn 1 → min 1", 80, 1, 0, 0, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				DefaultThresholdPercent: tt.percent,
				ThresholdTotal:          tt.initTotal,
				ThresholdActive:        tt.initActive,
			}
			ApplySingleThresholdDefaults(cfg, tt.maxConn)
			if cfg.ThresholdTotal != tt.wantTotal || cfg.ThresholdActive != tt.wantActive {
				t.Errorf("ApplySingleThresholdDefaults: got total=%d active=%d, want total=%d active=%d",
					cfg.ThresholdTotal, cfg.ThresholdActive, tt.wantTotal, tt.wantActive)
			}
		})
	}
}

func TestValidateThresholdConfig(t *testing.T) {
	errServer := errors.New("server error")
	tests := []struct {
		name      string
		cfg       *config.Config
		maxConn   int
		maxConnErr error
		wantErr   bool
	}{
		{"dry-run: no error", &config.Config{DryRun: true}, 0, nil, false},
		{"force-notification: no error", &config.Config{ForceNotification: true}, 0, nil, false},
		{"has threshold total: no error", &config.Config{ThresholdTotal: 10}, 100, nil, false},
		{"has threshold active: no error", &config.Config{ThresholdActive: 5}, 100, nil, false},
		{"level mode + maxConn 0 + no server err: error", &config.Config{ThresholdLevels: "75,85,95"}, 0, nil, true},
		{"level mode + maxConn 0 + server err: error", &config.Config{ThresholdLevels: "75,85,95"}, 0, errServer, true},
		{"no thresholds + maxConn 0: error", &config.Config{}, 0, nil, true},
		{"no thresholds + maxConnErr: error", &config.Config{}, 100, errServer, true},
		{"no thresholds + maxConn ok: error (unreachable in practice)", &config.Config{}, 100, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateThresholdConfig(tt.cfg, tt.maxConn, tt.maxConnErr)
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Errorf("ValidateThresholdConfig: err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestBaseEvent(t *testing.T) {
	stats := postgres.ConnectionStats{Total: 50, Active: 10, Idle: 40}
	ev := BaseEvent(stats, 100, true, "cl", "client", "ns", "db")
	if ev.Stats.Total != 50 || ev.MaxConnections != 100 || !ev.MaxConnectionsIsOverride {
		t.Errorf("BaseEvent: unexpected event %+v", ev)
	}
	if ev.Cluster != "cl" || ev.Client != "client" || ev.Namespace != "ns" || ev.Database != "db" {
		t.Errorf("BaseEvent: labels wrong: cluster=%q client=%q ns=%q db=%q", ev.Cluster, ev.Client, ev.Namespace, ev.Database)
	}
}

func TestCollectLevelModeEvent(t *testing.T) {
	levels := config.ParseThresholdLevels("75,85,95")
	if len(levels) < 3 {
		t.Fatal("ParseThresholdLevels failed")
	}
	cfg := &config.Config{ThresholdLevels: "75,85,95"}
	base := notify.Event{Cluster: "c", Client: "x", Namespace: "n", Database: "d"}

	tests := []struct {
		name      string
		stats     postgres.ConnectionStats
		maxConn   int
		wantNil   bool
		wantLevel string
	}{
		{"below 75%: nil", postgres.ConnectionStats{50, 10, 40}, 100, true, ""},
		{"75% total: attention", postgres.ConnectionStats{75, 10, 65}, 100, false, "attention"},
		{"85% total: alert", postgres.ConnectionStats{85, 10, 75}, 100, false, "alert"},
		{"95% total: danger", postgres.ConnectionStats{95, 10, 85}, 100, false, "danger"},
		{"active 90% > total 80%: active wins", postgres.ConnectionStats{80, 90, 0}, 100, false, "alert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := CollectLevelModeEvent(base, cfg, tt.stats, tt.maxConn)
			if tt.wantNil {
				if ev != nil {
					t.Errorf("CollectLevelModeEvent: want nil, got %+v", ev)
				}
				return
			}
			if ev == nil {
				t.Fatal("CollectLevelModeEvent: want non-nil")
			}
			if ev.Level != tt.wantLevel {
				t.Errorf("CollectLevelModeEvent: Level = %q, want %q", ev.Level, tt.wantLevel)
			}
		})
	}
}

func TestCollectExplicitThresholdEvents(t *testing.T) {
	cfg := &config.Config{ThresholdTotal: 80, ThresholdActive: 50}
	base := notify.Event{Cluster: "c", Client: "x"}

	tests := []struct {
		name        string
		stats       postgres.ConnectionStats
		maxConn     int
		wantCount   int
		wantTotal   bool
		wantActive  bool
	}{
		{"below both", postgres.ConnectionStats{50, 30, 20}, 100, 0, false, false},
		{"total exceeded", postgres.ConnectionStats{90, 30, 60}, 100, 1, true, false},
		{"active exceeded", postgres.ConnectionStats{50, 60, 0}, 100, 1, false, true},
		{"both exceeded", postgres.ConnectionStats{90, 60, 30}, 100, 2, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := CollectExplicitThresholdEvents(base, cfg, tt.stats, tt.maxConn)
			if len(events) != tt.wantCount {
				t.Errorf("CollectExplicitThresholdEvents: got %d events, want %d", len(events), tt.wantCount)
			}
			hasTotal, hasActive := false, false
			for _, e := range events {
				if e.Threshold == "total" {
					hasTotal = true
				}
				if e.Threshold == "active" {
					hasActive = true
				}
			}
			if hasTotal != tt.wantTotal || hasActive != tt.wantActive {
				t.Errorf("CollectExplicitThresholdEvents: total=%v active=%v, want total=%v active=%v",
					hasTotal, hasActive, tt.wantTotal, tt.wantActive)
			}
		})
	}
}
