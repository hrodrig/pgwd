package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestApplyRetryConfig_normalizes(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })

	ApplyRetryConfig(RetryConfig{MaxAttempts: 0, InitialBackoff: 0, MaxBackoff: 0})
	if notifyRetry.MaxAttempts != 1 {
		t.Fatalf("MaxAttempts=%d", notifyRetry.MaxAttempts)
	}
	if notifyRetry.InitialBackoff != time.Second {
		t.Fatalf("InitialBackoff=%v", notifyRetry.InitialBackoff)
	}
	if notifyRetry.MaxBackoff != 10*time.Second {
		t.Fatalf("MaxBackoff=%v", notifyRetry.MaxBackoff)
	}
}

func TestPostJSONWithRetry_retries5xx(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond})

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	if err := postJSONWithRetry(context.Background(), srv.URL, []byte(`{}`), nil); err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Fatalf("hits=%d want 3", hits)
	}
}

func TestPostJSONWithRetry_noRetry4xx(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond})

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	if err := postJSONWithRetry(context.Background(), srv.URL, []byte(`{}`), nil); err == nil {
		t.Fatal("expected error")
	}
	if hits != 1 {
		t.Fatalf("hits=%d want 1", hits)
	}
}

func TestPostJSONWithRetry_setHeaders(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	err := postJSONWithRetry(context.Background(), srv.URL, []byte(`{}`), func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer token")
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("Authorization=%q", gotAuth)
	}
}

func TestPostJSONWithRetry_contextCancelDuringBackoff(t *testing.T) {
	t.Cleanup(func() { ApplyRetryConfig(DefaultRetryConfig) })
	ApplyRetryConfig(RetryConfig{MaxAttempts: 3, InitialBackoff: 500 * time.Millisecond, MaxBackoff: time.Second})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := postJSONWithRetry(ctx, srv.URL, []byte(`{}`), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("err=%v", err)
	}
}
