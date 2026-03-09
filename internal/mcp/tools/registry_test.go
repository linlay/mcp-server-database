package tools

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryShouldLoadAndListTools(t *testing.T) {
	r, err := NewRegistry(projectToolsPattern(), testHandlers(), log.New(os.Stdout, "", 0))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	listed := r.ListTools()
	if len(listed) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(listed))
	}
}

func TestRegistryShouldFailWhenInputSchemaInvalid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "invalid.yml"), `type: function
name: db_list_tables
description: test
inputSchema:
  type: 123
`)

	_, err := NewRegistry(filePattern(dir), []ToolHandler{stubHandler{name: "db_list_tables"}}, log.New(os.Stdout, "", 0))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "compile inputSchema") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryExecuteShouldReturnValidationError(t *testing.T) {
	r, err := NewRegistry(projectToolsPattern(), testHandlers(), log.New(os.Stdout, "", 0))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, err = r.Execute(context.Background(), "db_query", map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestRegistryShouldPreserveExtendedMetadataInListTools(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "frontend.yml"), `type: function
name: db_demo_frontend
description: test
toolType: html
viewportKey: show_db_demo
inputSchema:
  type: object
  additionalProperties: false
`)
	writeFile(t, filepath.Join(dir, "action.yml"), `type: function
name: db_demo_action
description: test
toolAction: true
inputSchema:
  type: object
  additionalProperties: false
`)

	r, err := NewRegistry(filePattern(dir), []ToolHandler{
		stubHandler{name: "db_demo_frontend"},
		stubHandler{name: "db_demo_action"},
	}, log.New(os.Stdout, "", 0))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	listed := r.ListTools()
	if len(listed) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(listed))
	}

	byName := map[string]map[string]any{}
	for _, item := range listed {
		name, _ := item["name"].(string)
		byName[name] = item
	}

	if byName["db_demo_frontend"]["toolType"] != "html" {
		t.Fatalf("unexpected toolType: %#v", byName["db_demo_frontend"]["toolType"])
	}
	if byName["db_demo_frontend"]["viewportKey"] != "show_db_demo" {
		t.Fatalf("unexpected viewportKey: %#v", byName["db_demo_frontend"]["viewportKey"])
	}
	if byName["db_demo_action"]["toolAction"] != true {
		t.Fatalf("unexpected toolAction: %#v", byName["db_demo_action"]["toolAction"])
	}
}

func TestRegistryShouldFailWhenToolModeMetadataIsInvalid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "invalid.yml"), `type: function
name: db_demo_frontend
description: test
toolType: html
inputSchema:
  type: object
  additionalProperties: false
`)

	_, err := NewRegistry(filePattern(dir), []ToolHandler{stubHandler{name: "db_demo_frontend"}}, log.New(os.Stdout, "", 0))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "toolType and viewportKey must be declared together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryShouldFailWhenActionToolAlsoDeclaresFrontendFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "invalid.yml"), `type: function
name: db_demo_action
description: test
toolAction: true
toolType: html
viewportKey: show_db_demo
inputSchema:
  type: object
  additionalProperties: false
`)

	_, err := NewRegistry(filePattern(dir), []ToolHandler{stubHandler{name: "db_demo_action"}}, log.New(os.Stdout, "", 0))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "toolAction=true cannot be combined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type stubHandler struct {
	name string
}

func (s stubHandler) Name() string {
	return s.name
}

func (s stubHandler) Call(_ context.Context, _ map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

func testHandlers() []ToolHandler {
	return []ToolHandler{
		stubHandler{name: ToolListConnections},
		stubHandler{name: ToolListSchemas},
		stubHandler{name: ToolListTables},
		stubHandler{name: ToolDescribeTable},
		stubHandler{name: ToolListIndexes},
		stubHandler{name: ToolQuery},
		stubHandler{name: ToolExec},
		stubHandler{name: ToolDDL},
	}
}

func projectToolsPattern() string {
	return filepath.Join("..", "..", "..", "tools", "*.yml")
}

func filePattern(dir string) string {
	return filepath.Join(dir, "*.yml")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
