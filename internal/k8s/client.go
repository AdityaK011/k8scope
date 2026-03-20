package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client holds typed and dynamic Kubernetes clients for a single API server.
// Built per-request from (apiServer, token).
type Client struct {
	typed   kubernetes.Interface
	dynamic dynamic.Interface
}

// NewClient creates a Client from the given API server URL and bearer token.
// If caCert is non-empty, it is expected to be a base64-encoded PEM CA certificate.
// If insecure is true, TLS verification is skipped (not recommended for production).
func NewClient(apiServer, token, caCert string, insecure bool) (*Client, error) {
	cfg := &rest.Config{
		Host:        apiServer,
		BearerToken: token,
	}

	if caCert != "" {
		decoded, err := base64.StdEncoding.DecodeString(caCert)
		if err != nil {
			return nil, fmt.Errorf("decoding ca_cert: %w", err)
		}
		cfg.TLSClientConfig = rest.TLSClientConfig{CAData: decoded}
	} else if insecure {
		cfg.TLSClientConfig = rest.TLSClientConfig{Insecure: true}
	}

	ts, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building typed client: %w", err)
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client: %w", err)
	}

	return &Client{typed: ts, dynamic: dc}, nil
}

// ── Client cache ──────────────────────────────────────────────

type cacheEntry struct {
	client  *Client
	created time.Time
}

var (
	clientCache   = make(map[string]*cacheEntry)
	clientCacheMu sync.Mutex
	cacheTTL      = 5 * time.Minute
)

// GetOrCreateClient returns a cached Client or creates a new one.
// Cache key is (apiServer, sha256(token)).
func GetOrCreateClient(apiServer, token, caCert string, insecure bool) (*Client, error) {
	h := sha256.Sum256([]byte(token))
	key := apiServer + "|" + fmt.Sprintf("%x", h[:8])

	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	if entry, ok := clientCache[key]; ok && time.Since(entry.created) < cacheTTL {
		return entry.client, nil
	}

	c, err := NewClient(apiServer, token, caCert, insecure)
	if err != nil {
		return nil, err
	}
	clientCache[key] = &cacheEntry{client: c, created: time.Now()}
	return c, nil
}

// ── Tool implementations ──────────────────────────────────────

// ListNamespaces returns all namespace names.
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, ns := range list.Items {
		names[i] = ns.Name
	}
	return names, nil
}

// ListPods returns a concise summary of pods in the given namespace.
func (c *Client) ListPods(ctx context.Context, namespace string) (interface{}, error) {
	list, err := c.typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	type podSummary struct {
		Name     string `json:"name"`
		Phase    string `json:"phase"`
		Ready    bool   `json:"ready"`
		Restarts int32  `json:"restarts"`
		Node     string `json:"node"`
		Message  string `json:"message,omitempty"`
	}

	out := make([]podSummary, len(list.Items))
	for i, p := range list.Items {
		restarts := int32(0)
		ready := false
		for _, cs := range p.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.Ready {
				ready = true
			}
		}
		out[i] = podSummary{
			Name:     p.Name,
			Phase:    string(p.Status.Phase),
			Ready:    ready,
			Restarts: restarts,
			Node:     p.Spec.NodeName,
			Message:  p.Status.Message,
		}
	}
	return out, nil
}

// GetPodLogs returns the last `lines` log lines for a pod.
func (c *Client) GetPodLogs(ctx context.Context, namespace, pod string, lines int64) (string, error) {
	req := c.typed.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		TailLines: &lines,
	})
	raw, err := req.Do(ctx).Raw()
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// GetEvents returns warning events for a namespace.
func (c *Client) GetEvents(ctx context.Context, namespace string) (interface{}, error) {
	list, err := c.typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		return nil, err
	}

	type eventSummary struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
		Object  string `json:"object"`
		Count   int32  `json:"count"`
	}
	out := make([]eventSummary, len(list.Items))
	for i, e := range list.Items {
		out[i] = eventSummary{
			Reason:  e.Reason,
			Message: e.Message,
			Object:  fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Count:   e.Count,
		}
	}
	return out, nil
}

// ListDeployments returns deployments with replica status.
func (c *Client) ListDeployments(ctx context.Context, namespace string) (interface{}, error) {
	list, err := c.typed.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	type deploymentSummary struct {
		Name      string `json:"name"`
		Desired   int32  `json:"desired"`
		Ready     int32  `json:"ready"`
		Available int32  `json:"available"`
	}
	out := make([]deploymentSummary, len(list.Items))
	for i, d := range list.Items {
		out[i] = deploymentSummary{
			Name:      d.Name,
			Desired:   *d.Spec.Replicas,
			Ready:     d.Status.ReadyReplicas,
			Available: d.Status.AvailableReplicas,
		}
	}
	return out, nil
}

// ListNodes returns all nodes with their health conditions.
func (c *Client) ListNodes(ctx context.Context) (interface{}, error) {
	list, err := c.typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	type nodeSummary struct {
		Name       string            `json:"name"`
		Conditions map[string]string `json:"conditions"`
	}
	out := make([]nodeSummary, len(list.Items))
	for i, n := range list.Items {
		conds := make(map[string]string)
		for _, c := range n.Status.Conditions {
			conds[string(c.Type)] = string(c.Status)
		}
		out[i] = nodeSummary{Name: n.Name, Conditions: conds}
	}
	return out, nil
}

// ListCRDs returns the names of all CRDs installed in the cluster.
func (c *Client) ListCRDs(ctx context.Context) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	list, err := c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.GetName()
	}
	return names, nil
}

// GetCRDInstances returns all instances of a given CRD across all namespaces.
func (c *Client) GetCRDInstances(ctx context.Context, group, version, resource string) (interface{}, error) {
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	list, err := c.dynamic.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(list.Items))
	for i, item := range list.Items {
		result[i] = item.Object
	}
	return result, nil
}
