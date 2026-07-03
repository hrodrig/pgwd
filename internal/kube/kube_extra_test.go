package kube

import (
	"context"
	"testing"
)

func TestRequireKubectl_noop(t *testing.T) {
	if err := RequireKubectl(); err != nil {
		t.Fatal(err)
	}
}

func TestClusterName_noKubeconfig(t *testing.T) {
	_ = ClusterName(context.Background(), "nonexistent-context-xyz")
}
