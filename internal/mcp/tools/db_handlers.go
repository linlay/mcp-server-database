package tools

import (
	"context"
	"fmt"

	"mcp-server-database/internal/database"
	mcpargs "mcp-server-database/internal/mcp/args"
	"mcp-server-database/internal/mcp/jsonutil"
)

type ListConnectionsHandler struct{ baseDatabaseHandler }
type ListSchemasHandler struct{ baseDatabaseHandler }
type ListTablesHandler struct{ baseDatabaseHandler }
type DescribeTableHandler struct{ baseDatabaseHandler }
type ListIndexesHandler struct{ baseDatabaseHandler }
type QueryHandler struct{ baseDatabaseHandler }
type ExecHandler struct{ baseDatabaseHandler }
type DDLHandler struct{ baseDatabaseHandler }

func (h ListConnectionsHandler) Name() string { return ToolListConnections }

func (h ListConnectionsHandler) Call(ctx context.Context, _ map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connections, err := h.service.ListConnections(ctx)
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(struct {
		Connections []database.ConnectionSummary `json:"connections"`
	}{Connections: connections}), nil
}

func (h ListSchemasHandler) Name() string { return ToolListSchemas }

func (h ListSchemasHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	schemas, err := h.service.ListSchemas(ctx, connectionName)
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(struct {
		Schemas []database.SchemaInfo `json:"schemas"`
	}{Schemas: schemas}), nil
}

func (h ListTablesHandler) Name() string { return ToolListTables }

func (h ListTablesHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	tables, err := h.service.ListTables(ctx, connectionName, mcpargs.ReadText(args, "schema"))
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(struct {
		Tables []database.TableInfo `json:"tables"`
	}{Tables: tables}), nil
}

func (h DescribeTableHandler) Name() string { return ToolDescribeTable }

func (h DescribeTableHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	table := mcpargs.ReadText(args, "table")
	if table == "" {
		return nil, fmt.Errorf("table is required")
	}
	description, err := h.service.DescribeTable(ctx, connectionName, mcpargs.ReadText(args, "schema"), table)
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(struct {
		Table database.TableDescription `json:"table"`
	}{Table: description}), nil
}

func (h ListIndexesHandler) Name() string { return ToolListIndexes }

func (h ListIndexesHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	indexes, err := h.service.ListIndexes(ctx, connectionName, mcpargs.ReadText(args, "schema"), mcpargs.ReadText(args, "table"))
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(struct {
		Indexes []database.IndexInfo `json:"indexes"`
	}{Indexes: indexes}), nil
}

func (h QueryHandler) Name() string { return ToolQuery }

func (h QueryHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	sqlText := mcpargs.ReadText(args, "sql")
	if sqlText == "" {
		return nil, fmt.Errorf("sql is required")
	}
	result, err := h.service.Query(ctx, database.QueryRequest{
		ConnectionName: connectionName,
		SQL:            sqlText,
		Args:           mcpargs.ReadArray(args, "args"),
		MaxRows:        mcpargs.ReadInt(args, "max_rows", 0),
	})
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(result), nil
}

func (h ExecHandler) Name() string { return ToolExec }

func (h ExecHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	sqlText := mcpargs.ReadText(args, "sql")
	if sqlText == "" {
		return nil, fmt.Errorf("sql is required")
	}
	result, err := h.service.Exec(ctx, database.ExecRequest{
		ConnectionName: connectionName,
		SQL:            sqlText,
		Args:           mcpargs.ReadArray(args, "args"),
	})
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(result), nil
}

func (h DDLHandler) Name() string { return ToolDDL }

func (h DDLHandler) Call(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := h.requireService(); err != nil {
		return nil, err
	}
	connectionName, err := h.requireConnectionName(args)
	if err != nil {
		return nil, err
	}
	sqlText := mcpargs.ReadText(args, "sql")
	if sqlText == "" {
		return nil, fmt.Errorf("sql is required")
	}
	result, err := h.service.DDL(ctx, database.DDLRequest{
		ConnectionName: connectionName,
		SQL:            sqlText,
	})
	if err != nil {
		return nil, err
	}
	return jsonutil.ToMap(result), nil
}
