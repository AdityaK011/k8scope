# k8scope

A hosted MCP server that lets AI assistants (Claude Code, Cursor, etc.) interact with your GKE clusters using **your own Google identity**. No shared service accounts, no manual token passing — you log in once via browser and the server handles everything.

## How it works

1. You connect Claude Code to the k8scope server URL
2. First time, a browser opens → you log in with Google
3. k8scope stores your tokens server-side and issues a session ID
4. Claude Code sends the session ID on every MCP request
5. k8scope uses your Google access token to call the GKE API
6. All K8s operations run as **your IAM identity** with your RBAC permissions

## Prerequisites

- Go 1.23+
- A Google Cloud project with the GKE API enabled
- An OAuth 2.0 client ID (Web Application type) from Google Cloud Console

### Create the OAuth client

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. Create OAuth Client ID → **Web Application**
3. Add authorized redirect URI: `https://your-domain.com/callback`
4. Save the Client ID and Client Secret

## Run locally

```bash
export GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
export GOOGLE_CLIENT_SECRET=your-client-secret
export REDIRECT_URL=http://localhost:8080/callback
export PORT=8080

go run ./cmd/server
```

## Connect from Claude Code

```bash
claude mcp add --transport http k8scope http://localhost:8080/mcp
```

Then use it:

```
> list all clusters in project my-gcp-project
> show me crashing pods in namespace payments on cluster prod-us
> get logs from pod api-gateway-xyz in namespace default on cluster dev
```

## Deploy to Cloud Run

```bash
# Build and push
docker build -t gcr.io/YOUR_PROJECT/k8scope .
docker push gcr.io/YOUR_PROJECT/k8scope

# Deploy
gcloud run deploy k8scope \
  --image gcr.io/YOUR_PROJECT/k8scope \
  --set-env-vars "GOOGLE_CLIENT_ID=xxx,GOOGLE_CLIENT_SECRET=xxx,REDIRECT_URL=https://k8scope-xxx.run.app/callback" \
  --allow-unauthenticated \
  --port 8080
```

Update the OAuth client's redirect URI to match the Cloud Run URL.

## Available tools (20)

| Tool | Description |
|------|-------------|
| `list_clusters` | List all GKE clusters in a project |
| `list_pods` | List pods with status, restarts, age |
| `describe_pod` | Detailed pod info: conditions, containers, resources |
| `get_pod_logs` | Tail logs from a pod's container |
| `get_events` | Recent K8s events sorted by time |
| `get_nodes` | Node status, version, capacity, zone |
| `list_namespaces` | List all namespaces in a cluster |
| `list_deployments` | Deployments with ready/desired replica counts |
| `describe_deployment` | Detailed deployment: strategy, conditions, containers |
| `list_services` | Services with type, cluster IP, ports |
| `list_ingresses` | Ingresses with hosts and paths |
| `list_jobs` | Jobs with completion and failure counts |
| `list_hpa` | Horizontal pod autoscalers with scaling targets |
| `list_pvcs` | Persistent volume claims with status and capacity |
| `list_configmaps` | Config maps with key counts |
| `list_statefulsets` | Stateful sets with replica status |
| `list_daemonsets` | Daemon sets with scheduling status |
| `list_crds` | Custom resource definitions in the cluster |
| `get_crd_instances` | Instances of any CRD by group/version/resource |
| `get_resource_yaml` | Full YAML of any Kubernetes resource |

## Architecture

```
Claude Code ──Bearer: session_id──▶ k8scope MCP Server ──Bearer: ya29.xxx──▶ GKE API Server
                                         │
                                         ├── OAuth flow (one-time)
                                         ├── Session store (in-memory)
                                         └── Token refresh (automatic)
```

## Project structure

```
k8scope/
├── cmd/server/main.go           # Entrypoint, wires OAuth + MCP, graceful shutdown
├── internal/
│   ├── auth/
│   │   ├── oauth.go             # OAuth flow, Dynamic Client Registration, token refresh
│   │   ├── session.go           # In-memory store (sessions, clients, codes)
│   │   ├── middleware.go         # Bearer token extraction, session injection
│   │   └── ratelimit.go         # Per-IP rate limiting for auth endpoints
│   ├── k8s/
│   │   └── client.go            # Typed + dynamic K8s client, cluster cache
│   └── tools/
│       ├── tools.go             # Original 6 tools + helpers
│       └── tools_extended.go    # 14 new tools (deployments, CRDs, YAML, etc.)
├── Dockerfile
├── go.mod
└── README.md
```
