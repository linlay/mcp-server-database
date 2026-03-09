package spec

import (
	"fmt"
	"strings"

	"mcp-server-database/internal/mcp/schema"
)

func ValidateDefinitions(specs []ToolSpec) error {
	meta, err := schema.Compile("tool-meta-schema", MetaSchema())
	if err != nil {
		return fmt.Errorf("compile tool meta schema: %w", err)
	}

	names := make(map[string]string, len(specs))
	for _, item := range specs {
		if item.Raw == nil {
			return fmt.Errorf("tool spec raw document missing: %s", item.Source)
		}
		if err := schema.Validate(meta, item.Raw); err != nil {
			return fmt.Errorf("invalid tool spec %s: %w", item.Source, err)
		}
		if err := validateToolMode(item); err != nil {
			return fmt.Errorf("invalid tool spec %s: %w", item.Source, err)
		}

		normalized := strings.ToLower(strings.TrimSpace(item.Name))
		if normalized == "" {
			return fmt.Errorf("tool name is required: %s", item.Source)
		}
		if first, exists := names[normalized]; exists {
			return fmt.Errorf("duplicate tool name: %s in %s and %s", item.Name, first, item.Source)
		}
		names[normalized] = item.Source
	}
	return nil
}

func validateToolMode(item ToolSpec) error {
	hasToolType := strings.TrimSpace(item.ToolType) != ""
	hasViewportKey := strings.TrimSpace(item.ViewportKey) != ""

	if item.ToolAction && (hasToolType || hasViewportKey) {
		return fmt.Errorf("toolAction=true cannot be combined with toolType or viewportKey")
	}
	if hasToolType != hasViewportKey {
		return fmt.Errorf("toolType and viewportKey must be declared together")
	}
	return nil
}
