package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AdityaK011/kubelens/internal/k8s"
	"github.com/AdityaK011/kubelens/pkg/tools"
)

// Server is the KubeLens MCP HTTP server.
type Server struct {
	httpServer *http.Server
}

// ── MCP JSON-RPC types ────────────────────────────────────────

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// New creates the HTTP server on the given address.
func New(addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
	}

	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/sse", s.handleSSE)

	return s
}

// ListenAndServe starts the HTTP server. Blocks until the server stops.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// ── POST /mcp — stateless JSON-RPC ───────────────────────────

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}

	resp := s.handle(req)
	writeJSON(w, resp)
}

// ── GET /sse — server-sent events ────────────────────────────

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	// SSE clients send JSON-RPC messages as the request body or via a POST
	// to /mcp. This endpoint streams responses. For MCP SSE, we keep the
	// connection open and the client sends requests via POST /mcp with a
	// session ID. Here we provide the endpoint URL as an SSE event.
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	flusher.Flush()

	// Keep connection alive until client disconnects
	<-r.Context().Done()
}

// ── Request handling ─────────────────────────────────────────

func (s *Server) handle(req request) response {
	switch req.Method {

	case "initialize":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]string{"name": "kubelens", "version": "0.2.0"},
				"capabilities":    map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
			},
		}

	case "notifications/initialized":
		return response{JSONRPC: "2.0", ID: req.ID}

	case "tools/list":
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools.All()},
		}

	case "tools/call":
		var p toolCallParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := dispatch(ctx, p.Name, p.Arguments)
		if err != nil {
			return response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"content": []map[string]string{{"type": "text", "text": err.Error()}},
					"isError": true,
				},
			}
		}

		text, _ := json.MarshalIndent(result, "", "  ")
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": string(text)}},
			},
		}

	default:
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

// ── Tool dispatch ────────────────────────────────────────────

func dispatch(ctx context.Context, name string, args json.RawMessage) (interface{}, error) {
	var a map[string]interface{}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &a)
	}

	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	int64val := func(key string, def int64) int64 {
		if v, ok := a[key].(float64); ok {
			return int64(v)
		}
		return def
	}
	boolval := func(key string) bool {
		v, _ := a[key].(bool)
		return v
	}

	// Build a per-request k8s client from the auth args
	apiServer := str("api_server")
	token := str("token")
	if apiServer == "" || token == "" {
		return nil, fmt.Errorf("api_server and token are required")
	}

	client, err := k8s.GetOrCreateClient(apiServer, token, str("ca_cert"), boolval("insecure"))
	if err != nil {
		return nil, fmt.Errorf("building k8s client: %w", err)
	}

	switch name {
	case "list_namespaces":
		return client.ListNamespaces(ctx)

	case "list_pods":
		return client.ListPods(ctx, str("namespace"))

	case "get_pod_logs":
		return client.GetPodLogs(ctx, str("namespace"), str("pod"), int64val("lines", 100))

	case "get_events":
		return client.GetEvents(ctx, str("namespace"))

	case "list_deployments":
		return client.ListDeployments(ctx, str("namespace"))

	case "list_nodes":
		return client.ListNodes(ctx)

	case "list_crds":
		return client.ListCRDs(ctx)

	case "get_crd_instances":
		return client.GetCRDInstances(ctx, str("group"), str("version"), str("resource"))

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// ── Helpers ──────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
