package notify

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestRedactNetError_stripsPathAndQuery(t *testing.T) {
	inner := errors.New("connection refused")
	err := &url.Error{
		Op:  "Post",
		URL: "https://hooks.slack.com/services/T00/B00/secret-token",
		Err: inner,
	}
	got := RedactNetError(err)
	msg := got.Error()
	if strings.Contains(msg, "secret-token") || strings.Contains(msg, "/services/") {
		t.Fatalf("secret still in error: %q", msg)
	}
	if !strings.Contains(msg, "hooks.slack.com") {
		t.Fatalf("want host kept: %q", msg)
	}
	var ue *url.Error
	if !errors.As(got, &ue) {
		t.Fatal("want *url.Error in chain")
	}
	if ue.URL != "https://hooks.slack.com" {
		t.Fatalf("URL = %q", ue.URL)
	}
}

func TestRedactNetError_passthroughNonURL(t *testing.T) {
	err := errors.New("plain")
	if RedactNetError(err) != err {
		t.Fatal("want same error")
	}
}

func TestRedactNetError_nil(t *testing.T) {
	if RedactNetError(nil) != nil {
		t.Fatal("want nil")
	}
}
