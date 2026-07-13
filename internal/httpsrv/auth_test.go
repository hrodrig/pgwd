package httpsrv

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

func TestMetricsAuthorized(t *testing.T) {
	cfg := &config.Config{HTTPMetricsToken: "secret-token"}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	if !metricsAuthorized(req, cfg) {
		t.Fatal("want bearer authorized")
	}
	req = httptest.NewRequest(http.MethodGet, "/metrics?token=secret-token", nil)
	if !metricsAuthorized(req, cfg) {
		t.Fatal("want query token authorized")
	}
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	if metricsAuthorized(req, cfg) {
		t.Fatal("want unauthorized for wrong bearer")
	}
}

func TestMetricsAuthorized_Basic(t *testing.T) {
	cfg := &config.Config{HTTPMetricsBasicUser: "scraper", HTTPMetricsBasicPassword: "pass"}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("scraper:pass")))
	if !metricsAuthorized(req, cfg) {
		t.Fatal("want basic authorized")
	}
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.SetBasicAuth("scraper", "wrong")
	if metricsAuthorized(req, cfg) {
		t.Fatal("want basic unauthorized")
	}
}

func TestServer_HandleMetrics_AuthRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")
	st, err := store.Open(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cfg := &config.Config{
		HTTPListen:       ":0",
		HTTPBasePath:     "/api/pgwd/v1",
		HTTPHealthPath:   "/healthz",
		HTTPMetricsPath:  "/metrics",
		HTTPMetricsToken: "tok",
	}
	srv := New(cfg, st)

	req := httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/metrics", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with token: status %d body %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/healthz", nil)
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz should stay open: status %d", rec.Code)
	}
}
