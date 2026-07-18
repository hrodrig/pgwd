// Package checker provides pure logic for threshold levels, event collection,
// and state derivation used by cmd/pgwd. Extracted for testability.
package checker

import (
	"fmt"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/notify"
	"github.com/hrodrig/pgwd/internal/postgres"
)

// LevelFromPercent returns 1, 2, or 3 when percent >= levels[0], levels[1], levels[2]; 0 otherwise.
func LevelFromPercent(percent int, levels []int) int {
	for i := len(levels) - 1; i >= 0; i-- {
		if percent >= levels[i] {
			return i + 1
		}
	}
	return 0
}

// LevelToLabel maps level (1–5) to severity: attention, alert, danger.
func LevelToLabel(level int) string {
	switch level {
	case 1:
		return "attention"
	case 2:
		return "alert"
	case 3, 4, 5:
		return "danger"
	default:
		return "attention"
	}
}

// Title returns s with first character uppercased.
func Title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// AllStringsEqual reports whether all elements of sl equal v.
func AllStringsEqual(sl []string, v string) bool {
	for _, s := range sl {
		if s != v {
			return false
		}
	}
	return true
}

// StateAndThresholdFromEvents derives state (ok, attention, alert, danger, connect_failure) and threshold from events.
func StateAndThresholdFromEvents(events []notify.Event) (state, threshold string) {
	if len(events) == 0 {
		return "ok", ""
	}
	state, threshold = "attention", events[0].Threshold
	for _, e := range events {
		if e.Threshold == "connect_failure" || e.Threshold == "too_many_clients" {
			return "connect_failure", e.Threshold
		}
		if e.Level == "danger" {
			return "danger", e.Threshold
		}
		if e.Level == "alert" {
			state, threshold = "alert", e.Threshold
		}
	}
	return state, threshold
}

// ValidateThresholdConfig returns an error when thresholds cannot be resolved (level mode, no thresholds, maxConn issues).
func ValidateThresholdConfig(cfg *config.Config, maxConn int, maxConnErr error) error {
	if cfg.UsesLevelMode() && maxConn == 0 {
		if maxConnErr != nil {
			return fmt.Errorf("threshold-levels mode requires max_connections; could not read from server: %w", maxConnErr)
		}
		return fmt.Errorf("threshold-levels mode requires max_connections; server returned 0")
	}
	if cfg.HasAnyThreshold() || cfg.DryRun || cfg.ForceNotification {
		return nil
	}
	if maxConnErr != nil {
		return fmt.Errorf("no thresholds set and could not default from server. Set -db-threshold-levels, -db-threshold-idle, or -db-threshold-stale with -db-stale-age, or use -dry-run or -force-notification: %w", maxConnErr)
	}
	if maxConn == 0 {
		return fmt.Errorf("no thresholds set and could not default from server (server returned max_connections=0). Set -db-threshold-levels, -db-threshold-idle, or -db-threshold-stale with -db-stale-age, or use -dry-run or -force-notification")
	}
	return fmt.Errorf("no thresholds set. Set -db-threshold-levels, -db-threshold-idle, or -db-threshold-stale with -db-stale-age, or use -dry-run or -force-notification")
}

// BaseEvent builds a notify.Event from stats and context labels.
func BaseEvent(stats postgres.ConnectionStats, maxConn int, override bool, cluster, client, ns, db string) notify.Event {
	return notify.Event{
		Stats:                    stats,
		MaxConnections:           maxConn,
		MaxConnectionsIsOverride: override,
		Cluster:                  cluster,
		Client:                   client,
		Namespace:                ns,
		Database:                 db,
	}
}

// CollectLevelModeEvent produces one event when total or active percent crosses any level in ThresholdLevels.
func CollectLevelModeEvent(ev notify.Event, cfg *config.Config, stats postgres.ConnectionStats, maxConn int) *notify.Event {
	levels := config.ParseThresholdLevels(cfg.ThresholdLevels)
	if len(levels) < 3 {
		return nil
	}
	totalPercent := stats.Total * 100 / maxConn
	activePercent := stats.Active * 100 / maxConn
	totalLevel := LevelFromPercent(totalPercent, levels)
	activeLevel := LevelFromPercent(activePercent, levels)
	highestLevel := totalLevel
	threshold := "total"
	thresholdValue := 0
	if activeLevel > totalLevel {
		highestLevel = activeLevel
		threshold = "active"
		thresholdValue = (maxConn * levels[activeLevel-1]) / 100
	} else if totalLevel > 0 {
		thresholdValue = (maxConn * levels[totalLevel-1]) / 100
	}
	if highestLevel == 0 {
		return nil
	}
	val := stats.Total
	if threshold == "active" {
		val = stats.Active
	}
	e := ev
	e.Threshold = threshold
	e.ThresholdValue = thresholdValue
	e.Level = LevelToLabel(highestLevel)
	e.Message = fmt.Sprintf("%s connections %d >= %d (%d%% of max) — %s", Title(threshold), val, thresholdValue, levels[highestLevel-1], e.Level)
	return &e
}
