package args

import (
	"fmt"
	"strings"
)

func ReadText(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, exists := values[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func ReadInt(values map[string]any, key string, fallback int) int {
	if values == nil {
		return fallback
	}
	value, exists := values[key]
	if !exists || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func ReadMap(values map[string]any, key string) map[string]any {
	if values == nil {
		return nil
	}
	value, exists := values[key]
	if !exists || value == nil {
		return nil
	}
	node, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return node
}

func ReadArray(values map[string]any, key string) []any {
	if values == nil {
		return []any{}
	}
	value, exists := values[key]
	if !exists || value == nil {
		return []any{}
	}
	items, ok := value.([]any)
	if !ok {
		return []any{}
	}
	return items
}

func ReadBool(values map[string]any, key string) (bool, bool) {
	if values == nil {
		return false, false
	}
	value, exists := values[key]
	if !exists || value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	default:
		return false, false
	}
}
