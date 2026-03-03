package kube

import (
	"context"
	"strings"
	"testing"
)

func TestValidateKubernetesAccess_InvalidContext(t *testing.T) {
	ctx := context.Background()
	// Use a context that does not exist; kubectl will fail
	err := ValidateKubernetesAccess(ctx, "pgwd-test-nonexistent-context-xyz")
	if err == nil {
		t.Skip("kubectl succeeded (cluster may exist); cannot assert failure")
	}
	if !strings.Contains(err.Error(), "kubectl") {
		t.Errorf("error should mention kubectl, got: %v", err)
	}
}
