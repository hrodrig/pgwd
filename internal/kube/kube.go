package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

const discoverPasswordPlaceholder = "DISCOVER_MY_PASSWORD"

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

// ValidateKubernetesAccess lists pods across all namespaces to verify cluster connectivity.
func ValidateKubernetesAccess(ctx context.Context, kubeContext string) error {
	_, clientset, err := getConfigAndClientset(kubeContext)
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	opts := metav1.ListOptions{Limit: 10}
	_, err = clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, opts)
	if err != nil {
		return fmt.Errorf("list pods failed: %w", err)
	}
	// Stream a subset to stdout (mimic kubectl get pods -A brevity)
	pods, _ := clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 20})
	for _, p := range pods.Items {
		fmt.Fprintf(os.Stdout, "%-50s %s\n", p.Namespace+"/"+p.Name, p.Status.Phase)
	}
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

func resolvePodFromService(ctx context.Context, clientset *kubernetes.Clientset, namespace, svcName string) (string, error) {
	// Try endpoints first: get first address targetRef name
	ep, err := clientset.CoreV1().Endpoints(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err == nil && len(ep.Subsets) > 0 && len(ep.Subsets[0].Addresses) > 0 {
		if ref := ep.Subsets[0].Addresses[0].TargetRef; ref != nil && ref.Kind == "Pod" {
			return ref.Name, nil
		}
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

// GetPasswordFromPod reads the given env var from the pod's container.
func GetPasswordFromPod(ctx context.Context, kubeContext, namespace, podName, container, envVar string) (string, error) {
	config, clientset, err := getConfigAndClientset(kubeContext)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(namespace).Name(podName).
		SubResource("exec")
	opts := &corev1.PodExecOptions{
		Command: []string{"printenv", envVar},
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}
	if container != "" {
		opts.Container = container
	}
	req.VersionedParams(opts, scheme.ParameterCodec)
	var buf bytes.Buffer
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", err
	}
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &buf,
		Stderr: &buf,
	})
	if err != nil {
		return "", fmt.Errorf("exec in pod %s: %w", podName, err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "" {
		return out, nil
	}
	if envVar != "PGPASSWORD" {
		return GetPasswordFromPod(ctx, kubeContext, namespace, podName, container, "PGPASSWORD")
	}
	return "", fmt.Errorf("could not find %s or PGPASSWORD in pod %s", envVar, podName)
}

// StartPortForward runs port-forward to localPort:5432 (Postgres).
func StartPortForward(ctx context.Context, kubeContext, namespace, resource string, localPort int) (cleanup func(), err error) {
	return StartPortForwardTo(ctx, kubeContext, namespace, resource, localPort, 5432)
}

// StartPortForwardTo runs port-forward in the background (localPort:remotePort) and waits for the local port to be listening.
func StartPortForwardTo(ctx context.Context, kubeContext, namespace, resource string, localPort, remotePort int) (cleanup func(), err error) {
	podName := resource
	if strings.HasPrefix(resource, "svc/") {
		podName, err = ResolvePod(ctx, kubeContext, namespace, resource)
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(resource, "pod/") {
		podName = strings.TrimPrefix(resource, "pod/")
	} else {
		return nil, fmt.Errorf("resource must be pod/name or svc/name, got %q", resource)
	}

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

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	var errOut bytes.Buffer
	pf, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, remotePort)}, stopCh, readyCh, io.Discard, &errOut)
	if err != nil {
		return nil, fmt.Errorf("create port-forward: %w", err)
	}

	go func() {
		if err := pf.ForwardPorts(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "pgwd: port-forward: %v\n", err)
		}
	}()

	cleanupFn := func() {
		close(stopCh)
		pf.Close()
	}

	select {
	case <-readyCh:
		return cleanupFn, nil
	case <-ctx.Done():
		cleanupFn()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		cleanupFn()
		if errOut.Len() > 0 {
			return nil, fmt.Errorf("port %d did not become ready in time: %s", localPort, errOut.String())
		}
		return nil, fmt.Errorf("port %d did not become ready in time (port-forward may have failed)", localPort)
	}
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
