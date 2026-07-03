package notify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RetryConfig controls transient HTTP notify retries (5xx and network errors).
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig matches plan-0.7.x defaults.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:    3,
	InitialBackoff: time.Second,
	MaxBackoff:     10 * time.Second,
}

var (
	notifyRetry   = DefaultRetryConfig
	notifyHTTPCli = &http.Client{Timeout: 30 * time.Second}
)

// ApplyRetryConfig sets package-level retry behavior for outbound notifier HTTP.
func ApplyRetryConfig(cfg RetryConfig) {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 10 * time.Second
	}
	notifyRetry = cfg
}

func postJSONWithRetry(ctx context.Context, url string, body []byte, setHeaders func(*http.Request)) error {
	var lastErr error
	backoff := notifyRetry.InitialBackoff

	for attempt := 1; attempt <= notifyRetry.MaxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if setHeaders != nil {
			setHeaders(req)
		}

		resp, err := notifyHTTPCli.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send request: %w", err)
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return nil
			}
			if resp.StatusCode < 500 {
				return fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
			lastErr = fmt.Errorf("HTTP status %d", resp.StatusCode)
		}

		if attempt == notifyRetry.MaxAttempts {
			break
		}
		if !sleepCtx(ctx, backoff) {
			return lastErr
		}
		if backoff < notifyRetry.MaxBackoff {
			backoff *= 2
			if backoff > notifyRetry.MaxBackoff {
				backoff = notifyRetry.MaxBackoff
			}
		}
	}
	return lastErr
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
