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
}

// Sender can send an event to a destination (Slack, Loki).
type Sender interface {
	Send(ctx context.Context, ev Event) error
}
