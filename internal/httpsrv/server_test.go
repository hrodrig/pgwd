package httpsrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

func TestServer_HandleHealth_NoStore(t *testing.T) {
	cfg := &config.Config{
		HTTPListen:      ":0",
		HTTPBasePath:    "/api/pgwd/v1",
		HTTPHealthPath:  "/healthz",
		HTTPMetricsPath: "/metrics",
	}
	srv := New(cfg, nil)
	handler := srv.mux

	req := httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleHealth: status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("handleHealth: body = %q, want \"ok\"", body)
	}
}

func TestServer_HandleHealth_WithStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")
	st, err := store.Open(path, 100)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{
		HTTPListen:      ":0",
		HTTPBasePath:    "/api/pgwd/v1",
		HTTPHealthPath:  "/healthz",
		HTTPMetricsPath: "/metrics",
	}
	srv := New(cfg, st)
	req := httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/healthz", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleHealth: status = %d, want 200", rec.Code)
	}
}

func TestServer_HandleMetrics_NoStore(t *testing.T) {
	cfg := &config.Config{
		HTTPListen:      ":0",
		HTTPBasePath:    "/api/pgwd/v1",
		HTTPHealthPath:  "/healthz",
		HTTPMetricsPath: "/metrics",
	}
	srv := New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/metrics", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleMetrics: status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No metrics store configured") {
		t.Errorf("handleMetrics: want 'No metrics store configured', got %q", rec.Body.String())
	}
}

func TestServer_HandleMetrics_WithStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")
	st, err := store.Open(path, 100)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	if err := st.Insert(ctx, store.Record{
		Client: "c", Cluster: "cl", Database: "db",
		Total: 50, Active: 10, Idle: 40, Stale: 0, MaxConnections: 100,
		State: "ok", Threshold: "",
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	cfg := &config.Config{
		HTTPListen:      ":0",
		HTTPBasePath:    "/api/pgwd/v1",
		HTTPHealthPath:  "/healthz",
		HTTPMetricsPath: "/metrics",
	}
	srv := New(cfg, st)

	req := httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/metrics", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleMetrics: status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "pgwd_connections_total") {
		t.Errorf("handleMetrics: want pgwd_connections_total, got %q", body)
	}
	if !strings.Contains(body, `client="c"`) {
		t.Errorf("handleMetrics: want client label, got %q", body)
	}
	if !strings.Contains(body, "pgwd_state") {
		t.Errorf("handleMetrics: want pgwd_state, got %q", body)
	}
}

func TestServer_HandleMetrics_LabelEscape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")
	st, err := store.Open(path, 100)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	if err := st.Insert(ctx, store.Record{
		Client: `c"1`, Cluster: "cl\n2", Database: "db",
		Total: 1, Active: 0, Idle: 1, Stale: 0, MaxConnections: 10,
		State: "ok", Threshold: "",
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	cfg := &config.Config{
		HTTPListen:      ":0",
		HTTPBasePath:    "/api/pgwd/v1",
		HTTPHealthPath:  "/healthz",
		HTTPMetricsPath: "/metrics",
	}
	srv := New(cfg, st)

	req := httptest.NewRequest(http.MethodGet, "/api/pgwd/v1/metrics", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "\nclient") || strings.Contains(body, "cl\n2") {
		t.Errorf("raw newline in metrics body: %q", body)
	}
	if !strings.Contains(body, `client="c\"1"`) {
		t.Errorf("want escaped quote in client label, got %q", body)
	}
	if !strings.Contains(body, `cluster="cl\n2"`) {
		t.Errorf("want escaped newline in cluster label, got %q", body)
	}
}

func TestServer_StartStop(t *testing.T) {
	cfg := &config.Config{HTTPListen: ":0", HTTPBasePath: "/api/pgwd/v1", HTTPHealthPath: "/healthz", HTTPMetricsPath: "/metrics"}
	srv := New(cfg, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestServer_Start_EmptyListen(t *testing.T) {
	cfg := &config.Config{HTTPListen: "", HTTPBasePath: "/api/pgwd/v1", HTTPHealthPath: "/healthz", HTTPMetricsPath: "/metrics"}
	srv := New(cfg, nil)
	if err := srv.Start(); err != nil {
		t.Errorf("Start with empty listen: %v", err)
	}
	// Stop is no-op when server is nil
	if err := srv.Stop(context.Background()); err != nil {
		t.Errorf("Stop: %v", err)
	}
}
