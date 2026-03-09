//go:build integration

package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set")
	}
	runExternalIntegration(t, "mysql", dsn, "?", "create table if not exists mcp_it_users (id bigint primary key auto_increment, name varchar(255) not null)")
}

func TestServicePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	runExternalIntegration(t, "postgresql", dsn, "$1", "create table if not exists mcp_it_users (id bigserial primary key, name text not null)")
}

func runExternalIntegration(t *testing.T, driver string, dsn string, placeholder string, createTableSQL string) {
	t.Helper()
	dir := t.TempDir()
	configDir := filepath.Join(dir, "databases")
	writeConnectionConfig(t, filepath.Join(configDir, "integration-db.yml"), fmt.Sprintf(`name: integration-db
description: integration
driver: %s
dsn: %s
allow_write: true
allow_ddl: true
`, driver, dsn))

	service, err := NewService(Config{
		ConnectionsConfigPath: configDir,
		DefaultQueryTimeout:   10 * time.Second,
		MaxResultRows:         50,
		MaxCellBytes:          1024,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()
	if _, err := service.DDL(ctx, DDLRequest{ConnectionName: "integration-db", SQL: createTableSQL}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := service.Exec(ctx, ExecRequest{
		ConnectionName: "integration-db",
		SQL:            "insert into mcp_it_users(name) values (" + placeholder + ")",
		Args:           []any{"integration"},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	result, err := service.Query(ctx, QueryRequest{
		ConnectionName: "integration-db",
		SQL:            "select name from mcp_it_users",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.RowCount == 0 {
		t.Fatal("expected rows from integration query")
	}
}
