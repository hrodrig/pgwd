package config

import (
	"encoding/json"
	"os"
	"time"
)

func applyEnvSlackLoki(cfg *Config) {
	if v := env("NOTIFICATIONS_SLACK_WEBHOOK", ""); v != "" {
		cfg.SlackWebhook = v
	}
	if v := env("NOTIFICATIONS_LOKI_URL", ""); v != "" {
		cfg.LokiURL = v
	}
	if v := env("NOTIFICATIONS_LOKI_LABELS", ""); v != "" {
		cfg.LokiLabels = v
	}
	if v := env("NOTIFICATIONS_LOKI_ORG_ID", ""); v != "" {
		cfg.LokiOrgID = v
	}
	if v := env("NOTIFICATIONS_LOKI_BEARER_TOKEN", ""); v != "" {
		cfg.LokiBearerToken = v
	}
}

func applyEnvPagerDuty(cfg *Config) {
	if _, ok := os.LookupEnv("PGWD_NOTIFICATIONS_PAGERDUTY_ENABLED"); ok {
		cfg.PagerDutyEnabled = envBool("NOTIFICATIONS_PAGERDUTY_ENABLED", false)
	}
	if v := env("NOTIFICATIONS_PAGERDUTY_ROUTING_KEY", ""); v != "" {
		cfg.PagerDutyRoutingKey = v
		cfg.PagerDutyEnabled = true
	}
	if v := env("NOTIFICATIONS_PAGERDUTY_SEVERITY", ""); v != "" {
		cfg.PagerDutySeverity = v
	}
	if v := env("NOTIFICATIONS_PAGERDUTY_SOURCE", ""); v != "" {
		cfg.PagerDutySource = v
	}
}

func applyEnvTeams(cfg *Config) {
	if _, ok := os.LookupEnv("PGWD_NOTIFICATIONS_TEAMS_ENABLED"); ok {
		cfg.TeamsEnabled = envBool("NOTIFICATIONS_TEAMS_ENABLED", false)
	}
	if v := env("NOTIFICATIONS_TEAMS_WEBHOOK", ""); v != "" {
		cfg.TeamsWebhook = v
		cfg.TeamsEnabled = true
	}
}

func applyEnvGenericWebhook(cfg *Config) {
	if _, ok := os.LookupEnv("PGWD_NOTIFICATIONS_GENERIC_ENABLED"); ok {
		cfg.GenericEnabled = envBool("NOTIFICATIONS_GENERIC_ENABLED", false)
	}
	if v := env("NOTIFICATIONS_GENERIC_WEBHOOK_URL", ""); v != "" {
		cfg.GenericWebhookURL = v
		cfg.GenericEnabled = true
	}
	if v := env("NOTIFICATIONS_GENERIC_JSON_KEY", ""); v != "" {
		cfg.GenericJSONKey = v
	}
	if v := env("NOTIFICATIONS_GENERIC_BODY_TEMPLATE", ""); v != "" {
		cfg.GenericBodyTemplate = v
	}
	if v := env("NOTIFICATIONS_GENERIC_HMAC_SECRET", ""); v != "" {
		cfg.GenericHMACSecret = v
	}
	if v := env("NOTIFICATIONS_GENERIC_HMAC_HEADER", ""); v != "" {
		cfg.GenericHMACHeader = v
	}
	if m := envJSONMap("NOTIFICATIONS_GENERIC_HEADERS"); m != nil {
		cfg.GenericHeaders = m
	}
	if m := envJSONMap("NOTIFICATIONS_GENERIC_EXTRA_FIELDS"); m != nil {
		cfg.GenericExtraFields = m
	}
}

func applyEnvNotifyRetry(cfg *Config) {
	if v := envInt("NOTIFICATIONS_RETRY_MAX_ATTEMPTS", -1); v >= 0 {
		cfg.RetryMaxAttempts = v
	}
	if v, ok := os.LookupEnv("PGWD_NOTIFICATIONS_RETRY_INITIAL_BACKOFF"); ok && v != "" {
		cfg.RetryInitialBackoff = envDuration("NOTIFICATIONS_RETRY_INITIAL_BACKOFF", 0)
	}
	if v, ok := os.LookupEnv("PGWD_NOTIFICATIONS_RETRY_MAX_BACKOFF"); ok && v != "" {
		cfg.RetryMaxBackoff = envDuration("NOTIFICATIONS_RETRY_MAX_BACKOFF", 0)
	}
}

func applyNotifyDefaults(c *Config) {
	if c.PagerDutyRoutingKey != "" {
		c.PagerDutyEnabled = true
	}
	if c.PagerDutySeverity == "" {
		c.PagerDutySeverity = "warning"
	}
	if c.PagerDutySource == "" {
		c.PagerDutySource = "pgwd"
	}
	if c.TeamsWebhook != "" {
		c.TeamsEnabled = true
	}
	if c.GenericWebhookURL != "" {
		c.GenericEnabled = true
	}
	if c.GenericJSONKey == "" {
		c.GenericJSONKey = "text"
	}
	if c.GenericHMACHeader == "" {
		c.GenericHMACHeader = "X-Pgwd-Signature"
	}
	if c.RetryMaxAttempts <= 0 {
		c.RetryMaxAttempts = 3
	}
	if c.RetryInitialBackoff <= 0 {
		c.RetryInitialBackoff = time.Second
	}
	if c.RetryMaxBackoff <= 0 {
		c.RetryMaxBackoff = 10 * time.Second
	}
}

// MergeNotifyJSONFields parses CLI/env JSON overrides into GenericHeaders and GenericExtraFields.
func MergeNotifyJSONFields(c *Config) {
	if c.GenericHeadersJSON != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(c.GenericHeadersJSON), &m); err == nil {
			if c.GenericHeaders == nil {
				c.GenericHeaders = m
			} else {
				for k, v := range m {
					c.GenericHeaders[k] = v
				}
			}
		}
	}
	if c.GenericExtraFieldsJSON != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(c.GenericExtraFieldsJSON), &m); err == nil {
			if c.GenericExtraFields == nil {
				c.GenericExtraFields = m
			} else {
				for k, v := range m {
					c.GenericExtraFields[k] = v
				}
			}
		}
	}
}
