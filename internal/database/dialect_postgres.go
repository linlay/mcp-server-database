package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type postgresDialect struct{}

func (postgresDialect) Probe(ctx context.Context, db *sql.DB, _ ConnectionConfig) error {
	var ready int
	if err := db.QueryRowContext(ctx, `select 1`).Scan(&ready); err != nil {
		return err
	}
	return nil
}

func (postgresDialect) ResolveDefaultSchema(ctx context.Context, db *sql.DB, _ ConnectionConfig) (string, error) {
	var schema sql.NullString
	if err := db.QueryRowContext(ctx, `select current_schema()`).Scan(&schema); err != nil {
		return "", fmt.Errorf("resolve default schema: %w", err)
	}
	if !schema.Valid || strings.TrimSpace(schema.String) == "" {
		return "", fmt.Errorf("resolve default schema: current_schema() returned empty")
	}
	return strings.TrimSpace(schema.String), nil
}

func (postgresDialect) ListSchemas(ctx context.Context, db *sql.DB, _ ConnectionConfig) ([]SchemaInfo, error) {
	rows, err := db.QueryContext(ctx, `
		select schema_name
		from information_schema.schemata
		where schema_name not in ('information_schema')
		  and schema_name not like 'pg_%'
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

func (postgresDialect) ListTables(ctx context.Context, db *sql.DB, _ ConnectionConfig, schema string) ([]TableInfo, error) {
	rows, err := db.QueryContext(ctx, `
		select table_schema, table_name, table_type
		from information_schema.tables
		where table_schema = $1
		order by table_name`, strings.TrimSpace(schema))
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

func (d postgresDialect) DescribeTable(ctx context.Context, db *sql.DB, cfg ConnectionConfig, schema string, table string) (TableDescription, error) {
	schema = strings.TrimSpace(schema)
	description := TableDescription{
		Schema: schema,
		Name:   table,
		Type:   "BASE TABLE",
	}

	pkColumns, err := postgresPrimaryKeyColumns(ctx, db, schema, table)
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
		where table_schema = $1 and table_name = $2
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

func (postgresDialect) ListIndexes(ctx context.Context, db *sql.DB, _ ConnectionConfig, schema string, table string) ([]IndexInfo, error) {
	query := `
		select ns.nspname as schema_name,
		       tbl.relname as table_name,
		       idx.relname as index_name,
		       ind.indisunique,
		       ind.indisprimary,
		       coalesce(string_agg(att.attname, ',' order by ord.ordinality), '') as columns_csv,
		       pg_get_indexdef(ind.indexrelid) as definition
		from pg_index ind
		join pg_class tbl on tbl.oid = ind.indrelid
		join pg_namespace ns on ns.oid = tbl.relnamespace
		join pg_class idx on idx.oid = ind.indexrelid
		left join lateral unnest(ind.indkey) with ordinality as ord(attnum, ordinality) on true
		left join pg_attribute att on att.attrelid = tbl.oid and att.attnum = ord.attnum
		where ns.nspname = $1`
	args := []any{schema}
	if table != "" {
		query += ` and tbl.relname = $2`
		args = append(args, table)
	}
	query += `
		group by ns.nspname, tbl.relname, idx.relname, ind.indisunique, ind.indisprimary, ind.indexrelid
		order by tbl.relname, idx.relname`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IndexInfo
	for rows.Next() {
		var item IndexInfo
		var columnsCSV string
		if err := rows.Scan(&item.Schema, &item.Table, &item.Name, &item.Unique, &item.Primary, &columnsCSV, &item.Definition); err != nil {
			return nil, err
		}
		if columnsCSV != "" {
			item.Columns = strings.Split(columnsCSV, ",")
		} else {
			item.Columns = []string{}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func postgresPrimaryKeyColumns(ctx context.Context, db *sql.DB, schema string, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		select kcu.column_name
		from information_schema.table_constraints tc
		join information_schema.key_column_usage kcu
		  on tc.constraint_name = kcu.constraint_name
		 and tc.table_schema = kcu.table_schema
		 and tc.table_name = kcu.table_name
		where tc.constraint_type = 'PRIMARY KEY'
		  and tc.table_schema = $1
		  and tc.table_name = $2
		order by kcu.ordinal_position`, schema, table)
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
