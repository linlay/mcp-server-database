package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type mysqlDialect struct{}

func (mysqlDialect) DefaultSchema(cfg ConnectionConfig) string {
	return mysqlDatabaseName(cfg.DSN)
}

func (mysqlDialect) ListSchemas(ctx context.Context, db *sql.DB, _ ConnectionConfig) ([]SchemaInfo, error) {
	rows, err := db.QueryContext(ctx, `
		select schema_name
		from information_schema.schemata
		where schema_name not in ('information_schema', 'mysql', 'performance_schema', 'sys')
		order by schema_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SchemaInfo
	for rows.Next() {
		var item SchemaInfo
		if err := rows.Scan(&item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d mysqlDialect) ListTables(ctx context.Context, db *sql.DB, cfg ConnectionConfig, schema string) ([]TableInfo, error) {
	rows, err := db.QueryContext(ctx, `
		select table_schema, table_name, table_type
		from information_schema.tables
		where table_schema = ?
		order by table_name`, normalizeSchema(schema, d.DefaultSchema(cfg)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TableInfo
	for rows.Next() {
		var item TableInfo
		if err := rows.Scan(&item.Schema, &item.Name, &item.Type); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d mysqlDialect) DescribeTable(ctx context.Context, db *sql.DB, cfg ConnectionConfig, schema string, table string) (TableDescription, error) {
	schema = normalizeSchema(schema, d.DefaultSchema(cfg))
	description := TableDescription{
		Schema: schema,
		Name:   table,
		Type:   "BASE TABLE",
	}

	pkColumns, err := mysqlPrimaryKeyColumns(ctx, db, schema, table)
	if err != nil {
		return TableDescription{}, err
	}
	pkSet := make(map[string]struct{}, len(pkColumns))
	for _, name := range pkColumns {
		pkSet[name] = struct{}{}
	}
	description.PrimaryKey = pkColumns

	rows, err := db.QueryContext(ctx, `
		select column_name, data_type, is_nullable, column_default, ordinal_position
		from information_schema.columns
		where table_schema = ? and table_name = ?
		order by ordinal_position`, schema, table)
	if err != nil {
		return TableDescription{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item ColumnInfo
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&item.Name, &item.DataType, &nullable, &defaultValue, &item.Position); err != nil {
			return TableDescription{}, err
		}
		item.Nullable = strings.EqualFold(nullable, "YES")
		if defaultValue.Valid {
			item.DefaultValue = defaultValue.String
		}
		_, item.IsPrimaryKey = pkSet[item.Name]
		description.Columns = append(description.Columns, item)
	}
	if err := rows.Err(); err != nil {
		return TableDescription{}, err
	}

	indexes, err := d.ListIndexes(ctx, db, cfg, schema, table)
	if err != nil {
		return TableDescription{}, err
	}
	description.Indexes = indexes
	if len(description.Columns) == 0 {
		return TableDescription{}, fmt.Errorf("table not found: %s.%s", schema, table)
	}
	return description, nil
}

func (mysqlDialect) ListIndexes(ctx context.Context, db *sql.DB, _ ConnectionConfig, schema string, table string) ([]IndexInfo, error) {
	query := `
		select table_schema, table_name, index_name, non_unique, seq_in_index, column_name, index_type
		from information_schema.statistics
		where table_schema = ?`
	args := []any{schema}
	if table != "" {
		query += ` and table_name = ?`
		args = append(args, table)
	}
	query += ` order by table_name, index_name, seq_in_index`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]IndexInfo, 0)
	lookup := map[string]int{}
	for rows.Next() {
		var schemaName string
		var tableName string
		var indexName string
		var nonUnique int
		var seq int
		var columnName sql.NullString
		var indexType string
		if err := rows.Scan(&schemaName, &tableName, &indexName, &nonUnique, &seq, &columnName, &indexType); err != nil {
			return nil, err
		}
		key := schemaName + "." + tableName + "." + indexName
		pos, ok := lookup[key]
		if !ok {
			out = append(out, IndexInfo{
				Schema:     schemaName,
				Table:      tableName,
				Name:       indexName,
				Columns:    []string{},
				Unique:     nonUnique == 0,
				Primary:    indexName == "PRIMARY",
				Definition: indexType,
			})
			pos = len(out) - 1
			lookup[key] = pos
		}
		if columnName.Valid {
			out[pos].Columns = append(out[pos].Columns, columnName.String)
		}
	}
	return out, rows.Err()
}

func mysqlPrimaryKeyColumns(ctx context.Context, db *sql.DB, schema string, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		select column_name
		from information_schema.key_column_usage
		where table_schema = ? and table_name = ? and constraint_name = 'PRIMARY'
		order by ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
