//go:build openbsd

package openbsd

import (
	"log"

	"golang.org/x/sys/unix"
)

// ApplyPledge restricts process capabilities using OpenBSD pledge(2).
// Call after setupKube/setupKubeLoki (which need exec for kubectl).
// Promises: stdio (logging), inet (Postgres, Slack, Loki).
func ApplyPledge() {
	if err := unix.Pledge("stdio inet", ""); err != nil {
		log.Printf("pgwd: pledge failed: %v", err)
		// Do not fatal; pgwd can still run, just less secure
	}
}
