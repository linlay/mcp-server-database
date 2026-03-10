# CLAUDE.md

## 1. 项目概览

`mcp-server-database` 是一个面向 MCP 客户端的数据库服务端实现，统一暴露 MySQL、SQLite、PostgreSQL 的元数据查询、只读查询、写入和 DDL 工具。

- 项目状态：开发中
- 主要场景：MCP 客户端统一接入多个数据库连接
- 传输模式：HTTP JSON-RPC 和 stdio

## 2. 技术栈

- 语言：Go `1.26`
- 协议：JSON-RPC 2.0
- 运行形态：单进程后端服务
- 主要依赖：
  - `gopkg.in/yaml.v3`：解析连接配置和工具定义
  - `github.com/santhosh-tekuri/jsonschema/v5`：工具参数 schema 校验
  - `database/sql` + 驱动：MySQL、pgx、SQLite
- 部署方式：二进制或 Docker / Docker Compose 启动 MCP 服务本体；数据库实例由外部提供

## 3. 架构设计

- 架构风格：单体 Go 服务，按入口、协议、业务、可观测性分层
- 分层职责：
  - `cmd/mcp-server`：进程入口、传输模式切换、HTTP Server 生命周期
  - `internal/config`：环境变量加载和默认值决策
  - `internal/mcp`：工具注册、schema 校验、JSON-RPC 传输控制
  - `internal/database`：连接配置、连接池、SQL 分类、方言元数据查询
  - `internal/observability`：日志输出和敏感字段脱敏
- 关键约束：
  - `tools/*.yml` 是工具定义单一事实源
  - 连接配置默认放在 `configs/`，一个连接一个 YAML 文件
  - 每次工具调用只执行一条 SQL

## 4. 目录结构

- `cmd/`：程序入口
- `internal/config/`：配置加载
- `internal/database/`：数据库连接和元数据逻辑
- `internal/mcp/`：MCP 协议和工具注册
- `internal/observability/`：日志和脱敏
- `tools/`：MCP 工具定义 YAML
- `configs/`：受版本管理的结构化配置模板

## 5. 数据结构

- `config.Config`：聚合 `Server`、`MCP`、`Observability`、`Database` 配置
- `database.ConnectionConfig`：单个命名连接的驱动、DSN 和权限
- `database.QueryResult`：查询结果列、行、计数、截断标记、耗时
- `database.ExecResult`：受影响行数、可选 `last_insert_id`、耗时
- `database.DDLResult`：DDL 类型和耗时

## 6. API 定义

- HTTP 路由：
  - `POST /mcp`：MCP JSON-RPC 入口
- JSON-RPC 方法：
  - `initialize`
  - `tools/list`
  - `tools/call`
- `tools/list` 扩展字段：
  - `label`：工具的人类可读名称，适合前端直接展示中文名
  - `toolAction: true`：action 工具
  - `toolType` + `viewportKey`：frontend 工具
  - 当前服务已为内置工具声明 `label`，其余内置工具仍按 backend 工具处理
- 鉴权现状：
  - 当前 `/mcp` 无内建鉴权，依赖外层网络边界控制

## 7. 开发要点

- 入口与业务分离：`cmd` 只做启动，业务逻辑进入 `internal`
- 配置治理：
  - `.env.example` 维护环境变量契约
  - `.env`、`configs/` 均不得提交，`configs/*.example.yml` 可以提交
  - 配置优先级为代码默认值 < 环境变量
  - MySQL / PostgreSQL 示例配置只提供外部 DSN 模板，不内置数据库容器
- 日志治理：
  - 统一通过 `internal/observability` 输出
  - 默认脱敏 `dsn`、`password`、`token` 等敏感字段
- 工程命令统一由 `Makefile` 暴露：`build`、`run`、`test`、`test-integration`、`docker-build`、`docker-up`、`docker-down`、`clean`
