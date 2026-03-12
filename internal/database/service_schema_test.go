package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

type schemaTestDialect struct {
	resolve func(context.Context, *sql.DB, ConnectionConfig) (string, error)
}

func (d schemaTestDialect) ResolveDefaultSchema(ctx context.Context, db *sql.DB, cfg ConnectionConfig) (string, error) {
	return d.resolve(ctx, db, cfg)
}

func (schemaTestDialect) ListSchemas(context.Context, *sql.DB, ConnectionConfig) ([]SchemaInfo, error) {
	return nil, nil
}

func (schemaTestDialect) ListTables(context.Context, *sql.DB, ConnectionConfig, string) ([]TableInfo, error) {
	return nil, nil
}

func (schemaTestDialect) DescribeTable(context.Context, *sql.DB, ConnectionConfig, string, string) (TableDescription, error) {
	return TableDescription{}, nil
}

func (schemaTestDialect) ListIndexes(context.Context, *sql.DB, ConnectionConfig, string, string) ([]IndexInfo, error) {
	return nil, nil
}

func TestManagedConnectionResolveSchemaUsesExplicitSchema(t *testing.T) {
	conn := &managedConnection{
		cfg: ConnectionConfig{Name: "demo"},
		dialect: schemaTestDialect{
			resolve: func(context.Context, *sql.DB, ConnectionConfig) (string, error) {
				t.Fatal("resolver should not be called when schema is provided")
				return "", nil
			},
		},
	}

	schema, err := conn.resolveSchema(context.Background(), nil, " custom ")
	if err != nil {
		t.Fatalf("resolveSchema returned error: %v", err)
	}
	if schema != "custom" {
		t.Fatalf("expected trimmed explicit schema, got %q", schema)
	}
}

func TestManagedConnectionResolveSchemaUsesConnectionDefault(t *testing.T) {
	conn := &managedConnection{
		cfg: ConnectionConfig{Name: "demo"},
		dialect: schemaTestDialect{
			resolve: func(context.Context, *sql.DB, ConnectionConfig) (string, error) {
				return " current_schema ", nil
			},
		},
	}

	schema, err := conn.resolveSchema(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("resolveSchema returned error: %v", err)
	}
	if schema != "current_schema" {
		t.Fatalf("expected resolved schema, got %q", schema)
	}
}

func TestManagedConnectionResolveSchemaRejectsEmptyDefault(t *testing.T) {
	conn := &managedConnection{
		cfg: ConnectionConfig{Name: "demo"},
		dialect: schemaTestDialect{
			resolve: func(context.Context, *sql.DB, ConnectionConfig) (string, error) {
				return "   ", nil
			},
		},
	}

	_, err := conn.resolveSchema(context.Background(), nil, "")
	if err == nil || !strings.Contains(err.Error(), "has no default schema") {
		t.Fatalf("expected empty default schema error, got %v", err)
	}
}

func TestManagedConnectionResolveSchemaPropagatesResolverError(t *testing.T) {
	conn := &managedConnection{
		cfg: ConnectionConfig{Name: "demo"},
		dialect: schemaTestDialect{
			resolve: func(context.Context, *sql.DB, ConnectionConfig) (string, error) {
				return "", errors.New("boom")
			},
		},
	}

	_, err := conn.resolveSchema(context.Background(), nil, "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected resolver error, got %v", err)
	}
}
