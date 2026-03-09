package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type manager struct {
	cfg         Config
	connections map[string]*managedConnection
}

type managedConnection struct {
	cfg     ConnectionConfig
	dialect dialect
	driver  string

	mu sync.Mutex
	db *sql.DB
}

type dialect interface {
	DefaultSchema(cfg ConnectionConfig) string
	ListSchemas(ctx context.Context, db *sql.DB, cfg ConnectionConfig) ([]SchemaInfo, error)
	ListTables(ctx context.Context, db *sql.DB, cfg ConnectionConfig, schema string) ([]TableInfo, error)
	DescribeTable(ctx context.Context, db *sql.DB, cfg ConnectionConfig, schema string, table string) (TableDescription, error)
	ListIndexes(ctx context.Context, db *sql.DB, cfg ConnectionConfig, schema string, table string) ([]IndexInfo, error)
}

func NewService(cfg Config) (Service, error) {
	if strings.TrimSpace(cfg.ConnectionsConfigPath) == "" {
		cfg.ConnectionsConfigPath = "./configs"
	}
	if cfg.DefaultQueryTimeout <= 0 {
		cfg.DefaultQueryTimeout = 15 * time.Second
	}
	if cfg.MaxResultRows <= 0 {
		cfg.MaxResultRows = 200
	}
	if cfg.MaxCellBytes <= 0 {
		cfg.MaxCellBytes = 4096
	}

	catalog, err := LoadCatalog(cfg.ConnectionsConfigPath)
	if err != nil {
		return nil, err
	}

	out := &manager{
		cfg:         cfg,
		connections: make(map[string]*managedConnection, len(catalog.Connections)),
	}
	for _, item := range catalog.Connections {
		dialect, driverName, err := resolveDialect(item)
		if err != nil {
			return nil, err
		}
		out.connections[strings.ToLower(item.Name)] = &managedConnection{
			cfg:     item,
			dialect: dialect,
			driver:  driverName,
		}
	}
	return out, nil
}

func (m *manager) Close() error {
	var firstErr error
	for _, conn := range m.connections {
		conn.mu.Lock()
		db := conn.db
		conn.db = nil
		conn.mu.Unlock()
		if db != nil {
			if err := db.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (m *manager) ListConnections(ctx context.Context) ([]ConnectionSummary, error) {
	items := make([]ConnectionSummary, 0, len(m.connections))
	for _, conn := range m.sortedConnections() {
		status := "ready"
		pingCtx, cancel := context.WithTimeout(ctx, time.Second)
		if err := conn.ping(pingCtx); err != nil {
			status = "error"
		}
		cancel()

		items = append(items, ConnectionSummary{
			Name:        conn.cfg.Name,
			Description: conn.cfg.Description,
			Driver:      conn.cfg.Driver,
			AllowWrite:  conn.cfg.AllowWrite,
			AllowDDL:    conn.cfg.AllowDDL,
			Status:      status,
		})
	}
	return items, nil
}

func (m *manager) ListSchemas(ctx context.Context, connectionName string) ([]SchemaInfo, error) {
	conn, err := m.connection(connectionName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()
	db, err := conn.open()
	if err != nil {
		return nil, err
	}
	return conn.dialect.ListSchemas(ctx, db, conn.cfg)
}

func (m *manager) ListTables(ctx context.Context, connectionName string, schema string) ([]TableInfo, error) {
	conn, err := m.connection(connectionName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()
	db, err := conn.open()
	if err != nil {
		return nil, err
	}
	return conn.dialect.ListTables(ctx, db, conn.cfg, normalizeSchema(schema, conn.dialect.DefaultSchema(conn.cfg)))
}

func (m *manager) DescribeTable(ctx context.Context, connectionName string, schema string, table string) (TableDescription, error) {
	conn, err := m.connection(connectionName)
	if err != nil {
		return TableDescription{}, err
	}
	if strings.TrimSpace(table) == "" {
		return TableDescription{}, fmt.Errorf("table is required")
	}
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()
	db, err := conn.open()
	if err != nil {
		return TableDescription{}, err
	}
	return conn.dialect.DescribeTable(ctx, db, conn.cfg, normalizeSchema(schema, conn.dialect.DefaultSchema(conn.cfg)), strings.TrimSpace(table))
}

func (m *manager) ListIndexes(ctx context.Context, connectionName string, schema string, table string) ([]IndexInfo, error) {
	conn, err := m.connection(connectionName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()
	db, err := conn.open()
	if err != nil {
		return nil, err
	}
	return conn.dialect.ListIndexes(ctx, db, conn.cfg, normalizeSchema(schema, conn.dialect.DefaultSchema(conn.cfg)), strings.TrimSpace(table))
}

func (m *manager) Query(ctx context.Context, req QueryRequest) (QueryResult, error) {
	conn, err := m.connection(req.ConnectionName)
	if err != nil {
		return QueryResult{}, err
	}
	info, err := ClassifyStatement(req.SQL)
	if err != nil {
		return QueryResult{}, err
	}
	if info.Kind != StatementQuery {
		return QueryResult{}, fmt.Errorf("db_query only allows read-only statements")
	}
	db, ctx, cancel, err := m.prepareCall(ctx, conn)
	if err != nil {
		return QueryResult{}, err
	}
	defer cancel()

	start := time.Now()
	rows, err := db.QueryContext(ctx, info.Normalized, req.Args...)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}

	limit := req.MaxRows
	if limit <= 0 || limit > m.cfg.MaxResultRows {
		limit = m.cfg.MaxResultRows
	}

	result := QueryResult{
		Columns: columns,
		Rows:    make([]map[string]any, 0),
	}

	for rows.Next() {
		if len(result.Rows) >= limit {
			result.Truncated = true
			break
		}

		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return QueryResult{}, err
		}

		row := make(map[string]any, len(columns))
		for i, column := range columns {
			value, truncated := normalizeCellValue(values[i], m.cfg.MaxCellBytes)
			if truncated {
				result.Truncated = true
			}
			row[column] = value
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}

	result.RowCount = len(result.Rows)
	result.ElapsedMs = time.Since(start).Milliseconds()
	return result, nil
}

func (m *manager) Exec(ctx context.Context, req ExecRequest) (ExecResult, error) {
	conn, err := m.connection(req.ConnectionName)
	if err != nil {
		return ExecResult{}, err
	}
	if !conn.cfg.AllowWrite {
		return ExecResult{}, fmt.Errorf("connection %s does not allow write operations", conn.cfg.Name)
	}
	info, err := ClassifyStatement(req.SQL)
	if err != nil {
		return ExecResult{}, err
	}
	if info.Kind != StatementExec {
		return ExecResult{}, fmt.Errorf("db_exec only allows insert, update, delete, or replace statements")
	}
	db, ctx, cancel, err := m.prepareCall(ctx, conn)
	if err != nil {
		return ExecResult{}, err
	}
	defer cancel()

	start := time.Now()
	sqlResult, err := db.ExecContext(ctx, info.Normalized, req.Args...)
	if err != nil {
		return ExecResult{}, err
	}

	affected, err := sqlResult.RowsAffected()
	if err != nil {
		return ExecResult{}, err
	}
	out := ExecResult{
		AffectedRows: affected,
		ElapsedMs:    time.Since(start).Milliseconds(),
	}
	if lastInsertID, err := sqlResult.LastInsertId(); err == nil {
		out.LastInsertID = &lastInsertID
	}
	return out, nil
}

func (m *manager) DDL(ctx context.Context, req DDLRequest) (DDLResult, error) {
	conn, err := m.connection(req.ConnectionName)
	if err != nil {
		return DDLResult{}, err
	}
	if !conn.cfg.AllowDDL {
		return DDLResult{}, fmt.Errorf("connection %s does not allow ddl operations", conn.cfg.Name)
	}
	info, err := ClassifyStatement(req.SQL)
	if err != nil {
		return DDLResult{}, err
	}
	if info.Kind != StatementDDL {
		return DDLResult{}, fmt.Errorf("db_ddl only allows ddl statements")
	}
	db, ctx, cancel, err := m.prepareCall(ctx, conn)
	if err != nil {
		return DDLResult{}, err
	}
	defer cancel()

	start := time.Now()
	if _, err := db.ExecContext(ctx, info.Normalized); err != nil {
		return DDLResult{}, err
	}
	return DDLResult{
		StatementType: strings.ToLower(info.Keyword),
		ElapsedMs:     time.Since(start).Milliseconds(),
	}, nil
}

func (m *manager) prepareCall(ctx context.Context, conn *managedConnection) (*sql.DB, context.Context, context.CancelFunc, error) {
	db, err := conn.open()
	if err != nil {
		return nil, nil, nil, err
	}
	callCtx, cancel := m.withTimeout(ctx)
	return db, callCtx, cancel, nil
}

func (m *manager) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, m.cfg.DefaultQueryTimeout)
}

func (m *manager) connection(name string) (*managedConnection, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil, fmt.Errorf("connection_name is required")
	}
	conn, ok := m.connections[key]
	if !ok {
		return nil, fmt.Errorf("unknown connection: %s", strings.TrimSpace(name))
	}
	return conn, nil
}

func (m *manager) sortedConnections() []*managedConnection {
	items := make([]*managedConnection, 0, len(m.connections))
	for _, item := range m.connections {
		items = append(items, item)
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if strings.ToLower(items[j].cfg.Name) < strings.ToLower(items[i].cfg.Name) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items
}

func (c *managedConnection) open() (*sql.DB, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db != nil {
		return c.db, nil
	}
	dsn := c.cfg.DSN
	if c.cfg.Driver == "sqlite" {
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open(c.driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open connection %s: %w", c.cfg.Name, err)
	}
	if c.cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(c.cfg.MaxOpenConns)
	}
	if c.cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(c.cfg.MaxIdleConns)
	}
	if c.cfg.ConnMaxLifetimeSeconds > 0 {
		db.SetConnMaxLifetime(time.Duration(c.cfg.ConnMaxLifetimeSeconds) * time.Second)
	}
	c.db = db
	return db, nil
}

func (c *managedConnection) ping(ctx context.Context) error {
	db, err := c.open()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

func resolveDialect(cfg ConnectionConfig) (dialect, string, error) {
	switch cfg.Driver {
	case "mysql":
		return mysqlDialect{}, "mysql", nil
	case "postgresql":
		return postgresDialect{}, "pgx", nil
	case "sqlite":
		return sqliteDialect{}, "sqlite", nil
	default:
		return nil, "", fmt.Errorf("connection %s has unsupported driver", cfg.Name)
	}
}

func normalizeSchema(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func ensureSQLiteDir(dsn string) error {
	path := sqlitePathFromDSN(dsn)
	if path == "" || path == ":memory:" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func sqlitePathFromDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(trimmed, "file:") {
		trimmed = strings.TrimPrefix(trimmed, "file:")
		if idx := strings.Index(trimmed, "?"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		return trimmed
	}
	return trimmed
}

func mysqlDatabaseName(dsn string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.DBName)
}

func normalizeCellValue(value any, maxBytes int) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case []byte:
		return truncateUTF8(string(typed), maxBytes)
	case string:
		return truncateUTF8(typed, maxBytes)
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano), false
	default:
		return value, false
	}
}

func truncateUTF8(input string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(input) <= maxBytes {
		return input, false
	}
	bytes := []byte(input)
	if len(bytes) <= maxBytes {
		return input, false
	}
	cut := maxBytes
	for cut > 0 && !utf8.Valid(bytes[:cut]) {
		cut--
	}
	if cut == 0 {
		return "", true
	}
	return string(bytes[:cut]) + "...(truncated)", true
}
