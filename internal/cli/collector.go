package cli

import (
	"log"

	"github.com/hrodrig/pgwd/internal/collector"
	"github.com/hrodrig/pgwd/internal/config"
)

const collectorHelpURL = "https://github.com/hrodrig/pgwd/#anonymous-usage"

// startCollector runs opt-in telemetry and/or opt-out update check once at daemon startup.
func startCollector(cfg *config.Config) {
	if cfg == nil || cfg.Interval <= 0 {
		return
	}

	info := collector.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}
	features := collector.FeaturesFromConfig(cfg)
	debug := cfg.LogLevel == "debug"

	if cfg.EnableCollector && cfg.EnableUpdateCheck {
		log.Printf("pgwd: anonymous metric collection enabled — POST %s (once per daemon start) — see %s",
			collector.IngestURL, collectorHelpURL)
		log.Printf("pgwd: update check enabled — GET %s", collector.UpdateCheckURL())
		log.Printf("pgwd: thank you for supporting pgwd")
		go collector.CollectWithUpdate(info, features, debug)
		return
	}
	if cfg.EnableCollector {
		log.Printf("pgwd: anonymous metric collection enabled — POST %s (once per daemon start) — see %s — thank you for supporting pgwd",
			collector.IngestURL, collectorHelpURL)
		go collector.Collect(info, features, debug)
		return
	}
	if cfg.EnableUpdateCheck {
		log.Printf("pgwd: update check enabled — GET %s", collector.UpdateCheckURL())
		log.Printf("pgwd: anonymous metric collection disabled — set PGWD_ENABLE_COLLECTOR=true to POST %s — see %s",
			collector.IngestURL, collectorHelpURL)
		go collector.CheckUpdate(info, debug)
		return
	}
	log.Printf("pgwd: anonymous metric collection disabled — set PGWD_ENABLE_COLLECTOR=true to POST %s — see %s",
		collector.IngestURL, collectorHelpURL)
}
