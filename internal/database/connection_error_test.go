package database

import (
	"strings"
	"testing"
)

func TestSanitizeConnectionErrorMessageShouldMaskMySQLDSN(t *testing.T) {
	cfg := ConnectionConfig{
		Name:   "broken-mysql",
		Driver: "mysql",
		DSN:    "demo-user:super-secret@tcp(db.example.com:3306)/demo?timeout=1s",
	}

	message := "ping failed for demo-user:super-secret@tcp(db.example.com:3306)/demo?timeout=1s: dial tcp db.example.com:3306: i/o timeout"
	safe := sanitizeConnectionErrorMessage(cfg, message)

	if strings.Contains(safe, "super-secret") {
		t.Fatalf("expected mysql password to be masked, got %q", safe)
	}
	if !strings.Contains(safe, "i/o timeout") {
		t.Fatalf("expected useful error detail, got %q", safe)
	}
}

func TestSanitizeConnectionErrorMessageShouldMaskPostgresURLUserInfo(t *testing.T) {
	cfg := ConnectionConfig{
		Name:   "broken-postgres",
		Driver: "postgresql",
		DSN:    "postgres://demo-user:super-secret@db.example.com:5432/demo?sslmode=disable",
	}

	message := "probe failed with dsn postgres://demo-user:super-secret@db.example.com:5432/demo?sslmode=disable: EOF"
	safe := sanitizeConnectionErrorMessage(cfg, message)

	if strings.Contains(safe, "super-secret") {
		t.Fatalf("expected postgres password to be masked, got %q", safe)
	}
	if !strings.Contains(safe, "EOF") {
		t.Fatalf("expected useful error detail, got %q", safe)
	}
}

func TestSanitizeConnectionErrorMessageShouldMaskPasswordAssignments(t *testing.T) {
	cfg := ConnectionConfig{
		Name:   "broken-postgres",
		Driver: "postgresql",
		DSN:    "postgres://demo-user:super-secret@db.example.com:5432/demo?sslmode=disable",
	}

	message := "connect failed, dsn: postgres://demo-user:super-secret@db.example.com:5432/demo?sslmode=disable, password=super-secret, reason=no such host"
	safe := sanitizeConnectionErrorMessage(cfg, message)

	if strings.Contains(safe, "super-secret") {
		t.Fatalf("expected assignment password to be masked, got %q", safe)
	}
	if !strings.Contains(safe, "no such host") {
		t.Fatalf("expected useful error detail, got %q", safe)
	}
}
