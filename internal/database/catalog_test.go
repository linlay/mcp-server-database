package database

import (
	"path/filepath"
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
