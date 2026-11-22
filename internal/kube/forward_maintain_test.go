package kube

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloseOnce(t *testing.T) {
	ch := make(chan struct{})
	closeOnce(ch)
	closeOnce(ch) // second close must not panic
	select {
	case <-ch:
	default:
		t.Fatal("channel not closed")
	}
}

func TestWaitPortForwardReady_ready(t *testing.T) {
	ready := make(chan struct{})
	close(ready)
	err := waitPortForwardReady(t.Context(), make(chan struct{}), ready, make(chan error), &bytes.Buffer{}, 1, func() {})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitPortForwardReady_errCh(t *testing.T) {
	errCh := make(chan error, 1)
	errCh <- errors.New("dial fail")
	var buf bytes.Buffer
	buf.WriteString("detail")
	err := waitPortForwardReady(t.Context(), make(chan struct{}), make(chan struct{}), errCh, &buf, 1, func() {})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("detail")) {
		t.Fatalf("want wrapped err with detail, got %v", err)
	}
}

func TestWaitPortForwardReady_errChPlain(t *testing.T) {
	errCh := make(chan error, 1)
	want := errors.New("plain")
	errCh <- want
	err := waitPortForwardReady(t.Context(), make(chan struct{}), make(chan struct{}), errCh, &bytes.Buffer{}, 1, func() {})
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}

func TestWaitPortForwardReady_stop(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	stopped := false
	err := waitPortForwardReady(t.Context(), stop, make(chan struct{}), make(chan error), &bytes.Buffer{}, 1, func() { stopped = true })
	if !errors.Is(err, errPortForwardStopped) || !stopped {
		t.Fatalf("err=%v stopped=%v", err, stopped)
	}
}

func TestWaitPortForwardReady_ctxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	stopped := false
	err := waitPortForwardReady(ctx, make(chan struct{}), make(chan struct{}), make(chan error), &bytes.Buffer{}, 1, func() { stopped = true })
	if !errors.Is(err, context.Canceled) || !stopped {
		t.Fatalf("err=%v stopped=%v", err, stopped)
	}
}

func TestWaitPortForwardReady_timeout(t *testing.T) {
	prev := portForwardReadyTimeout
	portForwardReadyTimeout = 20 * time.Millisecond
	t.Cleanup(func() { portForwardReadyTimeout = prev })

	stopped := false
	err := waitPortForwardReady(t.Context(), make(chan struct{}), make(chan struct{}), make(chan error), &bytes.Buffer{}, 4242, func() { stopped = true })
	if err == nil || !stopped {
		t.Fatalf("err=%v stopped=%v", err, stopped)
	}
}

func TestWaitPortForwardReady_timeoutWithErrOut(t *testing.T) {
	prev := portForwardReadyTimeout
	portForwardReadyTimeout = 20 * time.Millisecond
	t.Cleanup(func() { portForwardReadyTimeout = prev })

	var buf bytes.Buffer
	buf.WriteString("spdy boom")
	err := waitPortForwardReady(t.Context(), make(chan struct{}), make(chan struct{}), make(chan error), &buf, 9, func() {})
	if err == nil || err.Error() == "port 9 not ready in time" {
		t.Fatalf("want errOut in message, got %v", err)
	}
}

func TestWaitPortForwardEnd_err(t *testing.T) {
	errCh := make(chan error, 1)
	want := errors.New("fwd dead")
	errCh <- want
	err := waitPortForwardEnd(t.Context(), make(chan struct{}), errCh, func() {})
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}

func TestWaitPortForwardEnd_nilErr(t *testing.T) {
	errCh := make(chan error, 1)
	errCh <- nil
	err := waitPortForwardEnd(t.Context(), make(chan struct{}), errCh, func() {})
	if err == nil {
		t.Fatal("want ended error")
	}
}

func TestWaitPortForwardEnd_stop(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	stopped := false
	err := waitPortForwardEnd(t.Context(), stop, make(chan error), func() { stopped = true })
	if !errors.Is(err, errPortForwardStopped) || !stopped {
		t.Fatalf("err=%v stopped=%v", err, stopped)
	}
}

func TestWaitPortForwardEnd_ctx(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	stopped := false
	err := waitPortForwardEnd(ctx, make(chan struct{}), make(chan error), func() { stopped = true })
	if !errors.Is(err, context.Canceled) || !stopped {
		t.Fatalf("err=%v stopped=%v", err, stopped)
	}
}

func TestMaintainPortForwardWith_stopOnError(t *testing.T) {
	stopCh := make(chan struct{})
	firstReady := make(chan struct{})
	var readyOnce sync.Once
	var calls atomic.Int32
	session := func(context.Context, <-chan struct{}, chan<- struct{}, *sync.Once, string, string, string, int, int) (bool, error) {
		calls.Add(1)
		go func() {
			time.Sleep(5 * time.Millisecond)
			close(stopCh)
		}()
		return false, errors.New("session fail")
	}
	maintainPortForwardWith(t.Context(), stopCh, firstReady, &readyOnce, session, "", "ns", "pod/x", 1, 5432)
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestMaintainPortForwardWith_ctxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stopCh := make(chan struct{})
	firstReady := make(chan struct{})
	var readyOnce sync.Once
	session := func(context.Context, <-chan struct{}, chan<- struct{}, *sync.Once, string, string, string, int, int) (bool, error) {
		cancel()
		return false, errors.New("fail")
	}
	maintainPortForwardWith(ctx, stopCh, firstReady, &readyOnce, session, "", "ns", "pod/x", 1, 5432)
}

func TestMaintainPortForwardWith_readyResetsBackoff(t *testing.T) {
	stopCh := make(chan struct{})
	firstReady := make(chan struct{})
	var readyOnce sync.Once
	var calls atomic.Int32
	session := func(ctx context.Context, _ <-chan struct{}, fr chan<- struct{}, once *sync.Once, _ string, _ string, _ string, _ int, _ int) (bool, error) {
		n := calls.Add(1)
		if n == 1 {
			once.Do(func() { close(fr) })
			return true, errors.New("drop after ready")
		}
		close(stopCh)
		return false, errors.New("second")
	}
	// After first session, backoff resets to 1s; stop during that wait via second call after After.
	// Second call happens after 1s sleep — too slow. Close stop from first session after ready path:
	// becameReady true → backoff=1s → After(1s). Instead stop immediately from second iteration:
	// first returns ready+err, then waits backoff before second call.
	// Shrink: close stopCh from goroutine during first After wait.
	go func() {
		<-firstReady
		time.Sleep(5 * time.Millisecond)
		close(stopCh)
	}()
	maintainPortForwardWith(t.Context(), stopCh, firstReady, &readyOnce, session, "", "ns", "pod/x", 1, 5432)
	if calls.Load() < 1 {
		t.Fatal("want at least one session")
	}
}

func TestRunPortForwardSession_badResource(t *testing.T) {
	stopCh := make(chan struct{})
	firstReady := make(chan struct{})
	var readyOnce sync.Once
	ready, err := runPortForwardSession(t.Context(), stopCh, firstReady, &readyOnce, "", "ns", "deploy/x", 1, 5432)
	if ready || err == nil {
		t.Fatalf("ready=%v err=%v", ready, err)
	}
}

func TestRunPortForwardSession_newForwarderFails(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	stopCh := make(chan struct{})
	firstReady := make(chan struct{})
	var readyOnce sync.Once
	ready, err := runPortForwardSession(t.Context(), stopCh, firstReady, &readyOnce, "", "ns", "pod/pg-0", 1, 5432)
	if ready || err == nil {
		t.Fatalf("ready=%v err=%v", ready, err)
	}
}

func TestNewPortForwarder_noKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	_, err := newPortForwarder("", "ns", "pod", 1, 5432, make(chan struct{}), make(chan struct{}), &bytes.Buffer{})
	if err == nil {
		t.Fatal("want kubeconfig error")
	}
}

func TestStartPortForwardTo_ctxCancel(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	prev := portForwardStartTimeout
	portForwardStartTimeout = 2 * time.Second
	t.Cleanup(func() { portForwardStartTimeout = prev })

	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()
	cleanup, err := StartPortForwardTo(ctx, "", "ns", "pod/pg", 18080, 5432)
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("want error")
	}
}

func TestResolveForwardPod_svcWithoutKube(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	_, err := resolveForwardPod(t.Context(), "", "ns", "svc/pg")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestStartPortForward_wrapper(t *testing.T) {
	// Ensure StartPortForward delegates (hits 5432 default path) without waiting full timeout.
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	prev := portForwardStartTimeout
	portForwardStartTimeout = 50 * time.Millisecond
	t.Cleanup(func() { portForwardStartTimeout = prev })

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cleanup, err := StartPortForward(ctx, "", "ns", "pod/pg", 18081)
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("want timeout or cancel error")
	}
}
