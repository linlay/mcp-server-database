package database

import (
	"context"
	"time"
)

type Config struct {
	ConnectionsConfigPath string
	DefaultQueryTimeout   time.Duration
	MaxResultRows         int
	MaxCellBytes          int
}

type Catalog struct {
	Version     int                `yaml:"version"`
	Connections []ConnectionConfig `yaml:"connections"`
}

type ConnectionConfig struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	Driver                 string `yaml:"driver"`
	DSN                    string `yaml:"dsn"`
	AllowWrite             bool   `yaml:"allow_write"`
	AllowDDL               bool   `yaml:"allow_ddl"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"`
}

type ConnectionSummary struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Driver       string `json:"driver"`
	AllowWrite   bool   `json:"allow_write"`
	AllowDDL     bool   `json:"allow_ddl"`
	Status       string `json:"status"`
	StatusReason string `json:"status_reason,omitempty"`
}

type SchemaInfo struct {
	Name string `json:"name"`
}

type TableInfo struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	Nullable     bool   `json:"nullable"`
	DefaultValue any    `json:"default_value,omitempty"`
	Position     int    `json:"position"`
	IsPrimaryKey bool   `json:"is_primary_key"`
}

type IndexInfo struct {
	Schema     string   `json:"schema"`
	Table      string   `json:"table"`
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	Unique     bool     `json:"unique"`
	Primary    bool     `json:"primary"`
	Definition string   `json:"definition,omitempty"`
}

type TableDescription struct {
	Schema     string       `json:"schema"`
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Columns    []ColumnInfo `json:"columns"`
	PrimaryKey []string     `json:"primary_key"`
	Indexes    []IndexInfo  `json:"indexes"`
}

type QueryRequest struct {
	ConnectionName string
	SQL            string
	Args           []any
	MaxRows        int
}

type QueryResult struct {
	Columns   []string         `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	RowCount  int              `json:"row_count"`
	Truncated bool             `json:"truncated"`
	ElapsedMs int64            `json:"elapsed_ms"`
}

type ExecRequest struct {
	ConnectionName string
	SQL            string
	Args           []any
}

type ExecResult struct {
	AffectedRows int64  `json:"affected_rows"`
	LastInsertID *int64 `json:"last_insert_id,omitempty"`
	ElapsedMs    int64  `json:"elapsed_ms"`
}

type DDLRequest struct {
	ConnectionName string
	SQL            string
}

type DDLResult struct {
	StatementType string `json:"statement_type"`
	ElapsedMs     int64  `json:"elapsed_ms"`
}

type Service interface {
	Close() error
	ListConnections(ctx context.Context) ([]ConnectionSummary, error)
	ListSchemas(ctx context.Context, connectionName string) ([]SchemaInfo, error)
	ListTables(ctx context.Context, connectionName string, schema string) ([]TableInfo, error)
	DescribeTable(ctx context.Context, connectionName string, schema string, table string) (TableDescription, error)
	ListIndexes(ctx context.Context, connectionName string, schema string, table string) ([]IndexInfo, error)
	Query(ctx context.Context, req QueryRequest) (QueryResult, error)
	Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
	DDL(ctx context.Context, req DDLRequest) (DDLResult, error)
}
