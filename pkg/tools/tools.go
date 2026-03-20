package tools

// Tool represents a single MCP tool definition sent to the LLM.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is a JSON Schema describing the tool's parameters.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a single parameter.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// authProperties returns the common auth + TLS properties for all tools.
func authProperties() map[string]Property {
	return map[string]Property{
		"api_server": {Type: "string", Description: "Kubernetes API server URL (e.g. https://1.2.3.4)"},
		"token":      {Type: "string", Description: "Bearer token for Kubernetes API authentication"},
		"ca_cert":    {Type: "string", Description: "Optional base64-encoded PEM CA certificate for TLS verification"},
		"insecure":   {Type: "boolean", Description: "Skip TLS verification (default: false, not recommended for production)"},
	}
}

// authRequired returns the required fields common to all tools.
var authRequired = []string{"api_server", "token"}

// mergeProps merges auth properties with tool-specific properties.
func mergeProps(extra map[string]Property) map[string]Property {
	props := authProperties()
	for k, v := range extra {
		props[k] = v
	}
	return props
}

// mergeRequired appends tool-specific required fields to the auth required fields.
func mergeRequired(extra ...string) []string {
	req := make([]string, len(authRequired)+len(extra))
	copy(req, authRequired)
	copy(req[len(authRequired):], extra)
	return req
}

// All returns the full list of tools KubeLens exposes to the LLM.
func All() []Tool {
	return []Tool{
		{
			Name:        "list_namespaces",
			Description: "List all namespaces in a Kubernetes cluster.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: mergeProps(nil),
				Required:   authRequired,
			},
		},
		{
			Name:        "list_pods",
			Description: "List all pods in a namespace with their phase, readiness, restart count, and node. Use this to spot CrashLoopBackOff, Pending, or OOMKilled pods.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: mergeProps(map[string]Property{
					"namespace": {Type: "string", Description: "Kubernetes namespace"},
				}),
				Required: mergeRequired("namespace"),
			},
		},
		{
			Name:        "get_pod_logs",
			Description: "Fetch the last N log lines from a specific pod. Use this after list_pods identifies a failing pod to understand the root cause.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: mergeProps(map[string]Property{
					"namespace": {Type: "string", Description: "Kubernetes namespace"},
					"pod":       {Type: "string", Description: "Exact pod name"},
					"lines":     {Type: "integer", Description: "Number of log lines to fetch (default: 100)"},
				}),
				Required: mergeRequired("namespace", "pod"),
			},
		},
		{
			Name:        "get_events",
			Description: "Get warning events in a namespace. Reveals OOMKills, image pull failures, failed scheduling, probe failures, and other Kubernetes-level issues.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: mergeProps(map[string]Property{
					"namespace": {Type: "string", Description: "Kubernetes namespace"},
				}),
				Required: mergeRequired("namespace"),
			},
		},
		{
			Name:        "list_deployments",
			Description: "List deployments in a namespace with desired vs ready vs available replica counts. Use this to check rollout status.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: mergeProps(map[string]Property{
					"namespace": {Type: "string", Description: "Kubernetes namespace"},
				}),
				Required: mergeRequired("namespace"),
			},
		},
		{
			Name:        "list_nodes",
			Description: "List all nodes in a cluster with their health conditions (Ready, MemoryPressure, DiskPressure, PIDPressure). Use this to diagnose cluster-wide scheduling or resource issues.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: mergeProps(nil),
				Required:   authRequired,
			},
		},
		{
			Name:        "list_crds",
			Description: "List all Custom Resource Definitions installed in a cluster. Use this to discover what platform resources are available.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: mergeProps(nil),
				Required:   authRequired,
			},
		},
		{
			Name:        "get_crd_instances",
			Description: "Fetch all instances of a specific CRD. Use this to inspect platform resources like PropagationPolicies, Certificates, HelmReleases, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: mergeProps(map[string]Property{
					"group":    {Type: "string", Description: "API group e.g. policy.karmada.io"},
					"version":  {Type: "string", Description: "API version e.g. v1alpha1"},
					"resource": {Type: "string", Description: "Plural resource name e.g. propagationpolicies"},
				}),
				Required: mergeRequired("group", "version", "resource"),
			},
		},
	}
}
