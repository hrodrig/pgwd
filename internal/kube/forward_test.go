package kube

import "testing"

func TestResolveForwardPod_pod(t *testing.T) {
	got, err := resolveForwardPod(t.Context(), "", "ns", "pod/pg-0")
	if err != nil || got != "pg-0" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestResolveForwardPod_badResource(t *testing.T) {
	_, err := resolveForwardPod(t.Context(), "", "ns", "deploy/x")
	if err == nil {
		t.Fatal("want error")
	}
}
