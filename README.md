# KubeLens

> Natural language Kubernetes debugging via MCP — ask questions about your clusters directly from Claude or Cursor.

## What it does

KubeLens is an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server in Go that exposes read-only Kubernetes APIs as LLM-callable tools. Connect it to Claude Desktop or Cursor and debug your clusters in plain English — no kubectl flags, no API group lookups.

**Example queries:**
- *"Why are my pods crashing in the payments namespace?"*
- *"Are there any nodes under memory pressure in the prod cluster?"*
- *"List all CRDs installed in staging"*
- *"Show me warning events in the checkout namespace"*
- *"What's the replica status of all deployments in the api namespace?"*

## Architecture

```
Claude Desktop / Cursor
        │  MCP protocol (stdio)
        ▼
  KubeLens MCP Server (Go)
        │  GCP Application Default Credentials
        │  client-go
        ▼
  GKE Clusters (via kubeconfig contexts)
```

## Available Tools

| Tool | Description |
|------|-------------|
| `list_clusters` | List all kubeconfig contexts |
| `list_namespaces` | List namespaces in a cluster |
| `list_pods` | Pods with status, restarts, phase |
| `get_pod_logs` | Last N log lines from a pod |
| `get_events` | Warning events in a namespace |
| `list_deployments` | Deployments with replica status |
| `list_nodes` | Nodes with health conditions |
| `list_crds` | All installed CRDs |
| `get_crd_instances` | Instances of any CRD |

All operations are **read-only**. KubeLens never modifies cluster state.

## Prerequisites

- Go 1.22+
- `gcloud` CLI installed
- A valid `~/.kube/config` with GKE cluster contexts

## Setup

```bash
# 1. Authenticate with GCP (only needed once)
gcloud auth application-default login

# 2. Make sure your kubeconfig has the clusters you want
gcloud container clusters get-credentials <cluster-name> --region <region> --project <project>

# 3. Clone and build
git clone https://github.com/AdityaK011/kubelens
cd kubelens
go mod tidy
go build -o kubelens ./cmd/kubelens

# 4. Test it works
./kubelens
# Should print: KubeLens ready — listening on stdio
```

## Connect to Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "kubelens": {
      "command": "/absolute/path/to/kubelens"
    }
  }
}
```

Restart Claude Desktop. You'll see KubeLens appear in the tools list.

## Connect to Cursor

Add to `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "kubelens": {
      "command": "/absolute/path/to/kubelens"
    }
  }
}
```

## Authentication

KubeLens uses **GCP Application Default Credentials (ADC)** — the same credentials `gcloud` and `kubectl` use when talking to GKE. No separate auth configuration needed.

If you're not authenticated, KubeLens will tell you exactly what to run:
```
not authenticated — run: gcloud auth application-default login
```

For least-privilege access, apply this RBAC to your clusters:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubelens-reader
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubelens-reader
subjects:
- kind: User
  name: your-google-account@gmail.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: kubelens-reader
  apiGroup: rbac.authorization.k8s.io
```

## Project Structure

```
kubelens/
├── cmd/kubelens/         # Entrypoint
├── internal/
│   ├── server/           # MCP JSON-RPC server (stdio transport)
│   └── k8s/              # Multi-cluster Kubernetes client manager
└── pkg/tools/            # MCP tool definitions (JSON schemas)
```

## Roadmap

- [ ] Helm release inspection
- [ ] OPA/Gatekeeper policy violation lookup
- [ ] Resource requests vs actual usage (right-sizing hints)
- [ ] Multi-cluster event correlation
