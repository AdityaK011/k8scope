# KubeLens

> Natural language Kubernetes debugging via MCP — ask questions about your clusters directly from Claude Code or Cursor.

## What it does

KubeLens is an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server in Go that exposes read-only Kubernetes APIs as LLM-callable tools. Connect it to Claude Code or Cursor and debug your clusters in plain English — no kubectl flags, no API group lookups.

**Example queries:**
- *"Why are my pods crashing in the payments namespace?"*
- *"Are there any nodes under memory pressure in the prod cluster?"*
- *"List all CRDs installed in staging"*
- *"Show me warning events in the checkout namespace"*
- *"What's the replica status of all deployments in the api namespace?"*

## Architecture

```
Claude Code / Cursor
        │  MCP protocol (SSE + JSON-RPC 2.0 over HTTP)
        ▼
  KubeLens MCP Server (Go, :8080)
        │  GET /sse   — long-lived SSE connection (server → client)
        │  POST /mcp  — stateless JSON-RPC endpoint (client → server)
        │
        │  Per-request auth: (api_server, token, ca_cert)
        │  client-go with static bearer token
        ▼
  Any Kubernetes Cluster (GKE, EKS, AKS, self-hosted)
```

## Available Tools

| Tool | Description |
|------|-------------|
| `list_namespaces` | List all namespaces in a cluster |
| `list_pods` | Pods with phase, readiness, restart count, and node |
| `get_pod_logs` | Last N log lines from a specific pod (default: 100) |
| `get_events` | Warning events in a namespace (OOMKills, probe failures, etc.) |
| `list_deployments` | Deployments with desired vs ready vs available replicas |
| `list_nodes` | Nodes with health conditions (Ready, MemoryPressure, DiskPressure) |
| `list_crds` | All Custom Resource Definitions installed in a cluster |
| `get_crd_instances` | Instances of any CRD by group/version/resource |

All operations are **read-only**. KubeLens never modifies cluster state.

## Prerequisites

- Go 1.22+
- A Kubernetes cluster with API server access
- A bearer token for authentication (e.g., `gcloud auth print-access-token` for GKE)

## Setup

```bash
# 1. Clone and build
git clone https://github.com/AdityaK011/kubelens
cd kubelens
go mod tidy
go build -o kubelens ./cmd/kubelens

# 2. Run the server (default port 8080, configurable via PORT env var)
./kubelens
# Prints: KubeLens MCP server listening on :8080
```

## Connect to Claude Code

Add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "kubelens": {
      "type": "sse",
      "url": "http://localhost:8080/sse"
    }
  }
}
```

## Connect to Cursor

Add to `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "kubelens": {
      "type": "sse",
      "url": "http://localhost:8080/sse"
    }
  }
}
```

## Authentication

KubeLens uses **per-request authentication**. Every tool call requires:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `api_server` | Yes | Kubernetes API server URL (e.g., `https://34.84.197.216`) |
| `token` | Yes | Bearer token for API authentication |
| `ca_cert` | No | Base64-encoded PEM CA certificate for TLS verification |
| `insecure` | No | Skip TLS verification (default: false) |

This design makes the server **stateless and cluster-agnostic** — each request can target a different cluster. The server has no kubeconfig dependency.

**Getting a token by cloud provider:**

```bash
# GKE
gcloud auth print-access-token

# EKS
aws eks get-token --cluster-name <name> --output json | jq -r '.status.token'

# AKS
az account get-access-token --query accessToken -o tsv
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
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubelens-reader
subjects:
- kind: User
  name: your-account@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: kubelens-reader
  apiGroup: rbac.authorization.k8s.io
```

## Project Structure

```
kubelens/
├── cmd/kubelens/         # Entrypoint, graceful shutdown
├── internal/
│   ├── server/           # MCP JSON-RPC server (SSE + HTTP transport)
│   └── k8s/              # Kubernetes client with per-request auth and caching
└── pkg/tools/            # MCP tool definitions (JSON schemas)
```

## Roadmap

- [ ] TLS support for hosted deployments
- [ ] Helm release inspection
- [ ] Resource requests vs actual usage (right-sizing hints)
- [ ] Pagination for large list responses
