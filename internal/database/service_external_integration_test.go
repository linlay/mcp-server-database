//go:build integration

package database

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
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

func TestServiceMySQLListConnectionsShouldBeReadyOnColdStart(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set")
	}

	service := newIntegrationService(t, "mysql", dsn)
	defer service.Close()

	assertIntegrationConnectionReady(t, service)
}

func TestServicePostgresListConnectionsShouldBeReadyOnColdStart(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	service := newIntegrationService(t, "postgresql", dsn)
	defer service.Close()

	assertIntegrationConnectionReady(t, service)
}

func TestServiceMySQLMetadataUsesConnectionDefaultDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set")
	}

	service := newIntegrationService(t, "mysql", dsn)
	defer service.Close()

	ctx := context.Background()
	if _, err := service.DDL(ctx, DDLRequest{
		ConnectionName: "integration-db",
		SQL:            "create table if not exists mcp_it_default_schema_users (id bigint primary key auto_increment, name varchar(255) not null)",
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}

	tables, err := service.ListTables(ctx, "integration-db", "")
	if err != nil {
		t.Fatalf("list tables with default database: %v", err)
	}
	if !containsTable(tables, "mcp_it_default_schema_users") {
		t.Fatalf("expected table in default database, got %#v", tables)
	}

	desc, err := service.DescribeTable(ctx, "integration-db", "", "mcp_it_default_schema_users")
	if err != nil {
		t.Fatalf("describe table with default database: %v", err)
	}
	if desc.Schema == "" {
		t.Fatal("expected resolved schema in table description")
	}
}

func TestServicePostgresMetadataUsesCurrentSchema(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	targetSchema := "mcp_it_default_schema"
	service := newIntegrationService(t, "postgresql", withPostgresSearchPath(t, dsn, targetSchema))
	defer service.Close()

	ctx := context.Background()
	if _, err := service.DDL(ctx, DDLRequest{
		ConnectionName: "integration-db",
		SQL:            fmt.Sprintf("create schema if not exists %s", targetSchema),
	}); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := service.DDL(ctx, DDLRequest{
		ConnectionName: "integration-db",
		SQL:            "create table if not exists mcp_it_default_schema_users (id bigserial primary key, name text not null)",
	}); err != nil {
		t.Fatalf("create table in current schema: %v", err)
	}

	tables, err := service.ListTables(ctx, "integration-db", "")
	if err != nil {
		t.Fatalf("list tables with current schema: %v", err)
	}
	if !containsTable(tables, "mcp_it_default_schema_users") {
		t.Fatalf("expected table in current schema, got %#v", tables)
	}
	for _, item := range tables {
		if item.Name == "mcp_it_default_schema_users" && item.Schema != targetSchema {
			t.Fatalf("expected schema %q, got %#v", targetSchema, item)
		}
	}

	desc, err := service.DescribeTable(ctx, "integration-db", "", "mcp_it_default_schema_users")
	if err != nil {
		t.Fatalf("describe table with current schema: %v", err)
	}
	if desc.Schema != targetSchema {
		t.Fatalf("expected schema %q, got %#v", targetSchema, desc)
	}

	indexes, err := service.ListIndexes(ctx, "integration-db", "", "mcp_it_default_schema_users")
	if err != nil {
		t.Fatalf("list indexes with current schema: %v", err)
	}
	if len(indexes) == 0 {
		t.Fatal("expected indexes in current schema")
	}
	for _, item := range indexes {
		if item.Schema != targetSchema {
			t.Fatalf("expected schema %q, got %#v", targetSchema, item)
		}
	}
}

func runExternalIntegration(t *testing.T, driver string, dsn string, placeholder string, createTableSQL string) {
	t.Helper()
	service := newIntegrationService(t, driver, dsn)
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

func newIntegrationService(t *testing.T, driver string, dsn string) Service {
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
	return service
}

func withPostgresSearchPath(t *testing.T, dsn string, schema string) string {
	t.Helper()

	parsed, err := neturl.Parse(dsn)
	if err != nil {
		t.Fatalf("parse postgres dsn: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func containsTable(items []TableInfo, table string) bool {
	for _, item := range items {
		if strings.EqualFold(item.Name, table) {
			return true
		}
	}
	return false
}

func assertIntegrationConnectionReady(t *testing.T, service Service) {
	t.Helper()

	connections, err := service.ListConnections(context.Background())
	if err != nil {
		t.Fatalf("list connections: %v", err)
	}
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %#v", connections)
	}
	if connections[0].Status != "ready" {
		t.Fatalf("expected ready status on cold start, got %#v", connections[0])
	}
	if connections[0].StatusReason != "" {
		t.Fatalf("expected empty status reason, got %#v", connections[0])
	}
}
