package notify

import (
	"context"

	"github.com/hrodrig/pgwd/internal/postgres"
)

// Event is sent to Slack and/or Loki when a threshold is exceeded.
type Event struct {
	Stats          postgres.ConnectionStats
	Threshold      string // e.g. "total", "active", "idle"
	ThresholdValue int
	Message        string
	// Optional context for Slack (health-check style): cluster, client (host/service/pod), namespace, database.
	Cluster   string
	Client    string
	Namespace string
	Database  string // database name from connection URL (e.g. for non-Kube runs)
}

// Sender can send an event to a destination (Slack, Loki).
type Sender interface {
	Send(ctx context.Context, ev Event) error
}
