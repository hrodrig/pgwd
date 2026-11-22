package httpsrv

import (
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
)

func TestIsLoopbackListen(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{":8080", false},
		{"0.0.0.0:8080", false},
		{"127.0.0.1:8080", true},
		{"[::1]:8080", true},
		{"localhost:9090", true},
	}
	for _, tt := range tests {
		if got := IsLoopbackListen(tt.addr); got != tt.want {
			t.Errorf("IsLoopbackListen(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestWarnInsecureMetricsBind(t *testing.T) {
	if !WarnInsecureMetricsBind(&config.Config{HTTPListen: ":8080"}) {
		t.Fatal("want warn for :8080 without auth")
	}
	if WarnInsecureMetricsBind(&config.Config{HTTPListen: "127.0.0.1:8080"}) {
		t.Fatal("no warn for loopback")
	}
	if WarnInsecureMetricsBind(&config.Config{HTTPListen: ":8080", HTTPMetricsToken: "t"}) {
		t.Fatal("no warn when token set")
	}
}
