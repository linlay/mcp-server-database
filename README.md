# mcp-server-database

`mcp-server-database` 是一个 Go 实现的 MCP Server，统一暴露 MySQL、SQLite、PostgreSQL 的数据库查询、写入、DDL 和元数据工具。

- MCP 入口：`POST /mcp`（HTTP）或 `stdio`
- 已实现工具：`db_list_connections`、`db_list_schemas`、`db_list_tables`、`db_describe_table`、`db_list_indexes`、`db_query`、`db_exec`、`db_ddl`
- 工具契约：`tools/*.yml`
- 协议版本：`2025-06`

## 快速开始

前置要求：

- Go `1.26+`
- Docker 和 Docker Compose（可选）

连接外部 MySQL / PostgreSQL：

```bash
cp .env.example .env
cp configs/local-mysql.example.yml configs/local-mysql.yml
cp configs/local-postgres.example.yml configs/local-postgres.yml
mkdir -p tmp
make run
```

将 `configs/local-mysql.yml`、`configs/local-postgres.yml` 里的 `dsn` 改成你自己的云数据库、本机数据库或其他现有数据库地址后，再启动服务。

容器方式启动 MCP 服务本体：

```bash
cp .env.example .env
cp configs/local-mysql.example.yml configs/local-mysql.yml
cp configs/local-postgres.example.yml configs/local-postgres.yml
mkdir -p tmp
make docker-up
```

快速本地验证：

```bash
cp .env.example .env
cp configs/local-sqlite.example.yml configs/local-sqlite.yml
mkdir -p tmp
make run
```

stdio 模式：

```bash
cp .env.example .env
cp configs/local-sqlite.example.yml configs/local-sqlite.yml
MCP_TRANSPORT=stdio make run
```

测试：

```bash
make test
```

## 配置说明

- 环境变量契约主维护文件是 `.env.example`，本地真实值写入 `.env`。
- 数据库连接示例模板放在 `configs/*.example.yml`，本地真实连接配置默认放在 `configs/`。
- `local-mysql.example.yml`、`local-postgres.example.yml` 使用外部数据库 DSN 占位符，复制后必须替换为真实地址和凭据。
- `local-sqlite.example.yml` 是零依赖示例，用于快速验证服务链路，不代表仅支持 SQLite。
- 配置优先级：代码默认值 < 环境变量。
- `DB_CONNECTIONS_CONFIG_PATH` 可显式指定连接配置目录或单个 YAML 文件；未设置时默认读取 `./configs`。
- 目录扫描会忽略 `*.example.yml` / `*.example.yaml` 模板文件，只加载真实连接配置。

常用变量分组：

- Server：`SERVER_PORT`、`SERVER_SHUTDOWN_TIMEOUT_SECONDS`
- MCP：`MCP_TRANSPORT`、`MCP_TOOLS_SPEC_LOCATION_PATTERN`、`MCP_HTTP_MAX_BODY_BYTES`
- MCP 限流：`MCP_RATE_LIMIT_ENABLED`、`MCP_RATE_LIMIT_RPS`、`MCP_RATE_LIMIT_BURST`
- Observability：`MCP_OBSERVABILITY_LOG_ENABLED`、`MCP_OBSERVABILITY_LOG_MAX_BODY_LENGTH`、`MCP_OBSERVABILITY_LOG_INCLUDE_HEADERS`
- Database：`DB_CONNECTIONS_CONFIG_PATH`、`DB_DEFAULT_QUERY_TIMEOUT_SECONDS`、`DB_MAX_RESULT_ROWS`、`DB_MAX_CELL_BYTES`
- Compose：`HOST_PORT`

连接配置文件字段：

- 一个数据库连接对应一个 `yml/yaml` 文件。
- `name`：MCP 工具里使用的连接名
- `description`：面向客户端的人类可读描述
- `driver`：`mysql`、`postgresql`、`sqlite`
- `dsn`：目标数据库原生 DSN
- `allow_write`：是否允许 `db_exec`
- `allow_ddl`：是否允许 `db_ddl`

## 主要 API 用例

以下示例默认服务运行在 `http://localhost:11965`（基于 `.env.example` 默认值）。

### 初始化握手

```bash
curl -sS -X POST http://localhost:11968/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "init-1",
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-06",
      "capabilities": {},
      "clientInfo": {
        "name": "curl",
        "version": "1.0.0"
      }
    }
  }'
```

### 查看工具列表

```bash
curl -sS -X POST http://localhost:11968/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":"tools-list-1","method":"tools/list","params":{}}'
```

### 列出已配置连接

```bash
curl -sS -X POST http://localhost:11968/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "conn-1",
    "method": "tools/call",
    "params": {
      "name": "db_list_connections",
      "arguments": {}
    }
  }'
```

### 查询表数据

```bash
curl -sS -X POST http://localhost:11968/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "query-1",
    "method": "tools/call",
    "params": {
      "name": "db_query",
      "arguments": {
        "connection_name": "local-sqlite",
        "sql": "select id, name from demo_users where id > ?",
        "args": [10],
        "max_rows": 20
      }
    }
  }'
```

### 执行写入

```bash
curl -sS -X POST http://localhost:11968/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "exec-1",
    "method": "tools/call",
    "params": {
      "name": "db_exec",
      "arguments": {
        "connection_name": "local-postgres",
        "sql": "insert into demo_users(name) values ($1)",
        "args": ["alice"]
      }
    }
  }'
```

### 执行 DDL

```bash
curl -sS -X POST http://localhost:11968/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "ddl-1",
    "method": "tools/call",
    "params": {
      "name": "db_ddl",
      "arguments": {
        "connection_name": "local-mysql",
        "sql": "create table if not exists demo_users (id bigint primary key auto_increment, name varchar(255) not null)"
      }
    }
  }'
```

## 开发说明

- 每次工具调用只执行一条 SQL，不支持跨请求事务。
- `db_query` 只允许只读语句；`db_exec` 只允许 DML；`db_ddl` 只允许 DDL。
- 工具使用目标数据库原生 SQL 和原生占位符风格，不做跨方言改写。
