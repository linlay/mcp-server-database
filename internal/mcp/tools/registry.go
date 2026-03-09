package tools

import (
	"context"
	"fmt"
	"log"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"

	"mcp-server-database/internal/mcp/jsonutil"
	"mcp-server-database/internal/mcp/schema"
	"mcp-server-database/internal/mcp/spec"
)

type ValidationError struct {
	ToolName string
	Err      error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "invalid params"
	}
	if e.ToolName == "" {
		return fmt.Sprintf("invalid params: %v", e.Err)
	}
	return fmt.Sprintf("invalid params for %s: %v", e.ToolName, e.Err)
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type UnknownToolError struct {
	ToolName string
}

func (e *UnknownToolError) Error() string {
	if e == nil {
		return "unknown tool"
	}
	return "unknown tool: " + strings.TrimSpace(e.ToolName)
}

type ToolRegistration struct {
	Spec           spec.ToolSpec
	Handler        ToolHandler
	CompiledSchema *jsonschema.Schema
}

type Registry struct {
	byName  map[string]ToolRegistration
	ordered []ToolRegistration
}

func NewRegistry(specPattern string, handlers []ToolHandler, std *log.Logger) (*Registry, error) {
	if std == nil {
		std = log.Default()
	}
	if strings.TrimSpace(specPattern) == "" {
		specPattern = "./tools/*.yml"
	}

	specs, err := spec.LoadFromPattern(specPattern)
	if err != nil {
		return nil, fmt.Errorf("load tools: %w", err)
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("no tool specs found with pattern %s", specPattern)
	}
	if err := spec.ValidateDefinitions(specs); err != nil {
		return nil, err
	}

	handlerByName := make(map[string]ToolHandler, len(handlers))
	for _, h := range handlers {
		if h == nil {
			return nil, fmt.Errorf("handler cannot be nil")
		}
		name := normalizeName(h.Name())
		if name == "" {
			return nil, fmt.Errorf("handler name cannot be empty")
		}
		if _, exists := handlerByName[name]; exists {
			return nil, fmt.Errorf("duplicate handler name: %s", h.Name())
		}
		handlerByName[name] = h
	}

	ordered := make([]ToolRegistration, 0, len(specs))
	byName := make(map[string]ToolRegistration, len(specs))
	for _, item := range specs {
		name := normalizeName(item.Name)
		handler, ok := handlerByName[name]
		if !ok {
			return nil, fmt.Errorf("tool spec %s has no handler implementation", item.Name)
		}
		compiled, err := schema.Compile(item.Name+".inputSchema", item.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("compile inputSchema for %s: %w", item.Name, err)
		}
		reg := ToolRegistration{
			Spec:           item,
			Handler:        handler,
			CompiledSchema: compiled,
		}
		ordered = append(ordered, reg)
		byName[name] = reg
		delete(handlerByName, name)
	}

	if len(handlerByName) > 0 {
		extra := make([]string, 0, len(handlerByName))
		for name := range handlerByName {
			extra = append(extra, name)
		}
		return nil, fmt.Errorf("handlers without tool spec: %s", strings.Join(extra, ", "))
	}

	std.Printf("event=tool.registry.ready count=%d pattern=%s", len(ordered), specPattern)
	return &Registry{byName: byName, ordered: ordered}, nil
}

func (r *Registry) ListTools() []map[string]any {
	if r == nil || len(r.ordered) == 0 {
		return []map[string]any{}
	}
	tools := make([]map[string]any, 0, len(r.ordered))
	for _, item := range r.ordered {
		tools = append(tools, jsonutil.DeepCopyMap(item.Spec.Raw))
	}
	return tools
}

func (r *Registry) Find(toolName string) (ToolRegistration, bool) {
	if r == nil {
		return ToolRegistration{}, false
	}
	item, ok := r.byName[normalizeName(toolName)]
	if !ok {
		return ToolRegistration{}, false
	}
	return item, true
}

func (r *Registry) Execute(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	item, ok := r.Find(toolName)
	if !ok {
		return nil, &UnknownToolError{ToolName: toolName}
	}
	if args == nil {
		args = map[string]any{}
	}
	if err := schema.Validate(item.CompiledSchema, args); err != nil {
		return nil, &ValidationError{ToolName: item.Spec.Name, Err: err}
	}
	structured, err := item.Handler.Call(ctx, args)
	if err != nil {
		return nil, err
	}
	return SuccessResult(structured), nil
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
