package mcp

import (
	"context"
	"fmt"
)

// MCPToolBridge wraps an MCP tool as a uBot tool, allowing MCP tools
// to be used seamlessly within the uBot tool system.
type MCPToolBridge struct {
	manager    *Manager
	serverName string
	tool       Tool
}

// NewMCPToolBridge creates a new bridge for an MCP tool.
func NewMCPToolBridge(manager *Manager, serverName string, tool Tool) *MCPToolBridge {
	return &MCPToolBridge{
		manager:    manager,
		serverName: serverName,
		tool:       tool,
	}
}

// Name returns the tool's identifier.
// The name is prefixed with "mcp_<serverName>_" to avoid conflicts.
func (b *MCPToolBridge) Name() string {
	return fmt.Sprintf("mcp_%s_%s", b.serverName, b.tool.Name)
}

// Description returns a human-readable description for the LLM.
func (b *MCPToolBridge) Description() string {
	return b.tool.Description
}

// Parameters returns the JSON Schema for tool parameters.
func (b *MCPToolBridge) Parameters() map[string]interface{} {
	if b.tool.InputSchema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	return b.tool.InputSchema
}

// Execute runs the tool with given parameters.
func (b *MCPToolBridge) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	return b.manager.CallTool(ctx, b.serverName, b.tool.Name, params)
}

// GetServerName returns the name of the MCP server this tool belongs to.
func (b *MCPToolBridge) GetServerName() string {
	return b.serverName
}

// GetOriginalToolName returns the original tool name from the MCP server.
func (b *MCPToolBridge) GetOriginalToolName() string {
	return b.tool.Name
}
