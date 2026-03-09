package spec

import "encoding/json"

const toolMetaSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["type", "name", "description", "inputSchema"],
  "properties": {
    "type": {"type": "string", "minLength": 1},
    "name": {"type": "string", "minLength": 1},
    "description": {"type": "string", "minLength": 1},
    "afterCallHint": {"type": "string"},
    "inputSchema": {"type": "object"},
    "toolAction": {"type": "boolean"},
    "toolType": {"type": "string", "minLength": 1},
    "viewportKey": {"type": "string", "minLength": 1}
  },
  "additionalProperties": false
}`

func MetaSchema() map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal([]byte(toolMetaSchemaJSON), &out)
	return out
}
