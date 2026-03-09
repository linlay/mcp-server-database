package config

import "testing"

func TestLoadShouldApplyDefaultsAndEnvOverrides(t *testing.T) {
	t.Setenv("SERVER_PORT", "18080")
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("DB_CONNECTIONS_CONFIG_PATH", "/tmp/databases")
	t.Setenv("DB_DEFAULT_QUERY_TIMEOUT_SECONDS", "30")
	t.Setenv("DB_MAX_RESULT_ROWS", "99")
	t.Setenv("DB_MAX_CELL_BYTES", "1024")

	cfg := Load()
	if cfg.Server.Port != 18080 {
		t.Fatalf("expected port 18080, got %d", cfg.Server.Port)
	}
	if cfg.MCP.Transport != "stdio" {
		t.Fatalf("expected stdio transport, got %s", cfg.MCP.Transport)
	}
	if cfg.Database.ConnectionsConfigPath != "/tmp/databases" {
		t.Fatalf("unexpected database config path: %s", cfg.Database.ConnectionsConfigPath)
	}
	if cfg.Database.DefaultQueryTimeoutSeconds != 30 {
		t.Fatalf("expected timeout 30, got %d", cfg.Database.DefaultQueryTimeoutSeconds)
	}
	if cfg.Database.MaxResultRows != 99 {
		t.Fatalf("expected max rows 99, got %d", cfg.Database.MaxResultRows)
	}
	if cfg.Database.MaxCellBytes != 1024 {
		t.Fatalf("expected max cell bytes 1024, got %d", cfg.Database.MaxCellBytes)
	}
}
