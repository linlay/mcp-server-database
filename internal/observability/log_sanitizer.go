package observability

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	maskValue              = "***"
	maxObjectFieldsPreview = 20
	maxArrayItemsPreview   = 20
)

var sensitiveKeywords = []string{
	"password", "passwd", "pwd", "token", "secret", "apikey", "api-key", "api_key",
	"authorization", "auth", "accesskey", "access-key", "access_key", "privatekey", "private_key", "contentbase64", "dsn",
}

type LogSanitizer struct {
	maxBodyLength int
}

func NewLogSanitizer(maxBodyLength int) *LogSanitizer {
	return &LogSanitizer{maxBodyLength: maxBodyLength}
}

func (s *LogSanitizer) SummarizeJSON(value any) string {
	safe := s.summarizeAndMask(value)
	bytes, err := json.Marshal(safe)
	if err != nil {
		return s.truncate(fmt.Sprint(value))
	}
	return s.truncate(string(bytes))
}

func (s *LogSanitizer) SummarizeObject(value any) string {
	return s.SummarizeJSON(value)
}

func (s *LogSanitizer) summarizeAndMask(value any) any {
	normalized := normalizeValue(value)
	switch typed := normalized.(type) {
	case nil:
		return nil
	case []any:
		return s.summarizeArray(typed)
	case map[string]any:
		return s.summarizeMap(typed)
	default:
		return typed
	}
}

func (s *LogSanitizer) summarizeArray(items []any) any {
	if len(items) <= maxArrayItemsPreview {
		safe := make([]any, 0, len(items))
		for _, item := range items {
			safe = append(safe, s.summarizeAndMask(item))
		}
		return safe
	}
	preview := make([]any, 0, maxArrayItemsPreview)
	for i := 0; i < maxArrayItemsPreview; i++ {
		preview = append(preview, s.summarizeAndMask(items[i]))
	}
	return map[string]any{
		"_summary": fmt.Sprintf("array(%d items)", len(items)),
		"preview":  preview,
	}
}

func (s *LogSanitizer) summarizeMap(object map[string]any) any {
	keys := sortedKeys(object)
	if len(keys) <= maxObjectFieldsPreview {
		safe := make(map[string]any, len(keys))
		for _, key := range keys {
			safe[key] = s.maskField(key, object[key])
		}
		return safe
	}
	preview := make(map[string]any, maxObjectFieldsPreview)
	for i := 0; i < maxObjectFieldsPreview; i++ {
		key := keys[i]
		preview[key] = s.maskField(key, object[key])
	}
	return map[string]any{
		"_summary": fmt.Sprintf("object(%d fields)", len(keys)),
		"preview":  preview,
	}
}

func (s *LogSanitizer) maskField(fieldName string, value any) any {
	if isSensitiveKey(fieldName) {
		return maskValue
	}
	return s.summarizeAndMask(value)
}

func (s *LogSanitizer) truncate(input string) string {
	if input == "" {
		return ""
	}
	max := s.maxBodyLength
	if max < 80 {
		max = 80
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max]) + "...(truncated)"
}

func isSensitiveKey(fieldName string) bool {
	normalized := strings.ToLower(fieldName)
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	for _, keyword := range sensitiveKeywords {
		key := strings.ToLower(keyword)
		key = strings.ReplaceAll(key, "-", "")
		key = strings.ReplaceAll(key, "_", "")
		if strings.Contains(normalized, key) {
			return true
		}
	}
	return false
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]any:
		return typed
	case []any:
		return typed
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64,
		json.Number:
		return typed
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		var decoded any
		if err := json.Unmarshal(bytes, &decoded); err != nil {
			return fmt.Sprint(value)
		}
		return decoded
	}
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
