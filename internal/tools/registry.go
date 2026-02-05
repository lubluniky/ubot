package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ErrToolNotFound is returned when a requested tool doesn't exist in the registry.
type ErrToolNotFound struct {
	Name string
}

func (e ErrToolNotFound) Error() string {
	return fmt.Sprintf("tool %q not found", e.Name)
}

// ErrToolAlreadyExists is returned when attempting to register a tool with a name that already exists.
type ErrToolAlreadyExists struct {
	Name string
}

func (e ErrToolAlreadyExists) Error() string {
	return fmt.Sprintf("tool %q already exists", e.Name)
}

// ErrToolExecution is returned when a tool execution fails.
type ErrToolExecution struct {
	Name string
	Err  error
}

func (e ErrToolExecution) Error() string {
	return fmt.Sprintf("tool %q execution failed: %v", e.Name, e.Err)
}

func (e ErrToolExecution) Unwrap() error {
	return e.Err
}

// ToolRegistry manages a collection of tools with thread-safe operations.
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
// Returns ErrToolAlreadyExists if a tool with the same name is already registered.
func (r *ToolRegistry) Register(t Tool) error {
	if t == nil {
		return fmt.Errorf("cannot register nil tool")
	}

	name := t.Name()
	if name == "" {
		return fmt.Errorf("cannot register tool with empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return ErrToolAlreadyExists{Name: name}
	}

	r.tools[name] = t
	return nil
}

// MustRegister adds a tool to the registry, panicking on error.
// This is useful for registering tools during initialization.
func (r *ToolRegistry) MustRegister(t Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// Unregister removes a tool from the registry.
// It's a no-op if the tool doesn't exist.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get retrieves a tool by name.
// Returns the tool if found, nil otherwise.
func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Has checks if a tool with the given name exists in the registry.
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// Execute runs a tool by name with the given parameters.
// Returns ErrToolNotFound if the tool doesn't exist.
// Returns ErrToolExecution if the tool execution fails.
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (string, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return "", ErrToolNotFound{Name: name}
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		return "", ErrToolExecution{Name: name, Err: err}
	}

	return result, nil
}

// GetDefinitions returns tool definitions in OpenAI function calling format.
// This is compatible with the OpenAI API's tools parameter.
func (r *ToolRegistry) GetDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]ToolDefinition, 0, len(r.tools))

	// Get sorted names for consistent ordering
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		tool := r.tools[name]
		definitions = append(definitions, ToDefinition(tool))
	}

	return definitions
}

// List returns a sorted list of all registered tool names.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Names is an alias for List for backward compatibility.
func (r *ToolRegistry) Names() []string {
	return r.List()
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// All returns a copy of all registered tools as a map.
func (r *ToolRegistry) All() map[string]Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]Tool, len(r.tools))
	for name, tool := range r.tools {
		result[name] = tool
	}
	return result
}
