// Package collector sends anonymous, non-identifying usage statistics to help
// improve pgwd and checks for new releases. No credentials, hostnames, database
// names, or webhook URLs are ever transmitted — only boolean feature flags and a
// one-way hash for deduplication.
package collector

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// IngestURL is the HTTPS endpoint for opt-in anonymous usage reports (project pgwd).
	IngestURL = "https://collect.gghstats.com/a1b2c3d4e5f6a7b8"
)

var (
	collectorURL    = IngestURL
	releaseCheckURL = "https://api.github.com/repos/hrodrig/pgwd/releases/latest"
)

// UpdateCheckURL returns the GitHub API URL used for the opt-out release check.
func UpdateCheckURL() string {
	return releaseCheckURL
}

// BuildInfo holds release identity injected at link time.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

type data struct {
	Version   string   `json:"version"`
	Commit    string   `json:"commit"`
	BuildDate string   `json:"build_date"`
	Hash      string   `json:"hash"`
	Features  features `json:"features"`
}

type features struct {
	MultiDB            bool `json:"multi_db"`
	UsesLevelMode      bool `json:"uses_level_mode"`
	LongQueryEnabled   bool `json:"long_query_enabled"`
	HasSlack           bool `json:"has_slack"`
	HasLoki            bool `json:"has_loki"`
	HasKubePostgres    bool `json:"has_kube_postgres"`
	HasKubeLoki        bool `json:"has_kube_loki"`
	HasSQLiteStore     bool `json:"has_sqlite_store"`
	HasSQLMetricsStore bool `json:"has_sql_metrics_store"`
	HasHTTPListen      bool `json:"has_http_listen"`
	ConfirmAlertGT1    bool `json:"confirm_alert_gt_1"`
	ConfirmOkGT1       bool `json:"confirm_ok_gt_1"`
	DryRun             bool `json:"dry_run"`
}

// ConfigFeatures is the subset of pgwd configuration used for anonymous collection.
// All fields are booleans derived from the actual config so no real data leaves the machine.
type ConfigFeatures struct {
	MultiDB            bool
	UsesLevelMode      bool
	LongQueryEnabled   bool
	HasSlack           bool
	HasLoki            bool
	HasKubePostgres    bool
	HasKubeLoki        bool
	HasSQLiteStore     bool
	HasSQLMetricsStore bool
	HasHTTPListen      bool
	ConfirmAlertGT1    bool
	ConfirmOkGT1       bool
	DryRun             bool
}

// Collect sends one anonymous usage report. Errors are logged at debug only and never returned.
func Collect(info BuildInfo, cfg ConfigFeatures, debug bool) {
	buf, err := createBody(info, cfg)
	if err != nil {
		debugLog(debug, "collector: failed to create body: %v", err)
		return
	}

	client := makeHTTPClient()
	client.Timeout = 15 * time.Second

	debugLog(debug, "collector: sending anonymous usage report: %s", buf.String())

	resp, err := client.Post(collectorURL, "application/json; charset=utf-8", buf)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		debugLog(debug, "collector: send failed: %v", err)
		return
	}
	debugLog(debug, "collector: anonymous stats sent")
}

// CollectWithUpdate sends an anonymous usage report and checks for newer releases.
func CollectWithUpdate(info BuildInfo, cfg ConfigFeatures, debug bool) {
	Collect(info, cfg, debug)
	CheckUpdate(info, debug)
}

func createBody(info BuildInfo, cfg ConfigFeatures) (*bytes.Buffer, error) {
	hash := hashConfig(cfg)

	f := features{
		MultiDB:            cfg.MultiDB,
		UsesLevelMode:      cfg.UsesLevelMode,
		LongQueryEnabled:   cfg.LongQueryEnabled,
		HasSlack:           cfg.HasSlack,
		HasLoki:            cfg.HasLoki,
		HasKubePostgres:    cfg.HasKubePostgres,
		HasKubeLoki:        cfg.HasKubeLoki,
		HasSQLiteStore:     cfg.HasSQLiteStore,
		HasSQLMetricsStore: cfg.HasSQLMetricsStore,
		HasHTTPListen:      cfg.HasHTTPListen,
		ConfirmAlertGT1:    cfg.ConfirmAlertGT1,
		ConfirmOkGT1:       cfg.ConfirmOkGT1,
		DryRun:             cfg.DryRun,
	}

	d := &data{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildDate: info.BuildDate,
		Hash:      hash,
		Features:  f,
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(d); err != nil {
		return nil, err
	}
	return buf, nil
}

func hashConfig(cfg ConfigFeatures) string {
	h := sha256.New()
	fmt.Fprintf(h, "%v", cfg)
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func makeHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: transport}
}

// CheckUpdate queries the GitHub API for the latest release and logs a warning when older.
func CheckUpdate(info BuildInfo, debug bool) {
	client := makeHTTPClient()
	client.Timeout = 15 * time.Second
	checkUpdate(client, info, debug)
}

func checkUpdate(client *http.Client, info BuildInfo, debug bool) {
	req, err := http.NewRequest(http.MethodGet, releaseCheckURL, nil)
	if err != nil {
		debugLog(debug, "collector: update check request failed: %v", err)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pgwd/"+info.Version)

	resp, err := client.Do(req)
	if err != nil {
		debugLog(debug, "collector: update check request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	var latest struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		debugLog(debug, "collector: update check decode failed: %v", err)
		return
	}

	latestVer := strings.TrimPrefix(latest.TagName, "v")
	if latestVer == "" || latestVer == info.Version {
		return
	}

	if semverGT(latestVer, info.Version) {
		log.Printf("pgwd: a new release has been found: %s — please consider upgrading (current: v%s)",
			latest.TagName, info.Version)
	}
}

func semverGT(a, b string) bool {
	pa := parseSemver(a)
	pb := parseSemver(b)
	if len(pa) == 0 || len(pb) == 0 {
		return a > b
	}
	n := len(pa)
	if len(pb) < n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		if pa[i] > pb[i] {
			return true
		}
		if pa[i] < pb[i] {
			return false
		}
	}
	return len(pa) > len(pb)
}

func parseSemver(v string) []int {
	parts := strings.SplitN(v, ".", 3)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		var n int
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return nil
			}
			n = n*10 + int(ch-'0')
		}
		out = append(out, n)
	}
	return out
}

func debugLog(debug bool, format string, args ...any) {
	if debug {
		log.Printf(format, args...)
	}
}
