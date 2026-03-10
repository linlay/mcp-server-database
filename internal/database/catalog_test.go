package database

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalogShouldAggregateDirectoryConnections(t *testing.T) {
	dir := t.TempDir()
	writeConnectionConfig(t, filepath.Join(dir, "b.yml"), `name: beta
description: beta
driver: sqlite
dsn: file:./beta.db
`)
	writeConnectionConfig(t, filepath.Join(dir, "a.yaml"), `name: alpha
description: alpha
driver: mysql
dsn: root:secret@tcp(localhost:3306)/demo
`)

	catalog, err := LoadCatalog(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(catalog.Connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(catalog.Connections))
	}
	if catalog.Connections[0].Name != "alpha" || catalog.Connections[1].Name != "beta" {
		t.Fatalf("expected sorted connections, got %#v", catalog.Connections)
	}
}

func TestLoadCatalogShouldSupportLegacyCatalogFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "databases.yml")
	writeConnectionConfig(t, path, `version: 1
connections:
  - name: legacy
    description: legacy
    driver: postgresql
    dsn: postgres://postgres:secret@localhost:5432/demo?sslmode=disable
`)

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(catalog.Connections) != 1 || catalog.Connections[0].Name != "legacy" {
		t.Fatalf("unexpected catalog: %#v", catalog)
	}
}

func TestLoadCatalogShouldNormalizeMySQLURLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mysql.yml")
	writeConnectionConfig(t, path, `name: demo-mysql
description: mysql url
url: mysql://db.example.com:3306/demo?parseTime=true&charset=utf8mb4
username: demo-user
password: super-secret
`)

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := catalog.Connections[0].DSN; got != "demo-user:super-secret@tcp(db.example.com:3306)/demo?parseTime=true&charset=utf8mb4" {
		t.Fatalf("unexpected mysql dsn: %q", got)
	}
	if got := mysqlDatabaseName(catalog.Connections[0].DSN); got != "demo" {
		t.Fatalf("expected schema demo, got %q", got)
	}
}

func TestLoadCatalogShouldNormalizePostgresURLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "postgres.yml")
	writeConnectionConfig(t, path, `name: demo-postgres
description: postgres url
url: postgres://db.example.com:5432/demo?sslmode=disable
username: demo-user
password: super-secret
`)

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := catalog.Connections[0].DSN; got != "postgres://demo-user:super-secret@db.example.com:5432/demo?sslmode=disable" {
		t.Fatalf("unexpected postgres dsn: %q", got)
	}
}

func TestLoadCatalogShouldNormalizeSQLiteURLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sqlite.yml")
	writeConnectionConfig(t, path, `name: demo-sqlite
description: sqlite url
url: file:./tmp/demo.db?cache=shared
`)

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := catalog.Connections[0].DSN; got != "file:./tmp/demo.db?cache=shared" {
		t.Fatalf("unexpected sqlite dsn: %q", got)
	}
}

func TestLoadCatalogShouldAggregateDirectoryConnectionsUsingURLConfig(t *testing.T) {
	dir := t.TempDir()
	writeConnectionConfig(t, filepath.Join(dir, "a.yml"), `name: alpha
description: mysql url
url: mysql://db.example.com:3306/demo?parseTime=true
username: demo-user
password: super-secret
`)
	writeConnectionConfig(t, filepath.Join(dir, "b.yml"), `name: beta
description: postgres url
url: postgres://db.example.com:5432/demo?sslmode=disable
username: demo-user
password: super-secret
`)

	catalog, err := LoadCatalog(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(catalog.Connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(catalog.Connections))
	}
	if catalog.Connections[0].DSN == "" || catalog.Connections[1].DSN == "" {
		t.Fatalf("expected normalized dsns, got %#v", catalog.Connections)
	}
}

func TestLoadCatalogShouldInferSQLiteDriverFromLocalPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sqlite.yml")
	writeConnectionConfig(t, path, `name: demo-sqlite
description: sqlite path
url: ./tmp/demo.db
`)

	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := catalog.Connections[0].Driver; got != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", got)
	}
	if got := catalog.Connections[0].DSN; got != "./tmp/demo.db" {
		t.Fatalf("unexpected sqlite dsn: %q", got)
	}
}

func TestLoadCatalogShouldRejectMutuallyExclusiveDSNAndURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.yml")
	writeConnectionConfig(t, path, `name: broken
driver: mysql
dsn: demo-user:super-secret@tcp(db.example.com:3306)/demo
url: mysql://db.example.com:3306/demo
username: demo-user
password: super-secret
`)

	_, err := LoadCatalog(path)
	if err == nil || !strings.Contains(err.Error(), "dsn and url are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}
}

func TestLoadCatalogShouldRejectJDBCURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.yml")
	writeConnectionConfig(t, path, `name: broken
driver: mysql
url: jdbc:mysql://db.example.com:3306/demo?parseTime=true
username: demo-user
password: super-secret
`)

	_, err := LoadCatalog(path)
	if err == nil || !strings.Contains(err.Error(), "jdbc") {
		t.Fatalf("expected jdbc validation error, got %v", err)
	}
}

func TestLoadCatalogShouldRejectDriverURLMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.yml")
	writeConnectionConfig(t, path, `name: broken
driver: mysql
url: postgres://db.example.com:5432/demo?sslmode=disable
username: demo-user
password: super-secret
`)

	_, err := LoadCatalog(path)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected driver mismatch error, got %v", err)
	}
}

func TestLoadCatalogShouldRejectDuplicateConnectionNamesAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeConnectionConfig(t, filepath.Join(dir, "first.yml"), `name: demo
driver: sqlite
dsn: file:./demo-1.db
`)
	writeConnectionConfig(t, filepath.Join(dir, "second.yml"), `name: demo
driver: sqlite
dsn: file:./demo-2.db
`)

	if _, err := LoadCatalog(dir); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestLoadCatalogShouldIgnoreExampleConfigFiles(t *testing.T) {
	dir := t.TempDir()
	writeConnectionConfig(t, filepath.Join(dir, "local-sqlite.example.yml"), `name: example
driver: sqlite
dsn: file:./example.db
`)
	writeConnectionConfig(t, filepath.Join(dir, "local-sqlite.yml"), `name: active
driver: sqlite
dsn: file:./active.db
`)

	catalog, err := LoadCatalog(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(catalog.Connections) != 1 {
		t.Fatalf("expected 1 active connection, got %d", len(catalog.Connections))
	}
	if catalog.Connections[0].Name != "active" {
		t.Fatalf("expected active connection, got %#v", catalog.Connections)
	}
}
