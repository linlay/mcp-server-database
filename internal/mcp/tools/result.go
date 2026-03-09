package tools

import (
	"encoding/json"
	"strings"
)

func SuccessResult(structured map[string]any) map[string]any {
	if structured == nil {
		structured = map[string]any{}
	}
	structuredText, _ := json.Marshal(structured)
	return map[string]any{
		"structuredContent": structured,
		"content": []map[string]any{{
			"type": "text",
			"text": string(structuredText),
		}},
		"isError": false,
	}
}

func ErrorResult(message string) map[string]any {
	if strings.TrimSpace(message) == "" {
		message = "unknown error"
	}
	return map[string]any{
		"isError": true,
		"error":   message,
		"content": []map[string]any{{
			"type": "text",
			"text": message,
		}},
	}
}
