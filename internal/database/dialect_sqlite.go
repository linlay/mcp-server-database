package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type sqliteDialect struct{}

func (sqliteDialect) Probe(ctx context.Context, db *sql.DB, _ ConnectionConfig) error {
	var ready int
	if err := db.QueryRowContext(ctx, `select 1`).Scan(&ready); err != nil {
		return err
	}
	return nil
}

func (sqliteDialect) ResolveDefaultSchema(context.Context, *sql.DB, ConnectionConfig) (string, error) {
	return "main", nil
}

func (sqliteDialect) ListSchemas(ctx context.Context, db *sql.DB, _ ConnectionConfig) ([]SchemaInfo, error) {
	rows, err := db.QueryContext(ctx, `pragma database_list`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SchemaInfo
	for rows.Next() {
		var seq int
		var name string
		var file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return nil, err
		}
		out = append(out, SchemaInfo{Name: name})
	}
	return out, rows.Err()
}

func (d sqliteDialect) ListTables(ctx context.Context, db *sql.DB, _ ConnectionConfig, schema string) ([]TableInfo, error) {
	schema = strings.TrimSpace(schema)
	query := fmt.Sprintf(`
		select '%s' as table_schema, name as table_name, upper(type) as table_type
		from %s.sqlite_master
		where type in ('table', 'view')
		  and name not like 'sqlite_%%'
		order by name`, schema, quoteSQLiteIdent(schema))
	rows, err := db.QueryContext(ctx, query)
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

func (d sqliteDialect) DescribeTable(ctx context.Context, db *sql.DB, _ ConnectionConfig, schema string, table string) (TableDescription, error) {
	schema = strings.TrimSpace(schema)
	desc := TableDescription{
		Schema: schema,
		Name:   table,
		Type:   "TABLE",
	}

	columnQuery := fmt.Sprintf(`pragma %s.table_info(%s)`, quoteSQLiteIdent(schema), quoteSQLiteString(table))
	rows, err := db.QueryContext(ctx, columnQuery)
	if err != nil {
		return TableDescription{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return TableDescription{}, err
		}
		column := ColumnInfo{
			Name:         name,
			DataType:     dataType,
			Nullable:     notNull == 0,
			Position:     cid + 1,
			IsPrimaryKey: pk > 0,
		}
		if defaultValue.Valid {
			column.DefaultValue = defaultValue.String
		}
		if pk > 0 {
			desc.PrimaryKey = append(desc.PrimaryKey, name)
		}
		desc.Columns = append(desc.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return TableDescription{}, err
	}
	if len(desc.Columns) == 0 {
		return TableDescription{}, fmt.Errorf("table not found: %s.%s", schema, table)
	}

	indexes, err := d.ListIndexes(ctx, db, ConnectionConfig{}, schema, table)
	if err != nil {
		return TableDescription{}, err
	}
	desc.Indexes = indexes
	return desc, nil
}

func (sqliteDialect) ListIndexes(ctx context.Context, db *sql.DB, _ ConnectionConfig, schema string, table string) ([]IndexInfo, error) {
	schema = strings.TrimSpace(schema)
	tables := []string{}
	if strings.TrimSpace(table) != "" {
		tables = append(tables, strings.TrimSpace(table))
	} else {
		list, err := sqliteDialect{}.ListTables(ctx, db, ConnectionConfig{}, schema)
		if err != nil {
			return nil, err
		}
		for _, item := range list {
			if item.Type == "TABLE" {
				tables = append(tables, item.Name)
			}
		}
	}

	out := make([]IndexInfo, 0)
	for _, tableName := range tables {
		indexListQuery := fmt.Sprintf(`pragma %s.index_list(%s)`, quoteSQLiteIdent(schema), quoteSQLiteString(tableName))
		rows, err := db.QueryContext(ctx, indexListQuery)
		if err != nil {
			return nil, err
		}
		type sqliteIndexRow struct {
			name   string
			unique int
			origin string
		}
		indexRows := make([]sqliteIndexRow, 0)
		for rows.Next() {
			var seq int
			var indexName string
			var unique int
			var origin string
			var partial int
			if err := rows.Scan(&seq, &indexName, &unique, &origin, &partial); err != nil {
				rows.Close()
				return nil, err
			}
			_ = partial
			indexRows = append(indexRows, sqliteIndexRow{name: indexName, unique: unique, origin: origin})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()

		for _, item := range indexRows {
			columnRows, err := db.QueryContext(ctx, fmt.Sprintf(`pragma %s.index_info(%s)`, quoteSQLiteIdent(schema), quoteSQLiteString(item.name)))
			if err != nil {
				return nil, err
			}
			columns := []string{}
			for columnRows.Next() {
				var seqno int
				var cid int
				var columnName string
				if err := columnRows.Scan(&seqno, &cid, &columnName); err != nil {
					columnRows.Close()
					return nil, err
				}
				columns = append(columns, columnName)
			}
			if err := columnRows.Err(); err != nil {
				columnRows.Close()
				return nil, err
			}
			columnRows.Close()

			out = append(out, IndexInfo{
				Schema:     schema,
				Table:      tableName,
				Name:       item.name,
				Columns:    columns,
				Unique:     item.unique == 1,
				Primary:    item.origin == "pk",
				Definition: item.origin,
			})
		}
	}
	return out, nil
}

func quoteSQLiteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quoteSQLiteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
