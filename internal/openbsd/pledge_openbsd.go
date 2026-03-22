//go:build openbsd

package openbsd

import (
	"log"

	"golang.org/x/sys/unix"
)

// ApplyPledge restricts process capabilities using OpenBSD pledge(2).
// Call after setupKube/setupKubeLoki (which need exec for kubectl).
// Promises: stdio (logging), rpath (read config file), inet (Postgres, Slack, Loki),
// dns (resolve hostnames for Slack webhook), proc (fork), exec (Go runtime may exec).
func ApplyPledge() {
	if err := unix.Pledge("stdio rpath inet dns proc exec", ""); err != nil {
		log.Printf("pgwd: pledge failed: %v", err)
		// Do not fatal; pgwd can still run, just less secure
	}
}
