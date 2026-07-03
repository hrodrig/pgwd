package config

import (
	"testing"
	"time"
)

func TestApplyNotifyDefaults(t *testing.T) {
	c := Config{
		PagerDutyRoutingKey: "rk",
		TeamsWebhook:        "https://teams.example/hook",
		GenericWebhookURL:   "https://api.example/hook",
	}
	applyNotifyDefaults(&c)

	if !c.PagerDutyEnabled || c.PagerDutySeverity != "warning" || c.PagerDutySource != "pgwd" {
		t.Errorf("PagerDuty defaults: enabled=%v severity=%q source=%q", c.PagerDutyEnabled, c.PagerDutySeverity, c.PagerDutySource)
	}
	if !c.TeamsEnabled {
		t.Errorf("TeamsEnabled: got false")
	}
	if !c.GenericEnabled || c.GenericJSONKey != "text" || c.GenericHMACHeader != "X-Pgwd-Signature" {
		t.Errorf("Generic defaults: enabled=%v json_key=%q hmac_header=%q", c.GenericEnabled, c.GenericJSONKey, c.GenericHMACHeader)
	}
	if c.RetryMaxAttempts != 3 || c.RetryInitialBackoff != time.Second || c.RetryMaxBackoff != 10*time.Second {
		t.Errorf("Retry defaults: attempts=%d initial=%v max=%v", c.RetryMaxAttempts, c.RetryInitialBackoff, c.RetryMaxBackoff)
	}
}

func TestMergeNotifyJSONFields(t *testing.T) {
	c := Config{
		GenericHeaders:         map[string]string{"X-Existing": "1"},
		GenericHeadersJSON:     `{"Authorization":"Bearer tok"}`,
		GenericExtraFieldsJSON: `{"env":"prod"}`,
	}
	MergeNotifyJSONFields(&c)

	if c.GenericHeaders["X-Existing"] != "1" || c.GenericHeaders["Authorization"] != "Bearer tok" {
		t.Errorf("GenericHeaders: got %v", c.GenericHeaders)
	}
	if c.GenericExtraFields["env"] != "prod" {
		t.Errorf("GenericExtraFields: got %v", c.GenericExtraFields)
	}
}
