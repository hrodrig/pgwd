package collector

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/pgwd/internal/config"
	"github.com/hrodrig/pgwd/internal/metricsstore"
)

func TestCreateBody_allowedFieldsOnly(t *testing.T) {
	cfg := ConfigFeatures{
		MultiDB:          true,
		UsesLevelMode:    true,
		LongQueryEnabled: true,
		HasSlack:         true,
		HasLoki:          true,
		HasKubePostgres:  true,
		HasHTTPListen:    true,
		ConfirmAlertGT1:  true,
	}
	info := BuildInfo{Version: "0.9.0", Commit: "abc", BuildDate: "2026-07-13"}

	buf, err := createBody(info, cfg)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}

	for _, forbidden := range []string{
		"client", "url", "dsn", "webhook", "database", "host", "cluster", "namespace", "product",
	} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("payload must not contain %q", forbidden)
		}
	}

	for _, required := range []string{"version", "commit", "build_date", "hash", "features"} {
		if _, ok := raw[required]; ok {
			continue
		}
		t.Errorf("payload missing %q", required)
	}

	var feats map[string]bool
	if err := json.Unmarshal(raw["features"], &feats); err != nil {
		t.Fatal(err)
	}
	if !feats["multi_db"] || !feats["has_slack"] {
		t.Fatalf("features = %#v", feats)
	}
	if len(raw["hash"]) < 4 {
		t.Fatalf("hash too short: %s", raw["hash"])
	}
}

func TestFeaturesFromConfig(t *testing.T) {
	if f := FeaturesFromConfig(nil); f != (ConfigFeatures{}) {
		t.Fatalf("nil cfg = %+v", f)
	}

	cfg := &config.Config{
		Databases: []config.DatabaseTarget{
			{URL: "postgres://u:p@h1:5432/a"},
			{URL: "postgres://u:p@h2:5432/b"},
		},
		SlackWebhook:        "https://hooks.slack.com/x",
		KubePostgres:        "default/svc/pg",
		SqlitePath:          "/var/lib/pgwd/pgwd.db",
		HTTPListen:          ":8080",
		ConfirmAlert:        2,
		LongQueryMinSeconds: 60,
		ThresholdLevels:     "75,85,95",
	}
	f := FeaturesFromConfig(cfg)
	if !f.MultiDB || !f.HasSlack || !f.HasKubePostgres || !f.HasSQLiteStore || !f.HasHTTPListen || !f.ConfirmAlertGT1 || !f.LongQueryEnabled || !f.UsesLevelMode {
		t.Fatalf("FeaturesFromConfig = %+v", f)
	}

	cfg2 := &config.Config{
		MetricsStoreDriver: metricsstore.DriverPostgres,
		MetricsStoreDSN:    "postgres://x",
		LokiURL:            "http://loki/push",
		KubeLoki:           "monitoring/svc/loki",
	}
	f2 := FeaturesFromConfig(cfg2)
	if !f2.HasSQLMetricsStore || !f2.HasLoki || !f2.HasKubeLoki {
		t.Fatalf("FeaturesFromConfig sql = %+v", f2)
	}
}

func TestSemverGT(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.9.1", "0.9.0", true},
		{"0.9.0", "0.9.1", false},
		{"1.0.0", "0.9.9", true},
		{"0.9.0", "0.9.0", false},
		{"1.0", "0.9.9", true},
	}
	for _, tt := range tests {
		if got := semverGT(tt.a, tt.b); got != tt.want {
			t.Errorf("semverGT(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	if got := parseSemver("1.2.3"); len(got) != 3 || got[0] != 1 {
		t.Fatalf("parseSemver = %v", got)
	}
	if parseSemver("1.2rc") != nil {
		t.Fatal("expected nil for non-numeric segment")
	}
}

func TestUpdateCheckURL(t *testing.T) {
	if UpdateCheckURL() == "" || !strings.Contains(UpdateCheckURL(), "github.com") {
		t.Fatalf("UpdateCheckURL = %q", UpdateCheckURL())
	}
	if IngestURL == "" || !strings.Contains(IngestURL, "collect.gghstats.com") {
		t.Fatalf("IngestURL = %q", IngestURL)
	}
}

func TestHashConfig_stable(t *testing.T) {
	cfg := ConfigFeatures{HasSlack: true, UsesLevelMode: true}
	h1 := hashConfig(cfg)
	h2 := hashConfig(cfg)
	if h1 != h2 || len(h1) != 16 {
		t.Fatalf("hash = %q", h1)
	}
}

func TestCollect_postsToIngestURL(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	old := collectorURL
	collectorURL = srv.URL
	t.Cleanup(func() { collectorURL = old })

	info := BuildInfo{Version: "0.9.0", Commit: "abc", BuildDate: "2026-07-13"}
	Collect(info, ConfigFeatures{HasSlack: true}, true)
	if body == "" || !strings.Contains(body, "has_slack") {
		t.Fatalf("body = %q", body)
	}
}

func TestCollect_badURL_debugOnly(t *testing.T) {
	old := collectorURL
	collectorURL = "http://127.0.0.1:1"
	t.Cleanup(func() { collectorURL = old })

	Collect(BuildInfo{Version: "0.9.0"}, ConfigFeatures{}, true)
	Collect(BuildInfo{Version: "0.9.0"}, ConfigFeatures{}, false)
}

func TestCheckUpdate_warnsOnNewerRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v99.0.0"}`))
	}))
	defer srv.Close()

	old := releaseCheckURL
	releaseCheckURL = srv.URL
	t.Cleanup(func() { releaseCheckURL = old })

	CheckUpdate(BuildInfo{Version: "0.9.0"}, false)
}

func TestCheckUpdate_sameVersionNoWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.9.0"}`))
	}))
	defer srv.Close()

	old := releaseCheckURL
	releaseCheckURL = srv.URL
	t.Cleanup(func() { releaseCheckURL = old })

	CheckUpdate(BuildInfo{Version: "0.9.0"}, false)
}

func TestCheckUpdate_badJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	old := releaseCheckURL
	releaseCheckURL = srv.URL
	t.Cleanup(func() { releaseCheckURL = old })

	CheckUpdate(BuildInfo{Version: "0.9.0"}, true)
}

func TestCollectWithUpdate(t *testing.T) {
	collectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	releaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.9.0"}`))
	}))
	defer collectSrv.Close()
	defer releaseSrv.Close()

	oldCollect := collectorURL
	oldRelease := releaseCheckURL
	collectorURL = collectSrv.URL
	releaseCheckURL = releaseSrv.URL
	t.Cleanup(func() {
		collectorURL = oldCollect
		releaseCheckURL = oldRelease
	})

	CollectWithUpdate(BuildInfo{Version: "0.9.0"}, ConfigFeatures{HasLoki: true}, false)
}

func TestMakeHTTPClient(t *testing.T) {
	c := makeHTTPClient()
	if c == nil || c.Transport == nil {
		t.Fatal("expected client")
	}
}

func TestDebugLog(t *testing.T) {
	debugLog(false, "skip %s", "x")
	debugLog(true, "log %s", "y")
}
