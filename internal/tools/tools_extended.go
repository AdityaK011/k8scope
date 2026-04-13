package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/AdityaK011/k8scope/internal/auth"
	k8sClient "github.com/AdityaK011/k8scope/internal/k8s"
)

// --- Tool definitions ---

func listNamespacesTool() mcp.Tool {
	return mcp.NewTool("list_namespaces",
		mcp.WithDescription("List all namespaces in a GKE cluster"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
	)
}

func listDeploymentsTool() mcp.Tool {
	return mcp.NewTool("list_deployments",
		mcp.WithDescription("List deployments with replica status"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func describeDeploymentTool() mcp.Tool {
	return mcp.NewTool("describe_deployment",
		mcp.WithDescription("Get detailed status, conditions, and containers for a deployment"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Deployment namespace")),
		mcp.WithString("deployment", mcp.Required(), mcp.Description("Deployment name")),
	)
}

func listServicesTool() mcp.Tool {
	return mcp.NewTool("list_services",
		mcp.WithDescription("List services with type, cluster IP, and ports"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listIngressesTool() mcp.Tool {
	return mcp.NewTool("list_ingresses",
		mcp.WithDescription("List ingresses with hosts and paths"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listJobsTool() mcp.Tool {
	return mcp.NewTool("list_jobs",
		mcp.WithDescription("List jobs with completion status"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listHPATool() mcp.Tool {
	return mcp.NewTool("list_hpa",
		mcp.WithDescription("List horizontal pod autoscalers with scaling targets"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listPVCsTool() mcp.Tool {
	return mcp.NewTool("list_pvcs",
		mcp.WithDescription("List persistent volume claims with status and capacity"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listConfigMapsTool() mcp.Tool {
	return mcp.NewTool("list_configmaps",
		mcp.WithDescription("List config maps with key counts"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listStatefulSetsTool() mcp.Tool {
	return mcp.NewTool("list_statefulsets",
		mcp.WithDescription("List stateful sets with replica status"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listDaemonSetsTool() mcp.Tool {
	return mcp.NewTool("list_daemonsets",
		mcp.WithDescription("List daemon sets with scheduling status"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for all.")),
	)
}

func listCRDsTool() mcp.Tool {
	return mcp.NewTool("list_crds",
		mcp.WithDescription("List custom resource definitions installed in the cluster"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
	)
}

func getCRDInstancesTool() mcp.Tool {
	return mcp.NewTool("get_crd_instances",
		mcp.WithDescription("List instances of a custom resource by group/version/resource"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("group", mcp.Required(), mcp.Description("API group, e.g. networking.istio.io")),
		mcp.WithString("version", mcp.Required(), mcp.Description("API version, e.g. v1beta1")),
		mcp.WithString("resource", mcp.Required(), mcp.Description("Resource plural, e.g. virtualservices")),
		mcp.WithString("namespace", mcp.Description("K8s namespace. Omit for cluster-scoped.")),
	)
}

func getResourceYAMLTool() mcp.Tool {
	return mcp.NewTool("get_resource_yaml",
		mcp.WithDescription("Get the full YAML of any Kubernetes resource"),
		mcp.WithString("project", mcp.Required(), mcp.Description("GCP project ID")),
		mcp.WithString("location", mcp.Required(), mcp.Description("Cluster region/zone")),
		mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		mcp.WithString("api_version", mcp.Required(), mcp.Description("API version, e.g. apps/v1, v1, networking.k8s.io/v1")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Resource kind, e.g. Deployment, Service, Pod")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Resource name")),
		mcp.WithString("namespace", mcp.Description("Namespace. Omit for cluster-scoped resources.")),
	)
}

// --- Handlers ---

func handleListNamespaces(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	ci, err := clusterInfo(req.GetArguments())
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return safeErr("failed to connect to cluster", err)
	}

	nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d namespaces:\n\n", len(nsList.Items)))
	sb.WriteString(fmt.Sprintf("%-40s %-10s %s\n", "NAME", "STATUS", "AGE"))
	sb.WriteString(strings.Repeat("-", 65) + "\n")
	for _, ns := range nsList.Items {
		age := time.Since(ns.CreationTimestamp.Time).Truncate(time.Minute)
		sb.WriteString(fmt.Sprintf("%-40s %-10s %s\n", ns.Name, ns.Status.Phase, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListDeployments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	deps, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d deployments:\n\n", len(deps.Items)))
	sb.WriteString(fmt.Sprintf("%-45s %-10s %-12s %-10s %s\n", "NAMESPACE/NAME", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")
	for _, d := range deps.Items {
		age := time.Since(d.CreationTimestamp.Time).Truncate(time.Minute)
		desired := int32(1)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		ready := fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired)
		sb.WriteString(fmt.Sprintf("%-45s %-10s %-12d %-10d %s\n",
			d.Namespace+"/"+d.Name, ready, d.Status.UpdatedReplicas, d.Status.AvailableReplicas, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleDescribeDeployment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	args := req.GetArguments()
	namespace := str(args, "namespace")
	depName := str(args, "deployment")
	ci, err := clusterInfo(args)
	if err != nil {
		return errResult("validation: %v", err)
	}
	client, err := k8sClient.NewClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return safeErr("failed to connect to cluster", err)
	}

	d, err := client.AppsV1().Deployments(namespace).Get(ctx, depName, metav1.GetOptions{})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Deployment: %s/%s\n", d.Namespace, d.Name))
	replicas := int32(0)
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	sb.WriteString(fmt.Sprintf("Replicas: %d desired | %d ready | %d up-to-date | %d available\n",
		replicas, d.Status.ReadyReplicas, d.Status.UpdatedReplicas, d.Status.AvailableReplicas))
	sb.WriteString(fmt.Sprintf("Strategy: %s\n", d.Spec.Strategy.Type))
	sb.WriteString(fmt.Sprintf("Created: %s\n\n", d.CreationTimestamp.Format(time.RFC3339)))

	sb.WriteString("Conditions:\n")
	for _, c := range d.Status.Conditions {
		sb.WriteString(fmt.Sprintf("  %-25s %-8s %s\n", c.Type, c.Status, c.Message))
	}

	sb.WriteString("\nContainers:\n")
	for _, c := range d.Spec.Template.Spec.Containers {
		sb.WriteString(fmt.Sprintf("  %s:\n", c.Name))
		sb.WriteString(fmt.Sprintf("    Image: %s\n", c.Image))
		if c.Resources.Requests != nil {
			sb.WriteString(fmt.Sprintf("    Requests: cpu=%s, memory=%s\n",
				c.Resources.Requests.Cpu().String(), c.Resources.Requests.Memory().String()))
		}
		if c.Resources.Limits != nil {
			sb.WriteString(fmt.Sprintf("    Limits:   cpu=%s, memory=%s\n",
				c.Resources.Limits.Cpu().String(), c.Resources.Limits.Memory().String()))
		}
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListServices(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	svcs, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d services:\n\n", len(svcs.Items)))
	sb.WriteString(fmt.Sprintf("%-40s %-12s %-16s %-30s %s\n", "NAMESPACE/NAME", "TYPE", "CLUSTER-IP", "PORTS", "AGE"))
	sb.WriteString(strings.Repeat("-", 115) + "\n")
	for _, s := range svcs.Items {
		age := time.Since(s.CreationTimestamp.Time).Truncate(time.Minute)
		var ports []string
		for _, p := range s.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		portStr := strings.Join(ports, ",")
		if len(portStr) > 28 {
			portStr = portStr[:25] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-40s %-12s %-16s %-30s %s\n",
			s.Namespace+"/"+s.Name, s.Spec.Type, s.Spec.ClusterIP, portStr, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListIngresses(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	ingList, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d ingresses:\n\n", len(ingList.Items)))
	sb.WriteString(fmt.Sprintf("%-40s %-35s %-25s %s\n", "NAMESPACE/NAME", "HOSTS", "PATHS", "AGE"))
	sb.WriteString(strings.Repeat("-", 110) + "\n")
	for _, ing := range ingList.Items {
		age := time.Since(ing.CreationTimestamp.Time).Truncate(time.Minute)
		var hosts, paths []string
		for _, rule := range ing.Spec.Rules {
			hosts = append(hosts, rule.Host)
			if rule.HTTP != nil {
				for _, p := range rule.HTTP.Paths {
					paths = append(paths, p.Path)
				}
			}
		}
		hostStr := strings.Join(hosts, ",")
		pathStr := strings.Join(paths, ",")
		if len(hostStr) > 33 {
			hostStr = hostStr[:30] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-40s %-35s %-25s %s\n",
			ing.Namespace+"/"+ing.Name, hostStr, pathStr, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListJobs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d jobs:\n\n", len(jobs.Items)))
	sb.WriteString(fmt.Sprintf("%-45s %-14s %-10s %-8s %s\n", "NAMESPACE/NAME", "COMPLETIONS", "SUCCEEDED", "FAILED", "AGE"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")
	for _, j := range jobs.Items {
		age := time.Since(j.CreationTimestamp.Time).Truncate(time.Minute)
		desired := int32(1)
		if j.Spec.Completions != nil {
			desired = *j.Spec.Completions
		}
		sb.WriteString(fmt.Sprintf("%-45s %d/%-12d %-10d %-8d %s\n",
			j.Namespace+"/"+j.Name, j.Status.Succeeded, desired, j.Status.Succeeded, j.Status.Failed, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListHPA(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	hpas, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d HPAs:\n\n", len(hpas.Items)))
	sb.WriteString(fmt.Sprintf("%-40s %-20s %-8s %-8s %-8s %s\n", "NAMESPACE/NAME", "REFERENCE", "MIN", "MAX", "CURRENT", "AGE"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")
	for _, h := range hpas.Items {
		age := time.Since(h.CreationTimestamp.Time).Truncate(time.Minute)
		ref := fmt.Sprintf("%s/%s", h.Spec.ScaleTargetRef.Kind, h.Spec.ScaleTargetRef.Name)
		minReplicas := int32(1)
		if h.Spec.MinReplicas != nil {
			minReplicas = *h.Spec.MinReplicas
		}
		sb.WriteString(fmt.Sprintf("%-40s %-20s %-8d %-8d %-8d %s\n",
			h.Namespace+"/"+h.Name, ref, minReplicas, h.Spec.MaxReplicas, h.Status.CurrentReplicas, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListPVCs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d PVCs:\n\n", len(pvcs.Items)))
	sb.WriteString(fmt.Sprintf("%-40s %-10s %-20s %-10s %-15s %s\n", "NAMESPACE/NAME", "STATUS", "VOLUME", "CAPACITY", "STORAGECLASS", "AGE"))
	sb.WriteString(strings.Repeat("-", 115) + "\n")
	for _, p := range pvcs.Items {
		age := time.Since(p.CreationTimestamp.Time).Truncate(time.Minute)
		capacity := ""
		if p.Status.Capacity != nil {
			if storage, ok := p.Status.Capacity["storage"]; ok {
				capacity = storage.String()
			}
		}
		sc := ""
		if p.Spec.StorageClassName != nil {
			sc = *p.Spec.StorageClassName
		}
		sb.WriteString(fmt.Sprintf("%-40s %-10s %-20s %-10s %-15s %s\n",
			p.Namespace+"/"+p.Name, p.Status.Phase, p.Spec.VolumeName, capacity, sc, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListConfigMaps(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	cms, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d config maps:\n\n", len(cms.Items)))
	sb.WriteString(fmt.Sprintf("%-45s %-8s %s\n", "NAMESPACE/NAME", "KEYS", "AGE"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")
	for _, cm := range cms.Items {
		age := time.Since(cm.CreationTimestamp.Time).Truncate(time.Minute)
		sb.WriteString(fmt.Sprintf("%-45s %-8d %s\n",
			cm.Namespace+"/"+cm.Name, len(cm.Data), age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListStatefulSets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	ssets, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d stateful sets:\n\n", len(ssets.Items)))
	sb.WriteString(fmt.Sprintf("%-45s %-10s %s\n", "NAMESPACE/NAME", "READY", "AGE"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")
	for _, s := range ssets.Items {
		age := time.Since(s.CreationTimestamp.Time).Truncate(time.Minute)
		replicas := int32(0)
		if s.Spec.Replicas != nil {
			replicas = *s.Spec.Replicas
		}
		sb.WriteString(fmt.Sprintf("%-45s %d/%-8d %s\n",
			s.Namespace+"/"+s.Name, s.Status.ReadyReplicas, replicas, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListDaemonSets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return safeErr("failed to connect to cluster", err)
	}

	dsets, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d daemon sets:\n\n", len(dsets.Items)))
	sb.WriteString(fmt.Sprintf("%-45s %-10s %-8s %-12s %-10s %s\n", "NAMESPACE/NAME", "DESIRED", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"))
	sb.WriteString(strings.Repeat("-", 105) + "\n")
	for _, d := range dsets.Items {
		age := time.Since(d.CreationTimestamp.Time).Truncate(time.Minute)
		sb.WriteString(fmt.Sprintf("%-45s %-10d %-8d %-12d %-10d %s\n",
			d.Namespace+"/"+d.Name,
			d.Status.DesiredNumberScheduled, d.Status.NumberReady,
			d.Status.UpdatedNumberScheduled, d.Status.NumberAvailable, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleListCRDs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := auth.SessionFromContext(ctx)
	if err != nil {
		return errResult("auth: %v", err)
	}
	ci, err := clusterInfo(req.GetArguments())
	if err != nil {
		return errResult("validation: %v", err)
	}
	dynClient, err := k8sClient.NewDynamicClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return safeErr("failed to connect to cluster", err)
	}

	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	crdList, err := dynClient.Resource(crdGVR).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d CRDs:\n\n", len(crdList.Items)))
	sb.WriteString(fmt.Sprintf("%-55s %-30s %-12s %s\n", "NAME", "GROUP", "SCOPE", "AGE"))
	sb.WriteString(strings.Repeat("-", 110) + "\n")
	for _, item := range crdList.Items {
		name := item.GetName()
		spec, _ := item.Object["spec"].(map[string]interface{})
		group, _ := spec["group"].(string)
		scope, _ := spec["scope"].(string)
		age := time.Since(item.GetCreationTimestamp().Time).Truncate(time.Minute)
		sb.WriteString(fmt.Sprintf("%-55s %-30s %-12s %s\n", name, group, scope, age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleGetCRDInstances(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	group := str(args, "group")
	version := str(args, "version")
	resource := str(args, "resource")
	namespace := str(args, "namespace")

	dynClient, err := k8sClient.NewDynamicClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return safeErr("failed to connect to cluster", err)
	}

	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	var list *unstructured.UnstructuredList
	if namespace != "" {
		list, err = dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	} else {
		list, err = dynClient.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 500})
	}
	if err != nil {
		return safeErr("kubernetes API error", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d %s.%s/%s:\n\n", len(list.Items), resource, group, version))
	sb.WriteString(fmt.Sprintf("%-45s %-30s %s\n", "NAME", "NAMESPACE", "AGE"))
	sb.WriteString(strings.Repeat("-", 85) + "\n")
	for _, item := range list.Items {
		age := time.Since(item.GetCreationTimestamp().Time).Truncate(time.Minute)
		sb.WriteString(fmt.Sprintf("%-45s %-30s %s\n", item.GetName(), item.GetNamespace(), age))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleGetResourceYAML(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	apiVersion := str(args, "api_version")
	kind := str(args, "kind")
	name := str(args, "name")
	namespace := str(args, "namespace")

	if apiVersion == "" || kind == "" || name == "" {
		return errResult("api_version, kind, and name are required")
	}

	dynClient, err := k8sClient.NewDynamicClientForUser(ctx, session.AccessToken, ci)
	if err != nil {
		return safeErr("failed to connect to cluster", err)
	}

	// Parse apiVersion into group + version.
	gv := strings.SplitN(apiVersion, "/", 2)
	var group, version string
	if len(gv) == 1 {
		group = ""
		version = gv[0] // core API: "v1"
	} else {
		group = gv[0]
		version = gv[1]
	}

	// Convert kind to lowercase plural (simple heuristic).
	resource := strings.ToLower(kind) + "s"

	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	var obj *unstructured.Unstructured
	if namespace != "" {
		obj, err = dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = dynClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return safeErr("failed to get resource", err)
	}

	yamlBytes, err := sigsyaml.Marshal(obj.Object)
	if err != nil {
		return safeErr("failed to serialize resource", err)
	}

	return mcp.NewToolResultText(string(yamlBytes)), nil
}
