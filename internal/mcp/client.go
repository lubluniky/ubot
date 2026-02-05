package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Client manages a connection to an MCP server.
type Client struct {
	server  Server
	process *exec.Cmd      // For stdio transport
	stdin   io.WriteCloser // stdin pipe to process
	stdout  io.ReadCloser  // stdout pipe from process
	scanner *bufio.Scanner // Scanner for reading responses
	client  *http.Client   // For HTTP transport
	tools   []Tool         // Cached list of tools
	nextID  int            // Request ID counter
	mu      sync.Mutex     // Protects nextID and tools

	connected bool
	connMu    sync.RWMutex
}

// NewClient creates a new MCP client for the given server configuration.
func NewClient(server Server) *Client {
	return &Client{
		server: server,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		nextID: 1,
	}
}

// Connect establishes a connection to the MCP server.
func (c *Client) Connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.connected {
		return nil
	}

	var err error
	switch c.server.Transport {
	case "stdio", "":
		err = c.connectStdio(ctx)
	case "http":
		err = c.connectHTTP(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", c.server.Transport)
	}

	if err != nil {
		return err
	}

	c.connected = true
	return nil
}

// connectStdio establishes a stdio connection by spawning the MCP server process.
func (c *Client) connectStdio(ctx context.Context) error {
	// Build command
	c.process = exec.CommandContext(ctx, c.server.Command, c.server.Args...)

	// Set environment variables
	c.process.Env = os.Environ()
	for k, v := range c.server.Env {
		c.process.Env = append(c.process.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up pipes
	var err error
	c.stdin, err = c.process.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create scanner for reading line-by-line responses
	c.scanner = bufio.NewScanner(c.stdout)
	c.scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line size

	// Start the process
	if err := c.process.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Send initialize request
	if err := c.initialize(ctx); err != nil {
		c.Disconnect()
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	return nil
}

// connectHTTP establishes an HTTP connection to the MCP server.
func (c *Client) connectHTTP(ctx context.Context) error {
	// For HTTP, we just need to verify the server is reachable
	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize HTTP MCP server: %w", err)
	}
	return nil
}

// initialize sends the initialize request to the MCP server.
func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Roots: &RootsCapability{
				ListChanged: false,
			},
		},
		ClientInfo: ClientInfo{
			Name:    "uBot",
			Version: "0.1.0",
		},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return err
	}

	// Send initialized notification
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return err
	}

	return nil
}

// Disconnect closes the connection to the MCP server.
func (c *Client) Disconnect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.process != nil && c.process.Process != nil {
		c.process.Process.Kill()
		c.process.Wait()
	}

	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// ListTools retrieves the list of available tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}

	// Cache the tools
	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()

	return result.Tools, nil
}

// GetCachedTools returns the cached list of tools.
func (c *Client) GetCachedTools() []Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools
}

// CallTool invokes a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	var result CallToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}

	if result.IsError {
		// Extract error message from content
		for _, block := range result.Content {
			if block.Type == "text" {
				return nil, fmt.Errorf("tool error: %s", block.Text)
			}
		}
		return nil, fmt.Errorf("tool returned an error")
	}

	// Convert content blocks to a string result
	var textResult string
	for _, block := range result.Content {
		if block.Type == "text" {
			if textResult != "" {
				textResult += "\n"
			}
			textResult += block.Text
		}
	}

	return textResult, nil
}

// call sends a JSON-RPC request and waits for the response.
func (c *Client) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	var resp Response
	var err error

	switch c.server.Transport {
	case "stdio", "":
		resp, err = c.callStdio(ctx, req)
	case "http":
		resp, err = c.callHTTP(ctx, req)
	default:
		return fmt.Errorf("unsupported transport: %s", c.server.Transport)
	}

	if err != nil {
		return err
	}

	if resp.Error != nil {
		return resp.Error
	}

	// Decode result into the provided interface
	if result != nil && resp.Result != nil {
		resultBytes, err := json.Marshal(resp.Result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		if err := json.Unmarshal(resultBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// callStdio sends a request over stdio and reads the response.
func (c *Client) callStdio(ctx context.Context, req Request) (Response, error) {
	// Marshal request
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write request to stdin (with newline)
	c.mu.Lock()
	_, err = c.stdin.Write(append(reqBytes, '\n'))
	c.mu.Unlock()
	if err != nil {
		return Response{}, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response from stdout
	// We need to read until we get a response with our ID
	for {
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		default:
		}

		if !c.scanner.Scan() {
			if err := c.scanner.Err(); err != nil {
				return Response{}, fmt.Errorf("failed to read response: %w", err)
			}
			return Response{}, fmt.Errorf("unexpected end of response stream")
		}

		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			// Skip lines that aren't valid JSON responses (could be notifications)
			continue
		}

		// Check if this is a response to our request
		if resp.ID == req.ID {
			return resp, nil
		}

		// Otherwise it might be a notification or response to a different request
		// For now, we skip it
	}
}

// callHTTP sends a request over HTTP and reads the response.
func (c *Client) callHTTP(ctx context.Context, req Request) (Response, error) {
	// Marshal request
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.server.URL, bytes.NewReader(reqBytes))
	if err != nil {
		return Response{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("failed to read HTTP response: %w", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return Response{}, fmt.Errorf("failed to parse HTTP response: %w", err)
	}

	return resp, nil
}

// notify sends a notification (no response expected).
func (c *Client) notify(ctx context.Context, method string, params interface{}) error {
	req := struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	switch c.server.Transport {
	case "stdio", "":
		return c.notifyStdio(req)
	case "http":
		return c.notifyHTTP(ctx, req)
	default:
		return fmt.Errorf("unsupported transport: %s", c.server.Transport)
	}
}

// notifyStdio sends a notification over stdio.
func (c *Client) notifyStdio(req interface{}) error {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err = c.stdin.Write(append(reqBytes, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// notifyHTTP sends a notification over HTTP.
func (c *Client) notifyHTTP(ctx context.Context, req interface{}) error {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.server.URL, bytes.NewReader(reqBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP notification failed: %w", err)
	}
	httpResp.Body.Close()

	return nil
}

// GetServerName returns the server name.
func (c *Client) GetServerName() string {
	return c.server.Name
}
