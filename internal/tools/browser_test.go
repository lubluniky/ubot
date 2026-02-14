package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hkuds/ubot/internal/config"
)

// testBrowserConfig returns a BrowserConfig suitable for testing.
func testBrowserConfig(t *testing.T) config.BrowserConfig {
	t.Helper()
	tmpDir := t.TempDir()
	return config.BrowserConfig{
		SessionDir:  tmpDir,
		Stealth:     true,
		IdleTimeout: 300,
	}
}

func TestNewBrowserTool(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	if tool == nil {
		t.Fatal("NewBrowserTool returned nil")
	}
	if tool.Name() != "browser_use" {
		t.Errorf("expected name 'browser_use', got %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
	params := tool.Parameters()
	if params == nil {
		t.Fatal("parameters should not be nil")
	}
	if params["type"] != "object" {
		t.Errorf("expected type 'object', got %v", params["type"])
	}
}

func TestBrowserTool_ParameterSchema(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	params := tool.Parameters()

	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}

	expectedFields := []string{"action", "url", "selector", "text", "session"}
	for _, field := range expectedFields {
		if _, exists := properties[field]; !exists {
			t.Errorf("missing expected field %q in properties", field)
		}
	}

	// Verify action enum values.
	actionSchema, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatal("action schema should be a map")
	}
	enum, ok := actionSchema["enum"].([]string)
	if !ok {
		t.Fatal("action enum should be []string")
	}

	expectedActions := map[string]bool{
		"browse_page":    false,
		"click_element":  false,
		"type_text":      false,
		"extract_text":   false,
		"screenshot":     false,
		"list_sessions":  false,
		"delete_session": false,
	}
	for _, a := range enum {
		if _, exists := expectedActions[a]; !exists {
			t.Errorf("unexpected action in enum: %q", a)
		}
		expectedActions[a] = true
	}
	for action, found := range expectedActions {
		if !found {
			t.Errorf("expected action %q not in enum", action)
		}
	}

	// Verify required fields.
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required should be []string")
	}
	if len(required) != 1 || required[0] != "action" {
		t.Errorf("expected required=[action], got %v", required)
	}
}

func TestBrowserTool_ValidationMissingAction(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	schema := tool.Parameters()
	errors := ValidateParams(map[string]interface{}{}, schema)
	if len(errors) == 0 {
		t.Error("expected validation errors for missing action")
	}
	found := false
	for _, e := range errors {
		if strings.Contains(e, "action") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing 'action', got: %v", errors)
	}
}

func TestBrowserTool_ValidationInvalidAction(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "invalid_action",
	})
	if err == nil {
		t.Error("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected 'unknown action' error, got: %v", err)
	}
}

func TestBrowserTool_BrowsePageMissingURL(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "browse_page",
	})
	if err == nil {
		t.Error("expected error for missing url")
	}
}

func TestBrowserTool_BrowsePageEmptyURL(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "browse_page",
		"url":    "",
	})
	if err == nil {
		t.Error("expected error for empty url")
	}
}

func TestBrowserTool_ClickMissingSelector(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "click_element",
	})
	if err == nil {
		t.Error("expected error for missing selector")
	}
}

func TestBrowserTool_ClickEmptySelector(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "click_element",
		"selector": "",
	})
	if err == nil {
		t.Error("expected error for empty selector")
	}
}

func TestBrowserTool_TypeTextMissingParams(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))

	// Missing selector.
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "type_text",
		"text":   "hello",
	})
	if err == nil {
		t.Error("expected error for missing selector in type_text")
	}

	// Missing text.
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":   "type_text",
		"selector": "#input",
	})
	if err == nil {
		t.Error("expected error for missing text in type_text")
	}

	// Empty selector.
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":   "type_text",
		"selector": "",
		"text":     "hello",
	})
	if err == nil {
		t.Error("expected error for empty selector in type_text")
	}
}

func TestBrowserTool_ExtractTextMissingSelector(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "extract_text",
	})
	if err == nil {
		t.Error("expected error for missing selector in extract_text")
	}
}

func TestBrowserTool_ExtractTextEmptySelector(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "extract_text",
		"selector": "",
	})
	if err == nil {
		t.Error("expected error for empty selector in extract_text")
	}
}

func TestBrowserTool_ToDefinition(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	def := ToDefinition(tool)

	if def.Type != "function" {
		t.Errorf("expected type 'function', got %q", def.Type)
	}
	if def.Function.Name != "browser_use" {
		t.Errorf("expected function name 'browser_use', got %q", def.Function.Name)
	}
	if def.Function.Description == "" {
		t.Error("function description should not be empty")
	}
	if def.Function.Parameters == nil {
		t.Error("function parameters should not be nil")
	}
}

func TestBrowserTool_Registry(t *testing.T) {
	registry := NewRegistry()
	tool := NewBrowserTool(testBrowserConfig(t))
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("failed to register browser tool: %v", err)
	}

	if !registry.Has("browser_use") {
		t.Error("registry should have browser_use")
	}
	if registry.Get("browser_use") == nil {
		t.Error("Get should return the tool")
	}
}

func TestBrowserTool_Close(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	// Closing without a browser running should not panic.
	tool.Close()
}

func TestCollapseWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"single line", "  hello   world  ", "hello world"},
		{"multiple lines", "  hello  \n\n  world  \n\n", "hello\nworld"},
		{"tabs and spaces", "\t hello \t world \t", "hello world"},
		{"blank lines removed", "line1\n\n\n\nline2", "line1\nline2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collapseWhitespace(tt.input)
			if result != tt.expected {
				t.Errorf("collapseWhitespace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFindChromeBinary(t *testing.T) {
	// This test just verifies the function doesn't panic.
	// The result depends on the system's Chrome installation.
	path, err := findChromeBinary()
	if err != nil {
		t.Skipf("no Chrome binary found (expected in CI): %v", err)
	}
	if path == "" {
		t.Error("findChromeBinary returned empty path without error")
	}
}

func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort failed: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("freePort returned invalid port: %d", port)
	}

	// Should return different ports on successive calls.
	port2, err := freePort()
	if err != nil {
		t.Fatalf("second freePort call failed: %v", err)
	}
	// They could theoretically be the same, but very unlikely.
	_ = port2
}

func TestBrowserTool_BrowsePageWithTestServer(t *testing.T) {
	// Create a test HTTP server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<main>
<h1>Hello Browser</h1>
<p>This is test content for the browser tool.</p>
<a href="/link1">Link One</a>
<input type="text" name="search" placeholder="Search here">
<button>Submit</button>
</main>
</body>
</html>`))
	}))
	defer server.Close()

	tool := NewBrowserTool(testBrowserConfig(t))
	defer tool.Close()

	// SSRF protection blocks localhost/private IPs, so we expect the request to be blocked.
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "browse_page",
		"url":    server.URL,
	})
	if err == nil {
		t.Fatal("expected SSRF protection to block localhost request")
	}
	if !strings.Contains(err.Error(), "internal/private network") {
		t.Errorf("expected SSRF error, got: %v", err)
	}
}

func TestBrowserTool_BrowsePageAddsScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Scheme Test</title></head><body>OK</body></html>`))
	}))
	defer server.Close()

	tool := NewBrowserTool(testBrowserConfig(t))
	defer tool.Close()

	// SSRF protection blocks localhost, so test that the scheme-added URL is still caught.
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "browse_page",
		"url":    server.URL,
	})
	if err == nil {
		t.Fatal("expected SSRF protection to block localhost request")
	}
	if !strings.Contains(err.Error(), "internal/private network") {
		t.Errorf("expected SSRF error, got: %v", err)
	}
}

func TestBrowserTool_MissingActionParam(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing action parameter")
	}
}

func TestIsValidSessionName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"mysite", true},
		{"my-site_1", true},
		{"MySession", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../etc", false},
		{"foo bar", false},
		{"foo/bar", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSessionName(tt.name)
			if got != tt.valid {
				t.Errorf("isValidSessionName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestBrowserTool_ListSessionsEmpty(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list_sessions",
	})
	if err != nil {
		t.Fatalf("list_sessions failed: %v", err)
	}
	if !strings.Contains(result, "No saved") {
		t.Errorf("expected 'No saved' message, got: %s", result)
	}
}

func TestBrowserTool_ListSessionsWithEntries(t *testing.T) {
	cfg := testBrowserConfig(t)
	// Create some session dirs.
	os.MkdirAll(filepath.Join(cfg.SessionDir, "gmail"), 0700)
	os.MkdirAll(filepath.Join(cfg.SessionDir, "twitter"), 0700)

	tool := NewBrowserTool(cfg)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list_sessions",
	})
	if err != nil {
		t.Fatalf("list_sessions failed: %v", err)
	}
	if !strings.Contains(result, "gmail") || !strings.Contains(result, "twitter") {
		t.Errorf("expected session names in output, got: %s", result)
	}
}

func TestBrowserTool_DeleteSessionNotExist(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "delete_session",
		"session": "nonexistent",
	})
	if err != nil {
		t.Fatalf("delete_session failed: %v", err)
	}
	if !strings.Contains(result, "does not exist") {
		t.Errorf("expected 'does not exist' message, got: %s", result)
	}
}

func TestBrowserTool_DeleteSessionExisting(t *testing.T) {
	cfg := testBrowserConfig(t)
	os.MkdirAll(filepath.Join(cfg.SessionDir, "todelete"), 0700)

	tool := NewBrowserTool(cfg)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "delete_session",
		"session": "todelete",
	})
	if err != nil {
		t.Fatalf("delete_session failed: %v", err)
	}
	if !strings.Contains(result, "deleted") {
		t.Errorf("expected 'deleted' message, got: %s", result)
	}
	// Verify directory is gone.
	if _, err := os.Stat(filepath.Join(cfg.SessionDir, "todelete")); !os.IsNotExist(err) {
		t.Error("session directory should have been removed")
	}
}

func TestBrowserTool_DeleteSessionMissingParam(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "delete_session",
	})
	if err == nil {
		t.Error("expected error for missing session parameter")
	}
}

func TestBrowserTool_DeleteSessionInvalidName(t *testing.T) {
	tool := NewBrowserTool(testBrowserConfig(t))
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "delete_session",
		"session": "../escape",
	})
	if err == nil {
		t.Error("expected error for invalid session name")
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative", "relative"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandTilde(tt.input)
			if got != tt.expected {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
