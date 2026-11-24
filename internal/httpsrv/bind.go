package httpsrv

import (
	"net"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
)

// IsLoopbackListen reports whether addr binds only to loopback (127.0.0.1, ::1, localhost).
// Empty host (":8080") means all interfaces and returns false.
func IsLoopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Allow host-only or bare port forms used in tests.
		if strings.HasPrefix(addr, ":") {
			return false
		}
		host = addr
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// WarnInsecureMetricsBind is true when /metrics is anonymous on a non-loopback listen address.
func WarnInsecureMetricsBind(cfg *config.Config) bool {
	if cfg == nil || cfg.HTTPListen == "" {
		return false
	}
	if metricsAuthConfigured(cfg) {
		return false
	}
	return !IsLoopbackListen(cfg.HTTPListen)
}
