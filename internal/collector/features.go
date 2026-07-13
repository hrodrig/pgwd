package collector

import (
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/metricsstore"
)

// FeaturesFromConfig derives boolean feature flags from cfg without exposing secrets.
func FeaturesFromConfig(cfg *config.Config) ConfigFeatures {
	if cfg == nil {
		return ConfigFeatures{}
	}
	driver := metricsstore.Driver(cfg)
	return ConfigFeatures{
		MultiDB:            cfg.UsesDatabases(),
		UsesLevelMode:      cfg.UsesLevelMode(),
		LongQueryEnabled:   cfg.LongQueryMinSeconds > 0,
		HasSlack:           cfg.SlackWebhook != "",
		HasLoki:            cfg.LokiURL != "" || cfg.KubeLoki != "",
		HasKubePostgres:    cfg.KubePostgres != "",
		HasKubeLoki:        cfg.KubeLoki != "",
		HasSQLiteStore:     driver == metricsstore.DriverSQLite,
		HasSQLMetricsStore: driver == metricsstore.DriverPostgres || driver == metricsstore.DriverMySQL,
		HasHTTPListen:      strings.TrimSpace(cfg.HTTPListen) != "",
		ConfirmAlertGT1:    cfg.ConfirmAlert > 1,
		ConfirmOkGT1:       cfg.ConfirmOk > 1,
		DryRun:             cfg.DryRun,
	}
}
