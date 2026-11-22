package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const discoverPasswordRemovedMsg = "DISCOVER_MY_PASSWORD was removed in pgwd 0.9.x. Migration: docs/kubernetes-passwords.md — use kube.password_from_secret (contrib/profiles/kube-prod.yml), contrib/k8s/pgwd-kube-run.sh, or PGWD_DB_URL from kubectl get secret"

var errDiscoverPasswordRemoved = errors.New(discoverPasswordRemovedMsg)

// PasswordFromSecret mirrors kube.password_from_secret config (avoids importing internal/config).
type PasswordFromSecret struct {
	Namespace string
	Name      string
	Key       string
}

// getConfig loads rest.Config from kubeconfig. kubeContext empty = current context.
func getConfig(kubeContext string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if p := os.Getenv("KUBECONFIG"); p != "" {
		loadingRules.Precedence = append([]string{p}, loadingRules.Precedence...)
	}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return clientConfig.ClientConfig()
}

// getConfigAndClientset returns rest.Config and clientset for the given context.
func getConfigAndClientset(kubeContext string) (*rest.Config, *kubernetes.Clientset, error) {
	config, err := getConfig(kubeContext)
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return config, clientset, nil
}

// RequireKubectl is deprecated and no longer checks for kubectl. Kept for backward compatibility.
// pgwd now uses client-go natively; no kubectl required.
func RequireKubectl() error {
	return nil
}

// ValidateKubernetesAccess checks API reachability via Discovery (no pod list / names printed).
// Requires only authentication to the API server, not cluster-wide pods list RBAC.
func ValidateKubernetesAccess(ctx context.Context, kubeContext string) error {
	_, clientset, err := getConfigAndClientset(kubeContext)
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	info, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("kubernetes API unreachable: %w", err)
	}
	fmt.Fprintf(os.Stdout, "kubernetes ok: version %s\n", info.GitVersion)
	return nil
}

// ClusterName returns the current (or given) context's cluster name from kubeconfig.
func ClusterName(ctx context.Context, kubeContext string) string {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if p := os.Getenv("KUBECONFIG"); p != "" {
		loadingRules.Precedence = append([]string{p}, loadingRules.Precedence...)
	}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}
	raw, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).RawConfig()
	if err != nil {
		return ""
	}
	ctxName := raw.CurrentContext
	if kubeContext != "" {
		ctxName = kubeContext
	}
	ctxObj, ok := raw.Contexts[ctxName]
	if !ok || ctxObj.Cluster == "" {
		return ""
	}
	return ctxObj.Cluster
}

// ParseKubePostgres parses "namespace/type/name" (e.g. "default/svc/postgres" or "default/pod/postgres-0").
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

// podNameFromEndpointSlices returns a pod name from discovery.k8s.io/v1 EndpointSlices
// labeled for the given Service. False if none found (caller may fall back).
func podNameFromEndpointSlices(ctx context.Context, clientset *kubernetes.Clientset, namespace, svcName string) (string, bool) {
	list, err := clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{discoveryv1.LabelServiceName: svcName},
		}),
	})
	if err != nil || len(list.Items) == 0 {
		return "", false
	}
	for i := range list.Items {
		slice := &list.Items[i]
		for j := range slice.Endpoints {
			ep := &slice.Endpoints[j]
			if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" && ep.TargetRef.Name != "" {
				return ep.TargetRef.Name, true
			}
		}
	}
	return "", false
}

func resolvePodFromService(ctx context.Context, clientset *kubernetes.Clientset, namespace, svcName string) (string, error) {
	// Prefer EndpointSlice (avoids deprecated core/v1 Endpoints on Kubernetes 1.33+).
	if podName, ok := podNameFromEndpointSlices(ctx, clientset, namespace, svcName); ok {
		return podName, nil
	}
	// Fallback: get service selector, then list pods
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get service %s: %w", svcName, err)
	}
	if len(svc.Spec.Selector) == 0 {
		return "", fmt.Errorf("service %s has no selector", svcName)
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: svc.Spec.Selector}),
		Limit:         1,
	})
	if err != nil {
		return "", fmt.Errorf("list pods for service %s: %w", svcName, err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("no pods found for service %s", svcName)
	}
	return list.Items[0].Name, nil
}

// ResolvePod returns the pod name. If resource is "pod/name", returns name. If "svc/name", looks up a pod.
func ResolvePod(ctx context.Context, kubeContext, namespace, resource string) (string, error) {
	if strings.HasPrefix(resource, "pod/") {
		return strings.TrimPrefix(resource, "pod/"), nil
	}
	if !strings.HasPrefix(resource, "svc/") {
		return "", fmt.Errorf("resource must be pod/name or svc/name, got %q", resource)
	}
	_, clientset, err := getConfigAndClientset(kubeContext)
	if err != nil {
		return "", err
	}
	return resolvePodFromService(ctx, clientset, namespace, strings.TrimPrefix(resource, "svc/"))
}

// ReadSecretKey returns the string value for key in the named Secret (client-go read-only).
func ReadSecretKey(ctx context.Context, kubeContext, namespace, name, key string) (string, error) {
	if namespace == "" || name == "" {
		return "", fmt.Errorf("secret namespace and name are required")
	}
	if key == "" {
		key = "password"
	}
	_, clientset, err := getConfigAndClientset(kubeContext)
	if err != nil {
		return "", err
	}
	sec, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get secret %s/%s: %w", namespace, name, err)
	}
	b, ok := sec.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no key %q", namespace, name, key)
	}
	return string(b), nil
}

// ResolveKubeDBURL rejects DISCOVER_MY_PASSWORD, optionally loads password or full DSN from a Secret,
// and rewrites the host to localhost for port-forward.
func ResolveKubeDBURL(ctx context.Context, kubeContext, dbURL string, secret PasswordFromSecret, localPort int) (string, error) {
	if containsDiscoverPassword(dbURL) {
		return "", errDiscoverPasswordRemoved
	}
	baseURL := dbURL
	password := ""
	if secret.Name != "" {
		key := secret.Key
		if key == "" {
			key = "password"
		}
		ns := secret.Namespace
		if ns == "" {
			return "", fmt.Errorf("kube password_from_secret: namespace is required")
		}
		val, err := ReadSecretKey(ctx, kubeContext, ns, secret.Name, key)
		if err != nil {
			return "", err
		}
		if key == "url" || strings.HasPrefix(val, "postgres://") || strings.HasPrefix(val, "postgresql://") {
			baseURL = val
		} else {
			password = val
		}
		if containsDiscoverPassword(baseURL) {
			return "", errDiscoverPasswordRemoved
		}
	}
	return ReplaceDBURLForKube(baseURL, password, localPort)
}

func containsDiscoverPassword(dbURL string) bool {
	return strings.Contains(dbURL, "DISCOVER_MY_PASSWORD")
}

// StartPortForward runs port-forward to localPort:5432 (Postgres).
func StartPortForward(ctx context.Context, kubeContext, namespace, resource string, localPort int) (cleanup func(), err error) {
	return StartPortForwardTo(ctx, kubeContext, namespace, resource, localPort, 5432)
}

// StartPortForwardTo runs port-forward in the background (localPort:remotePort) and waits
// for the local port to be listening. If the forward drops (e.g. target pod restart), it
// re-resolves the pod (for svc/) and reconnects until cleanup or ctx cancel.
func StartPortForwardTo(ctx context.Context, kubeContext, namespace, resource string, localPort, remotePort int) (cleanup func(), err error) {
	stopCh := make(chan struct{})
	firstReady := make(chan struct{})
	var readyOnce sync.Once

	go maintainPortForward(ctx, stopCh, firstReady, &readyOnce, kubeContext, namespace, resource, localPort, remotePort)

	cleanupFn := func() {
		select {
		case <-stopCh:
		default:
			close(stopCh)
		}
	}

	select {
	case <-firstReady:
		return cleanupFn, nil
	case <-ctx.Done():
		cleanupFn()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		cleanupFn()
		return nil, fmt.Errorf("port %d did not become ready in time (port-forward may have failed)", localPort)
	}
}

func resolveForwardPod(ctx context.Context, kubeContext, namespace, resource string) (string, error) {
	if strings.HasPrefix(resource, "svc/") {
		return ResolvePod(ctx, kubeContext, namespace, resource)
	}
	if strings.HasPrefix(resource, "pod/") {
		return strings.TrimPrefix(resource, "pod/"), nil
	}
	return "", fmt.Errorf("resource must be pod/name or svc/name, got %q", resource)
}

func maintainPortForward(ctx context.Context, stopCh <-chan struct{}, firstReady chan<- struct{}, readyOnce *sync.Once, kubeContext, namespace, resource string, localPort, remotePort int) {
	backoff := time.Second
	for {
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}
		becameReady, err := runPortForwardSession(ctx, stopCh, firstReady, readyOnce, kubeContext, namespace, resource, localPort, remotePort)
		if becameReady {
			backoff = time.Second
		}
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "pgwd: port-forward: %v (reconnecting)\n", err)
		}
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func newPortForwarder(kubeContext, namespace, podName string, localPort, remotePort int, sessionStop, readyCh chan struct{}, errOut *bytes.Buffer) (*portforward.PortForwarder, error) {
	config, err := getConfig(kubeContext)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	fwdURL, err := url.Parse(config.Host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}
	fwdURL.Path = path
	fwdURL.RawQuery = ""
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("spdy round tripper: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", fwdURL)
	pf, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, remotePort)}, sessionStop, readyCh, io.Discard, errOut)
	if err != nil {
		return nil, fmt.Errorf("create port-forward: %w", err)
	}
	return pf, nil
}

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func waitPortForwardReady(ctx context.Context, stopCh, readyCh <-chan struct{}, errCh <-chan error, errOut *bytes.Buffer, localPort int, stopSession func()) error {
	select {
	case <-readyCh:
		return nil
	case err := <-errCh:
		if errOut.Len() > 0 {
			return fmt.Errorf("%w: %s", err, errOut.String())
		}
		return err
	case <-stopCh:
		stopSession()
		return errPortForwardStopped
	case <-ctx.Done():
		stopSession()
		return ctx.Err()
	case <-time.After(10 * time.Second):
		stopSession()
		if errOut.Len() > 0 {
			return fmt.Errorf("port %d not ready: %s", localPort, errOut.String())
		}
		return fmt.Errorf("port %d not ready in time", localPort)
	}
}

var errPortForwardStopped = errors.New("port-forward stopped")

func waitPortForwardEnd(ctx context.Context, stopCh <-chan struct{}, errCh <-chan error, stopSession func()) error {
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
		return fmt.Errorf("port-forward ended")
	case <-stopCh:
		stopSession()
		return errPortForwardStopped
	case <-ctx.Done():
		stopSession()
		return ctx.Err()
	}
}

func runPortForwardSession(ctx context.Context, stopCh <-chan struct{}, firstReady chan<- struct{}, readyOnce *sync.Once, kubeContext, namespace, resource string, localPort, remotePort int) (bool, error) {
	podName, err := resolveForwardPod(ctx, kubeContext, namespace, resource)
	if err != nil {
		return false, err
	}
	sessionStop := make(chan struct{})
	readyCh := make(chan struct{})
	var errOut bytes.Buffer
	pf, err := newPortForwarder(kubeContext, namespace, podName, localPort, remotePort, sessionStop, readyCh, &errOut)
	if err != nil {
		return false, err
	}
	defer pf.Close()
	stopSession := func() { closeOnce(sessionStop) }

	errCh := make(chan error, 1)
	go func() { errCh <- pf.ForwardPorts() }()

	if err := waitPortForwardReady(ctx, stopCh, readyCh, errCh, &errOut, localPort, stopSession); err != nil {
		if errors.Is(err, errPortForwardStopped) {
			return true, nil
		}
		return false, err
	}
	readyOnce.Do(func() { close(firstReady) })

	if err := waitPortForwardEnd(ctx, stopCh, errCh, stopSession); err != nil {
		if errors.Is(err, errPortForwardStopped) {
			return true, nil
		}
		return true, err
	}
	return true, nil
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
