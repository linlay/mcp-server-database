package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServiceSQLiteShouldSupportDDLCRUDAndMetadata(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "demo.db")
	configDir := filepath.Join(dir, "databases")
	writeConnectionConfig(t, filepath.Join(configDir, "local-sqlite.yml"), fmt.Sprintf(`name: local-sqlite
description: test sqlite
driver: sqlite
dsn: file:%s?cache=shared
allow_write: true
allow_ddl: true
max_open_conns: 1
max_idle_conns: 1
`, dbPath))

	service, err := NewService(Config{
		ConnectionsConfigPath: configDir,
		DefaultQueryTimeout:   5 * time.Second,
		MaxResultRows:         5,
		MaxCellBytes:          8,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer service.Close()

	ctx := context.Background()
	if _, err := service.DDL(ctx, DDLRequest{
		ConnectionName: "local-sqlite",
		SQL:            "create table demo_users (id integer primary key autoincrement, name text not null, bio text)",
	}); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if _, err := service.DDL(ctx, DDLRequest{
		ConnectionName: "local-sqlite",
		SQL:            "create index idx_demo_users_name on demo_users(name)",
	}); err != nil {
		t.Fatalf("create index failed: %v", err)
	}

	insert1, err := service.Exec(ctx, ExecRequest{
		ConnectionName: "local-sqlite",
		SQL:            "insert into demo_users(name, bio) values (?, ?)",
		Args:           []any{"alice", "123456789abcdef"},
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if insert1.AffectedRows != 1 {
		t.Fatalf("expected 1 affected row, got %d", insert1.AffectedRows)
	}

	if _, err := service.Exec(ctx, ExecRequest{
		ConnectionName: "local-sqlite",
		SQL:            "insert into demo_users(name, bio) values (?, ?)",
		Args:           []any{"bob", "short"},
	}); err != nil {
		t.Fatalf("second insert failed: %v", err)
	}

	updateResult, err := service.Exec(ctx, ExecRequest{
		ConnectionName: "local-sqlite",
		SQL:            "update demo_users set name = ? where name = ?",
		Args:           []any{"bobby", "bob"},
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updateResult.AffectedRows != 1 {
		t.Fatalf("expected update to affect 1 row, got %d", updateResult.AffectedRows)
	}

	deleteResult, err := service.Exec(ctx, ExecRequest{
		ConnectionName: "local-sqlite",
		SQL:            "delete from demo_users where name = ?",
		Args:           []any{"bobby"},
	})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if deleteResult.AffectedRows != 1 {
		t.Fatalf("expected delete to affect 1 row, got %d", deleteResult.AffectedRows)
	}

	queryResult, err := service.Query(ctx, QueryRequest{
		ConnectionName: "local-sqlite",
		SQL:            "select id, name, bio from demo_users order by id",
		MaxRows:        10,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if queryResult.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", queryResult.RowCount)
	}
	bio, _ := queryResult.Rows[0]["bio"].(string)
	if !strings.Contains(bio, "...(truncated)") {
		t.Fatalf("expected truncated bio, got %q", bio)
	}

	connections, err := service.ListConnections(ctx)
	if err != nil {
		t.Fatalf("list connections failed: %v", err)
	}
	if len(connections) != 1 || connections[0].Status != "ready" {
		t.Fatalf("unexpected connections response: %#v", connections)
	}
	if connections[0].StatusReason != "" {
		t.Fatalf("expected no status reason for ready connection, got %#v", connections[0])
	}

	tables, err := service.ListTables(ctx, "local-sqlite", "")
	if err != nil {
		t.Fatalf("list tables failed: %v", err)
	}
	if len(tables) != 1 || tables[0].Name != "demo_users" {
		t.Fatalf("unexpected tables: %#v", tables)
	}

	description, err := service.DescribeTable(ctx, "local-sqlite", "", "demo_users")
	if err != nil {
		t.Fatalf("describe table failed: %v", err)
	}
	if len(description.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(description.Columns))
	}
	if len(description.PrimaryKey) != 1 || description.PrimaryKey[0] != "id" {
		t.Fatalf("unexpected primary key: %#v", description.PrimaryKey)
	}

	indexes, err := service.ListIndexes(ctx, "local-sqlite", "", "demo_users")
	if err != nil {
		t.Fatalf("list indexes failed: %v", err)
	}
	if len(indexes) == 0 {
		t.Fatal("expected at least one index")
	}
	found := false
	for _, item := range indexes {
		if item.Name == "idx_demo_users_name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected custom index in %#v", indexes)
	}
}

func TestServiceSQLiteShouldRespectPermissionsAndQueryBoundaries(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "databases")
	writeConnectionConfig(t, filepath.Join(configDir, "readonly-sqlite.yml"), fmt.Sprintf(`name: readonly-sqlite
description: readonly test
driver: sqlite
dsn: file:%s?cache=shared
allow_write: false
allow_ddl: false
`, filepath.Join(dir, "readonly.db")))

	service, err := NewService(Config{
		ConnectionsConfigPath: configDir,
		DefaultQueryTimeout:   5 * time.Second,
		MaxResultRows:         10,
		MaxCellBytes:          256,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer service.Close()

	if _, err := service.Exec(context.Background(), ExecRequest{
		ConnectionName: "readonly-sqlite",
		SQL:            "insert into demo values (1)",
	}); err == nil || !strings.Contains(err.Error(), "does not allow write") {
		t.Fatalf("expected write permission error, got %v", err)
	}

	if _, err := service.DDL(context.Background(), DDLRequest{
		ConnectionName: "readonly-sqlite",
		SQL:            "create table demo(id integer)",
	}); err == nil || !strings.Contains(err.Error(), "does not allow ddl") {
		t.Fatalf("expected ddl permission error, got %v", err)
	}

	if _, err := service.Query(context.Background(), QueryRequest{
		ConnectionName: "readonly-sqlite",
		SQL:            "insert into demo values (1)",
	}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only boundary error, got %v", err)
	}
}

func TestServiceListConnectionsShouldExposeSanitizedFailureReason(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "databases")
	writeConnectionConfig(t, filepath.Join(configDir, "broken-mysql.yml"), `name: broken-mysql
description: broken mysql
url: mysql://127.0.0.1:1/demo?timeout=1s&readTimeout=1s&writeTimeout=1s
username: demo-user
password: super-secret
allow_write: false
allow_ddl: false
`)

	service, err := NewService(Config{
		ConnectionsConfigPath: configDir,
		DefaultQueryTimeout:   2 * time.Second,
		MaxResultRows:         10,
		MaxCellBytes:          256,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer service.Close()

	connections, err := service.ListConnections(context.Background())
	if err != nil {
		t.Fatalf("list connections failed: %v", err)
	}
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %#v", connections)
	}
	if connections[0].Status != "error" {
		t.Fatalf("expected error status, got %#v", connections[0])
	}
	if strings.TrimSpace(connections[0].StatusReason) == "" {
		t.Fatalf("expected failure reason, got %#v", connections[0])
	}
	if strings.Contains(connections[0].StatusReason, "super-secret") {
		t.Fatalf("expected password to be redacted, got %q", connections[0].StatusReason)
	}
	if strings.Contains(connections[0].StatusReason, "demo-user:super-secret@") {
		t.Fatalf("expected DSN userinfo to be redacted, got %q", connections[0].StatusReason)
	}
}

func writeConnectionConfig(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
