package tools

import (
	"context"
	"fmt"

	"mcp-server-database/internal/database"
	mcpargs "mcp-server-database/internal/mcp/args"
)

const (
	ToolListConnections = "db_list_connections"
	ToolListSchemas     = "db_list_schemas"
	ToolListTables      = "db_list_tables"
	ToolDescribeTable   = "db_describe_table"
	ToolListIndexes     = "db_list_indexes"
	ToolQuery           = "db_query"
	ToolExec            = "db_exec"
	ToolDDL             = "db_ddl"
)

type ToolHandler interface {
	Name() string
	Call(ctx context.Context, args map[string]any) (map[string]any, error)
}

type baseDatabaseHandler struct {
	service database.Service
}

func (b baseDatabaseHandler) requireService() error {
	if b.service == nil {
		return fmt.Errorf("database service is not configured")
	}
	return nil
}

func (b baseDatabaseHandler) requireConnectionName(args map[string]any) (string, error) {
	connectionName := mcpargs.ReadText(args, "connection_name")
	if connectionName == "" {
		return "", fmt.Errorf("connection_name is required")
	}
	return connectionName, nil
}

func BuiltinHandlers(service database.Service) []ToolHandler {
	base := baseDatabaseHandler{service: service}
	return []ToolHandler{
		ListConnectionsHandler{baseDatabaseHandler: base},
		ListSchemasHandler{baseDatabaseHandler: base},
		ListTablesHandler{baseDatabaseHandler: base},
		DescribeTableHandler{baseDatabaseHandler: base},
		ListIndexesHandler{baseDatabaseHandler: base},
		QueryHandler{baseDatabaseHandler: base},
		ExecHandler{baseDatabaseHandler: base},
		DDLHandler{baseDatabaseHandler: base},
	}
}
