package tools

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/AdityaK011/kubelens/internal/auth"
	k8sClient "github.com/AdityaK011/kubelens/internal/k8s"
)

// Register adds all MCP tools to the server.
func Register(s *server.MCPServer) {
	s.AddTool(listClustersTool(), handleListClusters)
	s.AddTool(listPodsTool(), handleListPods)
	s.AddTool(describePodTool(), handleDescribePod)
	s.AddTool(getPodLogsTool(), handleGetPodLogs)
	s.AddTool(getEventsTool(), handleGetEvents)
	s.AddTool(getNodesTool(), handleGetNodes)
}

// --- Tool definitions ---

func listClustersTool() mcp.Tool {
	return mcp.NewTool("list_clusters",
		mcp.WithDescription("List all GKE clusters the authenticated user has access to in a project"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
	)
}

func listPodsTool() mcp.Tool {
	return mcp.NewTool("list_pods",
		mcp.WithDescription("List pods in a GKE cluster. Returns name, namespace, status, restarts, and age."),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone, e.g. us-central1")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all namespaces.")),
	)
}

func describePodTool() mcp.Tool {
	return mcp.NewTool("describe_pod",
		mcp.WithDescription("Get detailed status, conditions, and events for a specific pod"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Pod namespace")),
		mcp.WithString("pod", mcp.Required(), mcp.Description("Pod name")),
	)
}

func getPodLogsTool() mcp.Tool {
	return mcp.NewTool("get_pod_logs",
		mcp.WithDescription("Get recent logs from a pod's container"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Pod namespace")),
		mcp.WithString("pod", mcp.Required(), mcp.Description("Pod name")),
		mcp.WithString("container", mcp.Description("Container name. Omit if pod has one container.")),
		mcp.WithNumber("tail_lines", mcp.Description("Number of lines from the end. Default 100.")),
	)
}

func getEventsTool() mcp.Tool {
	return mcp.NewTool("get_events",
		mcp.WithDescription("Get recent Kubernetes events, sorted by timestamp"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("Filter events to this namespace. Omit for all.")),
	)
}

func getNodesTool() mcp.Tool {
	return mcp.NewTool("get_nodes",
		mcp.WithDescription("List cluster nodes with status, roles, version, and resource capacity"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
	)
}

// --- Helpers ---

func str(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func num(args map[string]interface{}, key string, def float64) int64 {
	if v, ok := args[key].(float64); ok {
		return int64(v)
	}
	return int64(def)
}

var (
	projectRe  = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	locationRe = regexp.MustCompile(`^[a-z]+-[a-z]+\d+(-[a-z])?$`)
	nameRe     = regexp.MustCompile(`^[a-z][a-z0-9-]{0,38}[a-z0-9]$`)
)

func clusterInfo(args map[string]interface{}) (k8sClient.ClusterInfo, error) {
	project := str(args, "project")
	location := str(args, "location")
	name := str(args, "cluster")

	if !projectRe.MatchString(project) {
		return k8sClient.ClusterInfo{}, fmt.Errorf("invalid project ID %q", project)
	}
	if !locationRe.MatchString(location) {
		return k8sClient.ClusterInfo{}, fmt.Errorf("invalid location %q", location)
	}
	if !nameRe.MatchString(name) {
		return k8sClient.ClusterInfo{}, fmt.Errorf("invalid cluster name %q", name)
	}

	return k8sClient.ClusterInfo{
		Project:  project,
		Location: location,
		Name:     name,
	}, nil
}

func errResult(format string, a ...interface{}) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(fmt.Sprintf(format, a...)), nil
}

// --- Handlers ---

func handleListClusters(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	project := str(req.GetArguments(), "project")

	if !projectRe.MatchString(project) {
		return errResult("invalid project ID %q", project)
	}

	clusters, err := k8sClient.ListClusters(ctx, session.AccessToken, project)
	if err != nil {
		return errResult("failed to list clusters: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d clusters in project %s:\n\n", len(clusters), project))
	sb.WriteString(fmt.Sprintf("%-25s %-15s %-12s %-10s %s\n",
		"NAME", "LOCATION", "VERSION", "STATUS", "NODES"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, c := range clusters {
		sb.WriteString(fmt.Sprintf("%-25s %-15s %-12s %-10s %d\n",
			c.Name, c.Location, c.CurrentMasterVersion, c.Status, c.CurrentNodeCount))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleListPods(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	args := req.GetArguments()
	namespace := str(args, "namespace")

	ci, err := clusterInfo(args)
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return errResult("auth/connect failed: %v", err)
	}

	const podLimit = 500
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{Limit: podLimit})
	if err != nil {
		return errResult("k8s error: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d pods", len(pods.Items)))
	if pods.Continue != "" {
		sb.WriteString(" (more available — filter by namespace to narrow results)")
	}
	sb.WriteString(":\n\n")
	sb.WriteString(fmt.Sprintf("%-45s %-12s %-10s %-8s %s\n",
		"NAMESPACE/NAME", "STATUS", "REASON", "RESTARTS", "AGE"))
	sb.WriteString(strings.Repeat("-", 95) + "\n")

	for _, pod := range pods.Items {
		restarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}
		age := time.Since(pod.CreationTimestamp.Time).Truncate(time.Minute)
		reason := string(pod.Status.Phase)
		if pod.Status.Reason != "" {
			reason = pod.Status.Reason
		}

		sb.WriteString(fmt.Sprintf("%-45s %-12s %-10s %-8d %s\n",
			pod.Namespace+"/"+pod.Name,
			pod.Status.Phase,
			reason,
			restarts,
			age.String(),
		))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleDescribePod(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	args := req.GetArguments()
	namespace := str(args, "namespace")
	podName := str(args, "pod")

	ci, err := clusterInfo(args)
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return errResult("auth/connect failed: %v", err)
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return errResult("k8s error: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pod: %s/%s\n", pod.Namespace, pod.Name))
	sb.WriteString(fmt.Sprintf("Node: %s\n", pod.Spec.NodeName))
	sb.WriteString(fmt.Sprintf("Status: %s\n", pod.Status.Phase))
	sb.WriteString(fmt.Sprintf("IP: %s\n", pod.Status.PodIP))
	sb.WriteString(fmt.Sprintf("Service Account: %s\n", pod.Spec.ServiceAccountName))
	sb.WriteString(fmt.Sprintf("Created: %s\n\n", pod.CreationTimestamp.Format(time.RFC3339)))

	// Conditions
	sb.WriteString("Conditions:\n")
	for _, c := range pod.Status.Conditions {
		sb.WriteString(fmt.Sprintf("  %-22s %-8s %s\n", c.Type, c.Status, c.Message))
	}
	sb.WriteString("\n")

	// Container statuses
	sb.WriteString("Containers:\n")
	for _, cs := range pod.Status.ContainerStatuses {
		sb.WriteString(fmt.Sprintf("  %s:\n", cs.Name))
		sb.WriteString(fmt.Sprintf("    Image:    %s\n", cs.Image))
		sb.WriteString(fmt.Sprintf("    Ready:    %t\n", cs.Ready))
		sb.WriteString(fmt.Sprintf("    Restarts: %d\n", cs.RestartCount))

		if cs.State.Waiting != nil {
			sb.WriteString(fmt.Sprintf("    State:    Waiting (%s: %s)\n",
				cs.State.Waiting.Reason, cs.State.Waiting.Message))
		} else if cs.State.Running != nil {
			sb.WriteString(fmt.Sprintf("    State:    Running (since %s)\n",
				cs.State.Running.StartedAt.Format(time.RFC3339)))
		} else if cs.State.Terminated != nil {
			sb.WriteString(fmt.Sprintf("    State:    Terminated (%s, exit %d)\n",
				cs.State.Terminated.Reason, cs.State.Terminated.ExitCode))
		}
	}

	// Resource requests/limits
	sb.WriteString("\nResource Requests/Limits:\n")
	for _, c := range pod.Spec.Containers {
		sb.WriteString(fmt.Sprintf("  %s:\n", c.Name))
		if c.Resources.Requests != nil {
			sb.WriteString(fmt.Sprintf("    Requests: cpu=%s, memory=%s\n",
				c.Resources.Requests.Cpu().String(),
				c.Resources.Requests.Memory().String()))
		}
		if c.Resources.Limits != nil {
			sb.WriteString(fmt.Sprintf("    Limits:   cpu=%s, memory=%s\n",
				c.Resources.Limits.Cpu().String(),
				c.Resources.Limits.Memory().String()))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleGetPodLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	args := req.GetArguments()
	namespace := str(args, "namespace")
	podName := str(args, "pod")
	containerName := str(args, "container")
	tailLines := num(args, "tail_lines", 100)

	ci, err := clusterInfo(args)
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return errResult("auth/connect failed: %v", err)
	}

	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}
	if containerName != "" {
		opts.Container = containerName
	}

	stream, err := client.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return errResult("failed to get logs: %v", err)
	}
	defer stream.Close()

	const maxLogBytes = 1 << 20 // 1 MB
	logs, err := io.ReadAll(io.LimitReader(stream, maxLogBytes))
	if err != nil {
		return errResult("failed to read log stream: %v", err)
	}

	if len(logs) == 0 {
		return mcp.NewToolResultText("(no logs available)"), nil
	}

	return mcp.NewToolResultText(string(logs)), nil
}

func handleGetEvents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	args := req.GetArguments()
	namespace := str(args, "namespace")

	ci, err := clusterInfo(args)
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return errResult("auth/connect failed: %v", err)
	}

	events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{Limit: 200})
	if err != nil {
		return errResult("k8s error: %v", err)
	}

	// Sort by last timestamp (most recent first) — the API doesn't guarantee order.
	items := events.Items
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastTimestamp.Time.After(items[j].LastTimestamp.Time)
	})

	// Show at most 50 events.
	limit := 50
	if len(items) < limit {
		limit = len(items)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Showing %d of %d events:\n\n", limit, len(items)))
	sb.WriteString(fmt.Sprintf("%-22s %-10s %-30s %-20s %s\n",
		"TIME", "TYPE", "OBJECT", "REASON", "MESSAGE"))
	sb.WriteString(strings.Repeat("-", 120) + "\n")

	for _, e := range items[:limit] {
		ts := e.LastTimestamp.Format("2006-01-02 15:04:05")
		obj := e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name
		msg := e.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-22s %-10s %-30s %-20s %s\n",
			ts, e.Type, obj, e.Reason, msg))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleGetNodes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	args := req.GetArguments()

	ci, err := clusterInfo(args)
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return errResult("auth/connect failed: %v", err)
	}

	const nodeLimit = 500
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: nodeLimit})
	if err != nil {
		return errResult("k8s error: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d nodes", len(nodes.Items)))
	if nodes.Continue != "" {
		sb.WriteString(" (truncated)")
	}
	sb.WriteString(":\n\n")
	sb.WriteString(fmt.Sprintf("%-45s %-10s %-15s %-8s %-8s %s\n",
		"NAME", "STATUS", "VERSION", "CPU", "MEM", "ZONE"))
	sb.WriteString(strings.Repeat("-", 110) + "\n")

	for _, node := range nodes.Items {
		status := "NotReady"
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				status = "Ready"
			}
		}
		zone := node.Labels["topology.kubernetes.io/zone"]
		cpu := node.Status.Capacity.Cpu().String()
		mem := node.Status.Capacity.Memory().String()

		sb.WriteString(fmt.Sprintf("%-45s %-10s %-15s %-8s %-8s %s\n",
			node.Name, status, node.Status.NodeInfo.KubeletVersion,
			cpu, mem, zone))
	}

	return mcp.NewToolResultText(sb.String()), nil
}
