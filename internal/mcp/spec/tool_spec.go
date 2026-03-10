package spec

type ToolSpec struct {
	Type          string         `yaml:"type"`
	Name          string         `yaml:"name"`
	Label         string         `yaml:"label,omitempty"`
	Description   string         `yaml:"description"`
	AfterCallHint string         `yaml:"afterCallHint,omitempty"`
	InputSchema   map[string]any `yaml:"inputSchema"`
	ToolAction    bool           `yaml:"toolAction,omitempty"`
	ToolType      string         `yaml:"toolType,omitempty"`
	ViewportKey   string         `yaml:"viewportKey,omitempty"`

	Raw    map[string]any `yaml:"-"`
	Source string         `yaml:"-"`
}
