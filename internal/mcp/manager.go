package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/hkuds/ubot/internal/tools"
)

// Manager handles multiple MCP server connections.
type Manager struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

// NewManager creates a new MCP manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

// AddServer adds and connects to a new MCP server.
func (m *Manager) AddServer(ctx context.Context, server Server) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[server.Name]; exists {
		return fmt.Errorf("server %q already exists", server.Name)
	}

	client := NewClient(server)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to server %q: %w", server.Name, err)
	}

	// Fetch tools from the server
	if _, err := client.ListTools(ctx); err != nil {
		client.Disconnect()
		return fmt.Errorf("failed to list tools from server %q: %w", server.Name, err)
	}

	m.clients[server.Name] = client
	log.Printf("Connected to MCP server %q", server.Name)

	return nil
}

// RemoveServer disconnects and removes an MCP server.
func (m *Manager) RemoveServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[name]
	if !exists {
		return fmt.Errorf("server %q not found", name)
	}

	if err := client.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect from server %q: %w", name, err)
	}

	delete(m.clients, name)
	log.Printf("Disconnected from MCP server %q", name)

	return nil
}

// GetClient returns the client for a specific server.
func (m *Manager) GetClient(name string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[name]
}

// GetAllTools returns all tools from all connected servers as uBot tool definitions.
func (m *Manager) GetAllTools() []tools.ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var definitions []tools.ToolDefinition

	for serverName, client := range m.clients {
		mcpTools := client.GetCachedTools()
		for _, tool := range mcpTools {
			def := tools.ToolDefinition{
				Type: "function",
				Function: tools.FunctionDefinition{
					Name:        fmt.Sprintf("mcp_%s_%s", serverName, tool.Name),
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
			definitions = append(definitions, def)
		}
	}

	return definitions
}

// GetServerTools returns tools from a specific server.
func (m *Manager) GetServerTools(serverName string) ([]Tool, error) {
	m.mu.RLock()
	client, exists := m.clients[serverName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %q not found", serverName)
	}

	return client.GetCachedTools(), nil
}

// CallTool calls a tool on a specific server.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	client, exists := m.clients[serverName]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("server %q not found", serverName)
	}

	result, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		return "", err
	}

	// Convert result to string
	switch v := result.(type) {
	case string:
		return v, nil
	default:
		// Try to marshal as JSON
		return fmt.Sprintf("%v", v), nil
	}
}

// RefreshTools refreshes the tool list from all connected servers.
func (m *Manager) RefreshTools(ctx context.Context) error {
	m.mu.RLock()
	clients := make(map[string]*Client, len(m.clients))
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	for name, client := range clients {
		if _, err := client.ListTools(ctx); err != nil {
			log.Printf("Failed to refresh tools from server %q: %v", name, err)
		}
	}

	return nil
}

// ListServers returns the names of all connected servers.
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// Close disconnects from all MCP servers.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, client := range m.clients {
		if err := client.Disconnect(); err != nil {
			log.Printf("Error disconnecting from server %q: %v", name, err)
			lastErr = err
		}
	}

	m.clients = make(map[string]*Client)
	return lastErr
}

// CreateBridgedTools creates MCPToolBridge instances for all MCP tools
// that can be registered with the uBot tool registry.
func (m *Manager) CreateBridgedTools() []tools.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var bridgedTools []tools.Tool

	for serverName, client := range m.clients {
		mcpTools := client.GetCachedTools()
		for _, tool := range mcpTools {
			bridge := NewMCPToolBridge(m, serverName, tool)
			bridgedTools = append(bridgedTools, bridge)
		}
	}

	return bridgedTools
}
