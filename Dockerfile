FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mcp-server ./cmd/mcp-server

FROM alpine:3.21

WORKDIR /app
RUN adduser -D -u 10001 appuser

COPY --from=builder /out/mcp-server /app/mcp-server
COPY --chown=appuser:appuser tools /app/tools
COPY --chown=appuser:appuser configs /app/configs

ENV SERVER_PORT=8080 \
    MCP_HTTP_MAX_BODY_BYTES=1048576 \
    MCP_TOOLS_SPEC_LOCATION_PATTERN=./tools/*.yml \
    MCP_OBSERVABILITY_LOG_ENABLED=true \
    MCP_OBSERVABILITY_LOG_MAX_BODY_LENGTH=2000 \
    MCP_OBSERVABILITY_LOG_INCLUDE_HEADERS=false \
    DB_CONNECTIONS_CONFIG_PATH=./configs/databases \
    DB_DEFAULT_QUERY_TIMEOUT_SECONDS=15 \
    DB_MAX_RESULT_ROWS=200 \
    DB_MAX_CELL_BYTES=4096

EXPOSE 8080

USER appuser
ENTRYPOINT ["/app/mcp-server"]
