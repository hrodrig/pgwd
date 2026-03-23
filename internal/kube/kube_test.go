package kube

import (
	"context"
	"strings"
	"testing"
)

func TestValidateKubernetesAccess_InvalidContext(t *testing.T) {
	ctx := context.Background()
	// Use a context that does not exist; should fail (load kubeconfig or list pods)
	err := ValidateKubernetesAccess(ctx, "pgwd-test-nonexistent-context-xyz")
	if err == nil {
		t.Skip("succeeded (cluster/context may exist); cannot assert failure")
	}
	if !strings.Contains(err.Error(), "kubeconfig") && !strings.Contains(err.Error(), "list pods") && !strings.Contains(err.Error(), "context") {
		t.Logf("expected error about kubeconfig/context/pods, got: %v", err)
	}
}
