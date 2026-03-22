package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath is the standard config file location.
const DefaultConfigPath = "/etc/pgwd/pgwd.conf"

// fileConfig mirrors the YAML structure: db, kube, notifications, and top-level keys.
type fileConfig struct {
	Client                 string `yaml:"client"`
	DryRun                 bool   `yaml:"dry_run"`
	Interval               int    `yaml:"interval"`
	NotifyOnConnectFailure bool   `yaml:"notify_on_connect_failure"`
	DB                     struct {
		URL                     string `yaml:"url"`
		StaleAge                int    `yaml:"stale_age"`
		DefaultThresholdPercent int    `yaml:"default_threshold_percent"`
		Threshold               struct {
			Active int    `yaml:"active"`
			Idle   int    `yaml:"idle"`
			Levels string `yaml:"levels"`
			Stale  int    `yaml:"stale"`
			Total  int    `yaml:"total"`
		} `yaml:"threshold"`
	} `yaml:"db"`
	Kube struct {
		Context           string `yaml:"context"`
		LocalPort         int    `yaml:"local_port"`
		Loki              string `yaml:"loki"`
		LokiLocalPort     int    `yaml:"loki_local_port"`
		LokiRemotePort    int    `yaml:"loki_remote_port"`
		PasswordContainer string `yaml:"password_container"`
		PasswordVar       string `yaml:"password_var"`
		Postgres          string `yaml:"postgres"`
	} `yaml:"kube"`
	Notifications struct {
		Loki struct {
			URL         string `yaml:"url"`
			BearerToken string `yaml:"bearer_token"`
			Labels      string `yaml:"labels"`
			OrgID       string `yaml:"org_id"`
		} `yaml:"loki"`
		Slack struct {
			Webhook string `yaml:"webhook"`
		} `yaml:"slack"`
	} `yaml:"notifications"`
}

// FromFile loads config from a YAML file. Returns (Config, loaded, error).
// When path is empty or file does not exist: returns empty Config, loaded=false, nil.
// When file exists and parses: returns Config, loaded=true, nil. When loaded=true,
// env vars (PGWD_*) are not applied; config file is the single source. Use -config
// to specify a custom path.
func FromFile(path string) (Config, bool, error) {
	if path == "" {
		return Config{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return Config{}, false, err
	}
	return fileConfigToConfig(fc), true, nil
}

func fileConfigToConfig(fc fileConfig) Config {
	c := Config{
		DBURL:                   fc.DB.URL,
		Client:                  fc.Client,
		DefaultThresholdPercent: fc.DB.DefaultThresholdPercent,
		DryRun:                  fc.DryRun,
		Interval:                fc.Interval,
		KubePostgres:            fc.Kube.Postgres,
		KubeContext:             fc.Kube.Context,
		KubeLocalPort:           fc.Kube.LocalPort,
		KubeLoki:                fc.Kube.Loki,
		KubeLokiLocalPort:       fc.Kube.LokiLocalPort,
		KubeLokiRemotePort:      fc.Kube.LokiRemotePort,
		KubePasswordContainer:   fc.Kube.PasswordContainer,
		KubePasswordVar:         fc.Kube.PasswordVar,
		LokiURL:                 fc.Notifications.Loki.URL,
		LokiLabels:              fc.Notifications.Loki.Labels,
		LokiOrgID:               fc.Notifications.Loki.OrgID,
		LokiBearerToken:         fc.Notifications.Loki.BearerToken,
		NotifyOnConnectFailure:  fc.NotifyOnConnectFailure,
		SlackWebhook:            fc.Notifications.Slack.Webhook,
		StaleAge:                fc.DB.StaleAge,
		ThresholdTotal:          fc.DB.Threshold.Total,
		ThresholdActive:         fc.DB.Threshold.Active,
		ThresholdIdle:           fc.DB.Threshold.Idle,
		ThresholdStale:          fc.DB.Threshold.Stale,
		ThresholdLevels:         fc.DB.Threshold.Levels,
	}
	ApplyDefaults(&c)
	return c
}

// ApplyDefaults sets default values for fields that are zero. Call after FromFile when no file exists.
func ApplyDefaults(c *Config) {
	if c.KubePasswordVar == "" {
		c.KubePasswordVar = "POSTGRES_PASSWORD"
	}
	if c.KubeLocalPort == 0 {
		c.KubeLocalPort = 5432
	}
	if c.KubeLokiLocalPort == 0 {
		c.KubeLokiLocalPort = 3100
	}
	if c.KubeLokiRemotePort == 0 {
		c.KubeLokiRemotePort = 3100
	}
	if c.DefaultThresholdPercent == 0 {
		c.DefaultThresholdPercent = 80
	}
	if c.ThresholdLevels == "" {
		c.ThresholdLevels = DefaultThresholdLevels
	}
}
