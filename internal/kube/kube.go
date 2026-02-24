package kube

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const discoverPasswordPlaceholder = "DISCOVER_MY_PASSWORD"

// RequireKubectl returns an error if kubectl is not found in PATH. Call this when -kube-postgres is set so the user gets a clear message before any other kube step.
func RequireKubectl() error {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl not found in PATH (required for -kube-postgres): %w", err)
	}
	return nil
}

// ClusterName returns the current context's cluster name from kubeconfig (e.g. for Slack notifications). Empty string on error or when not set.
func ClusterName(ctx context.Context) string {
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return ""
	}
	cmd := exec.CommandContext(ctx, kubectl, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].name}")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ParseKubePostgres parses "namespace/type/name" (e.g. "default/svc/postgres" or "default/pod/postgres-0").
// Returns namespace, resource (e.g. "svc/postgres"), and error if format is invalid.
func ParseKubePostgres(s string) (namespace, resource string, err error) {
	parts := strings.SplitN(s, "/", 3)
	if len(parts) != 3 {
		return "", "", fmt.Errorf("kube-postgres must be namespace/type/name (e.g. default/svc/postgres), got %q", s)
	}
	namespace, resType, name := parts[0], strings.ToLower(parts[1]), parts[2]
	if namespace == "" || name == "" {
		return "", "", fmt.Errorf("namespace and name must be non-empty")
	}
	if resType != "svc" && resType != "pod" {
		return "", "", fmt.Errorf("type must be svc or pod, got %q", resType)
	}
	return namespace, resType + "/" + name, nil
}

// ResolvePod returns the pod name. If resource is "pod/name", returns name. If "svc/name", looks up a pod via endpoints or service selector.
func ResolvePod(ctx context.Context, namespace, resource string) (string, error) {
	if strings.HasPrefix(resource, "pod/") {
		return strings.TrimPrefix(resource, "pod/"), nil
	}
	if !strings.HasPrefix(resource, "svc/") {
		return "", fmt.Errorf("resource must be pod/name or svc/name, got %q", resource)
	}
	svcName := strings.TrimPrefix(resource, "svc/")
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return "", fmt.Errorf("kubectl not found in PATH: %w", err)
	}
	// Try endpoints first: get first address targetRef name
	cmd := exec.CommandContext(ctx, kubectl, "get", "endpoints", "-n", namespace, svcName, "-o", "jsonpath={.subsets[0].addresses[0].targetRef.name}")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return strings.TrimSpace(string(out)), nil
	}
	// Fallback: get service selector, then get first pod
	cmd = exec.CommandContext(ctx, kubectl, "get", "svc", "-n", namespace, svcName, "-o", "go-template={{range $k,$v := .spec.selector}}{{$k}}={{$v}},{{end}}")
	out, err = cmd.Output()
	if err != nil || len(out) == 0 {
		return "", fmt.Errorf("could not get pod from service %s (no endpoints or selector): %w", svcName, err)
	}
	selector := strings.TrimSuffix(strings.TrimSpace(string(out)), ",")
	if selector == "" {
		return "", fmt.Errorf("service %s has no selector", svcName)
	}
	cmd = exec.CommandContext(ctx, kubectl, "get", "pods", "-n", namespace, "-l", selector, "-o", "jsonpath={.items[0].metadata.name}")
	out, err = cmd.Output()
	if err != nil || len(out) == 0 {
		return "", fmt.Errorf("no pods found for service %s (selector %s)", svcName, selector)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetPasswordFromPod reads the given env var (and PGPASSWORD as fallback) from the pod's container.
func GetPasswordFromPod(ctx context.Context, namespace, podName, container, envVar string) (string, error) {
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return "", fmt.Errorf("kubectl not found in PATH: %w", err)
	}
	args := []string{"exec", "-n", namespace, podName}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--", "printenv", envVar)
	cmd := exec.CommandContext(ctx, kubectl, args...)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return strings.TrimSpace(string(out)), nil
	}
	if envVar != "PGPASSWORD" {
		return GetPasswordFromPod(ctx, namespace, podName, container, "PGPASSWORD")
	}
	return "", fmt.Errorf("could not find %s or PGPASSWORD in pod %s", envVar, podName)
}

// StartPortForward runs kubectl port-forward in the background and waits for the local port to be listening.
// Returns a cleanup function that kills the port-forward process. Call it on exit.
func StartPortForward(ctx context.Context, namespace, resource string, localPort int) (cleanup func(), err error) {
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("kubectl not found in PATH: %w", err)
	}
	addr := fmt.Sprintf("%d:5432", localPort)
	cmd := exec.CommandContext(ctx, kubectl, "port-forward", "-n", namespace, resource, addr)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start port-forward: %w", err)
	}
	cleanup = func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	// Wait for port to be reachable
	for i := 0; i < 40; i++ {
		select {
		case <-ctx.Done():
			cleanup()
			return nil, ctx.Err()
		default:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 200*time.Millisecond)
			if err == nil {
				conn.Close()
				return cleanup, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	cleanup()
	return nil, fmt.Errorf("port %d did not become ready in time (port-forward may have failed)", localPort)
}

// DiscoverPasswordPlaceholder returns the placeholder string used in DBURL to trigger password discovery from the pod.
func DiscoverPasswordPlaceholder() string {
	return discoverPasswordPlaceholder
}

// URLContainsDiscoverPassword returns true if the connection URL contains the discover-password placeholder.
func URLContainsDiscoverPassword(dbURL string) bool {
	return strings.Contains(dbURL, discoverPasswordPlaceholder)
}

// ReplaceDBURLForKube returns a new connection URL with host set to localhost:localPort and, if newPassword is non-empty, the user info password replaced.
func ReplaceDBURLForKube(dbURL string, newPassword string, localPort int) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("parse DB URL: %w", err)
	}
	u.Host = net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort))
	if newPassword != "" && u.User != nil {
		user := u.User.Username()
		u.User = url.UserPassword(user, newPassword)
	}
	return u.String(), nil
}
