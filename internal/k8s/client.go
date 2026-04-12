package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	container "google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// cachedCluster holds a GKE cluster's endpoint and CA, which rarely change.
type cachedCluster struct {
	endpoint string
	ca       []byte
	cachedAt time.Time
}

var (
	clusterCacheMu sync.RWMutex
	clusterCache   = make(map[string]*cachedCluster)
	clusterCacheTTL = 10 * time.Minute
)

// ClusterInfo holds the GKE cluster coordinates.
// The user passes these as tool parameters.
type ClusterInfo struct {
	Project  string
	Location string
	Name     string
}

// NewClientForUser builds a Kubernetes clientset authenticated as the user.
//
// The access token is the user's Google OAuth token obtained during login.
// It is sent directly to the GKE API server as a Bearer token.
// The user's IAM roles determine what K8s operations are permitted.
func NewClientForUser(ctx context.Context, accessToken string, cluster ClusterInfo) (*kubernetes.Clientset, error) {
	endpoint, ca, err := getCachedClusterDetails(ctx, accessToken, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster details: %w", err)
	}

	config := &rest.Config{
		Host:        "https://" + endpoint,
		BearerToken: accessToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}

	return kubernetes.NewForConfig(config)
}

// getCachedClusterDetails returns the cluster endpoint and CA from cache,
// or fetches from GKE API and caches for 10 minutes.
func getCachedClusterDetails(ctx context.Context, accessToken string, c ClusterInfo) (string, []byte, error) {
	key := fmt.Sprintf("%s/%s/%s", c.Project, c.Location, c.Name)

	clusterCacheMu.RLock()
	if cached, ok := clusterCache[key]; ok && time.Since(cached.cachedAt) < clusterCacheTTL {
		clusterCacheMu.RUnlock()
		return cached.endpoint, cached.ca, nil
	}
	clusterCacheMu.RUnlock()

	// Cache miss or expired — fetch from GKE API.
	gkeCluster, err := getClusterDetails(ctx, accessToken, c)
	if err != nil {
		return "", nil, err
	}

	ca, err := base64.StdEncoding.DecodeString(gkeCluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode cluster CA: %w", err)
	}

	clusterCacheMu.Lock()
	clusterCache[key] = &cachedCluster{
		endpoint: gkeCluster.Endpoint,
		ca:       ca,
		cachedAt: time.Now(),
	}
	clusterCacheMu.Unlock()

	return gkeCluster.Endpoint, ca, nil
}

// ListClusters returns all GKE clusters the user can see in a project.
func ListClusters(ctx context.Context, accessToken, project string) ([]*container.Cluster, error) {
	svc, err := newGKEService(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	parent := fmt.Sprintf("projects/%s/locations/-", project)
	resp, err := svc.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}

	return resp.Clusters, nil
}

func getClusterDetails(ctx context.Context, accessToken string, c ClusterInfo) (*container.Cluster, error) {
	svc, err := newGKEService(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("projects/%s/locations/%s/clusters/%s",
		c.Project, c.Location, c.Name)

	cluster, err := svc.Projects.Locations.Clusters.Get(name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get cluster %q: %w", c.Name, err)
	}

	return cluster, nil
}

func newGKEService(ctx context.Context, accessToken string) (*container.Service, error) {
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	svc, err := container.NewService(ctx, option.WithTokenSource(src))
	if err != nil {
		return nil, fmt.Errorf("create GKE service: %w", err)
	}
	return svc, nil
}
