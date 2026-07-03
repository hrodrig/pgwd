package notify

import (
	"fmt"
	"strings"
	"time"
)

// SummaryText returns a plain-text alert summary for Teams, generic webhook, and PagerDuty.
func SummaryText(ev Event, ts string) string {
	if ts == "" {
		ts = time.Now().Format("2006-01-02 15:04:05")
	}
	var b strings.Builder
	b.WriteString("pgwd – ")
	b.WriteString(summaryTitle(ev))
	b.WriteString("\n")
	b.WriteString(ev.Message)
	b.WriteString("\n")
	b.WriteString(plainConnLine(ev))
	b.WriteString("\n")
	if ev.Cluster != "" {
		b.WriteString(fmt.Sprintf("• Cluster: %s\n", ev.Cluster))
	}
	if ev.Database != "" {
		b.WriteString(fmt.Sprintf("• Database: %s\n", ev.Database))
	}
	if ev.Client != "" {
		b.WriteString(fmt.Sprintf("• Client: %s\n", ev.Client))
	}
	if ev.Namespace != "" {
		b.WriteString(fmt.Sprintf("• Namespace: %s\n", ev.Namespace))
	}
	b.WriteString(fmt.Sprintf("• Time: %s", ts))
	return b.String()
}

func summaryTitle(ev Event) string {
	switch ev.Threshold {
	case "test":
		return "Test notification"
	case "connect_failure":
		return "Connection failure"
	case "too_many_clients":
		return "URGENT: too many clients (DB saturated)"
	case "resolution":
		return "Resolved: connections returned to normal"
	default:
		if ev.Level != "" {
			switch ev.Level {
			case "attention":
				return "Attention"
			case "alert":
				return "Alert"
			case "danger":
				return "Danger"
			}
		}
		return "Threshold exceeded"
	}
}

func plainConnLine(ev Event) string {
	line := fmt.Sprintf("• Connections: total=%d active=%d idle=%d", ev.Stats.Total, ev.Stats.Active, ev.Stats.Idle)
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
	case "resolution":
		line += " (returned to normal)"
	default:
		line += fmt.Sprintf(" (limit %s=%d)", ev.Threshold, ev.ThresholdValue)
	}
	return line
}
