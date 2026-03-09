package observability

import (
	"strings"
	"testing"
)

func TestLogSanitizerShouldMaskSensitiveFields(t *testing.T) {
	sanitizer := NewLogSanitizer(200)
	output := sanitizer.SummarizeObject(map[string]any{
		"dsn":      "postgres://postgres:secret@localhost/demo",
		"password": "change_me",
		"query":    "select 1",
	})

	if strings.Contains(output, "secret") || strings.Contains(output, "change_me") {
		t.Fatalf("expected sensitive fields to be masked, got %s", output)
	}
	if !strings.Contains(output, `"query":"select 1"`) {
		t.Fatalf("expected non-sensitive field to remain visible, got %s", output)
	}
}
