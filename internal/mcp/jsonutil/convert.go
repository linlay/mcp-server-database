package jsonutil

import "encoding/json"

func ToMap(value any) map[string]any {
	payload, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func DeepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	return ToMap(src)
}
