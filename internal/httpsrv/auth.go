package httpsrv

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
)

func metricsAuthConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.HTTPMetricsToken) != "" {
		return true
	}
	return strings.TrimSpace(cfg.HTTPMetricsBasicUser) != "" && cfg.HTTPMetricsBasicPassword != ""
}

func metricsAuthorized(r *http.Request, cfg *config.Config) bool {
	if !metricsAuthConfigured(cfg) {
		return true
	}
	token := strings.TrimSpace(cfg.HTTPMetricsToken)
	if token != "" && metricsTokenOK(r, token) {
		return true
	}
	user := strings.TrimSpace(cfg.HTTPMetricsBasicUser)
	if user != "" && cfg.HTTPMetricsBasicPassword != "" && metricsBasicOK(r, user, cfg.HTTPMetricsBasicPassword) {
		return true
	}
	return false
}

func metricsTokenOK(r *http.Request, want string) bool {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		got := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return subtle.ConstantTimeCompare([]byte(q), []byte(want)) == 1
	}
	return false
}

func metricsBasicOK(r *http.Request, wantUser, wantPass string) bool {
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(u), []byte(wantUser)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(wantPass)) == 1
}

func writeMetricsUnauthorized(w http.ResponseWriter, cfg *config.Config) {
	var parts []string
	if strings.TrimSpace(cfg.HTTPMetricsToken) != "" {
		parts = append(parts, `Bearer realm="pgwd metrics"`)
	}
	if strings.TrimSpace(cfg.HTTPMetricsBasicUser) != "" && cfg.HTTPMetricsBasicPassword != "" {
		parts = append(parts, `Basic realm="pgwd metrics"`)
	}
	if len(parts) > 0 {
		w.Header().Set("WWW-Authenticate", strings.Join(parts, ", "))
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
