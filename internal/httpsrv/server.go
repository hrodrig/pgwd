package httpsrv

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/store"
)

func escapePrometheusLabelValue(val string) string {
	var b strings.Builder
	for _, r := range val {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			// drop CR; LF is escaped above
		default:
			if r < 0x20 {
				b.WriteRune('_')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func promLabel(key, val string) string {
	return fmt.Sprintf(`%s="%s"`, key, escapePrometheusLabelValue(val))
}

// Server serves /healthz and /metrics for Kubernetes probes and Prometheus.
type Server struct {
	mux    *http.ServeMux
	cfg    *config.Config
	st     store.MetricsStorer
	server *http.Server
}

// New creates an HTTP server. cfg.HTTPListen must be non-empty. st can be nil (metrics returns empty).
func New(cfg *config.Config, st store.MetricsStorer) *Server {
	mux := http.NewServeMux()
	s := &Server{mux: mux, cfg: cfg, st: st}
	base := strings.TrimSuffix(cfg.HTTPBasePath, "/")
	if base == "" {
		base = "/"
	}
	healthPath := base + "/" + strings.TrimPrefix(cfg.HTTPHealthPath, "/")
	metricsPath := base + "/" + strings.TrimPrefix(cfg.HTTPMetricsPath, "/")
	mux.HandleFunc(healthPath, s.handleHealth)
	mux.HandleFunc(metricsPath, s.handleMetrics)
	return s
}

// Start starts the server in a goroutine. Call Stop to shut down.
func (s *Server) Start() error {
	if s.cfg.HTTPListen == "" {
		return nil
	}
	s.server = &http.Server{Addr: s.cfg.HTTPListen, Handler: s.mux}
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("pgwd http: %v\n", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.st != nil {
		if err := s.st.Ping(r.Context()); err != nil {
			http.Error(w, "store unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if !metricsAuthorized(r, s.cfg) {
		writeMetricsUnauthorized(w, s.cfg)
		return
	}
	if s.st == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# No metrics store configured\n"))
		return
	}
	recs, err := s.st.LatestRecords(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var b strings.Builder
	stateVal := map[string]int{"ok": 0, "attention": 1, "alert": 2, "danger": 3, "connect_failure": 4}
	for _, r := range recs {
		labels := strings.Join([]string{promLabel("client", r.Client), promLabel("cluster", r.Cluster), promLabel("database", r.Database)}, ",")
		b.WriteString(fmt.Sprintf("pgwd_connections_total{%s} %d\n", labels, r.Total))
		b.WriteString(fmt.Sprintf("pgwd_connections_active{%s} %d\n", labels, r.Active))
		b.WriteString(fmt.Sprintf("pgwd_connections_idle{%s} %d\n", labels, r.Idle))
		b.WriteString(fmt.Sprintf("pgwd_connections_stale{%s} %d\n", labels, r.Stale))
		b.WriteString(fmt.Sprintf("pgwd_max_connections{%s} %d\n", labels, r.MaxConnections))
		v := 0
		if x, ok := stateVal[r.State]; ok {
			v = x
		}
		b.WriteString(fmt.Sprintf("pgwd_state{%s} %d\n", labels, v))
	}
	if b.Len() == 0 {
		b.WriteString("# No metrics yet (no checks recorded)\n")
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(b.String()))
}
