package tools

import (
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestStr(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		want string
	}{
		{
			name: "key exists with string value",
			args: map[string]interface{}{"project": "my-project"},
			key:  "project",
			want: "my-project",
		},
		{
			name: "key exists with non-string value",
			args: map[string]interface{}{"count": 42},
			key:  "count",
			want: "",
		},
		{
			name: "key missing",
			args: map[string]interface{}{},
			key:  "project",
			want: "",
		},
		{
			name: "nil map value",
			args: map[string]interface{}{"ns": nil},
			key:  "ns",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := str(tt.args, tt.key)
			if got != tt.want {
				t.Errorf("str(%v, %q) = %q, want %q", tt.args, tt.key, got, tt.want)
			}
		})
	}
}

func TestNum(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		def  float64
		want int64
	}{
		{
			name: "key exists with float64 value",
			args: map[string]interface{}{"tail_lines": float64(50)},
			key:  "tail_lines",
			def:  100,
			want: 50,
		},
		{
			name: "key missing returns default",
			args: map[string]interface{}{},
			key:  "tail_lines",
			def:  100,
			want: 100,
		},
		{
			name: "key exists with wrong type returns default",
			args: map[string]interface{}{"tail_lines": "not-a-number"},
			key:  "tail_lines",
			def:  200,
			want: 200,
		},
		{
			name: "zero value",
			args: map[string]interface{}{"lines": float64(0)},
			key:  "lines",
			def:  100,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := num(tt.args, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("num(%v, %q, %v) = %d, want %d", tt.args, tt.key, tt.def, got, tt.want)
			}
		})
	}
}

func TestClusterInfoValid(t *testing.T) {
	args := map[string]interface{}{
		"project":  "my-project-123",
		"location": "us-central1",
		"cluster":  "my-cluster",
	}

	ci, err := clusterInfo(args)
	if err != nil {
		t.Fatalf("clusterInfo returned unexpected error: %v", err)
	}
	if ci.Project != "my-project-123" {
		t.Errorf("Project = %q, want %q", ci.Project, "my-project-123")
	}
	if ci.Location != "us-central1" {
		t.Errorf("Location = %q, want %q", ci.Location, "us-central1")
	}
	if ci.Name != "my-cluster" {
		t.Errorf("Name = %q, want %q", ci.Name, "my-cluster")
	}
}

func TestClusterInfoInvalidProject(t *testing.T) {
	args := map[string]interface{}{
		"project":  "INVALID",
		"location": "us-central1",
		"cluster":  "my-cluster",
	}

	_, err := clusterInfo(args)
	if err == nil {
		t.Fatal("expected error for invalid project, got nil")
	}
	if got := err.Error(); !contains(got, "invalid project") {
		t.Errorf("error = %q, want it to contain %q", got, "invalid project")
	}
}

func TestClusterInfoInvalidLocation(t *testing.T) {
	args := map[string]interface{}{
		"project":  "my-project-123",
		"location": "bad",
		"cluster":  "my-cluster",
	}

	_, err := clusterInfo(args)
	if err == nil {
		t.Fatal("expected error for invalid location, got nil")
	}
	if got := err.Error(); !contains(got, "invalid location") {
		t.Errorf("error = %q, want it to contain %q", got, "invalid location")
	}
}

func TestClusterInfoInvalidCluster(t *testing.T) {
	args := map[string]interface{}{
		"project":  "my-project-123",
		"location": "us-central1",
		"cluster":  "",
	}

	_, err := clusterInfo(args)
	if err == nil {
		t.Fatal("expected error for invalid cluster, got nil")
	}
	if got := err.Error(); !contains(got, "invalid cluster") {
		t.Errorf("error = %q, want it to contain %q", got, "invalid cluster")
	}
}

func TestErrResult(t *testing.T) {
	result, err := errResult("something went wrong: %s", "details")
	if err != nil {
		t.Fatalf("errResult returned non-nil error: %v", err)
	}
	if result == nil {
		t.Fatal("errResult returned nil CallToolResult")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestSafeErr(t *testing.T) {
	testErr := errors.New("connection refused")
	result, err := safeErr("failed to connect", testErr)
	if err != nil {
		t.Fatalf("safeErr returned non-nil error: %v", err)
	}
	if result == nil {
		t.Fatal("safeErr returned nil CallToolResult")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}

	// Verify the result contains the error message.
	found := false
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			if contains(tc.Text, "connection refused") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected result content to contain the error message 'connection refused'")
	}
}

// contains is a small helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
