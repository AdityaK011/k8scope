package k8s

import (
	"context"
	"testing"
	"time"
)

func TestClusterCacheHit(t *testing.T) {
	// Manually insert an entry into the cluster cache.
	key := "test-project/us-central1/test-cluster"
	expectedEndpoint := "10.0.0.1"
	expectedCA := []byte("fake-ca-data")

	clusterCacheMu.Lock()
	clusterCache[key] = &cachedCluster{
		endpoint: expectedEndpoint,
		ca:       expectedCA,
		cachedAt: time.Now(),
	}
	clusterCacheMu.Unlock()

	// Clean up after test.
	defer func() {
		clusterCacheMu.Lock()
		delete(clusterCache, key)
		clusterCacheMu.Unlock()
	}()

	ci := ClusterInfo{
		Project:  "test-project",
		Location: "us-central1",
		Name:     "test-cluster",
	}

	endpoint, ca, err := getCachedClusterDetails(context.Background(), "fake-token", ci)
	if err != nil {
		t.Fatalf("getCachedClusterDetails returned error: %v", err)
	}
	if endpoint != expectedEndpoint {
		t.Errorf("endpoint = %q, want %q", endpoint, expectedEndpoint)
	}
	if string(ca) != string(expectedCA) {
		t.Errorf("ca = %q, want %q", string(ca), string(expectedCA))
	}
}

func TestClusterCacheExpiry(t *testing.T) {
	// Insert an entry that is older than the TTL (11 minutes ago).
	key := "expired-project/us-east1/expired-cluster"
	clusterCacheMu.Lock()
	clusterCache[key] = &cachedCluster{
		endpoint: "old-endpoint",
		ca:       []byte("old-ca"),
		cachedAt: time.Now().Add(-11 * time.Minute),
	}
	clusterCacheMu.Unlock()

	// Clean up after test.
	defer func() {
		clusterCacheMu.Lock()
		delete(clusterCache, key)
		clusterCacheMu.Unlock()
	}()

	ci := ClusterInfo{
		Project:  "expired-project",
		Location: "us-east1",
		Name:     "expired-cluster",
	}

	// The cache entry is expired, so getCachedClusterDetails will attempt
	// to call the GKE API, which will fail because we have a fake token.
	// The important thing is that it did NOT return the stale cached data.
	_, _, err := getCachedClusterDetails(context.Background(), "fake-token", ci)
	if err == nil {
		t.Fatal("expected error when cache is expired and GKE API call fails, got nil")
	}
}
