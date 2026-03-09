package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mcp-server-database/internal/config"
	"mcp-server-database/internal/database"
	"mcp-server-database/internal/mcp/tools"
	"mcp-server-database/internal/mcp/transport"
	"mcp-server-database/internal/observability"
)

func main() {
	cfg := config.Load()
	std := log.Default()

	service, err := database.NewService(database.Config{
		ConnectionsConfigPath: cfg.Database.ConnectionsConfigPath,
		DefaultQueryTimeout:   time.Duration(cfg.Database.DefaultQueryTimeoutSeconds) * time.Second,
		MaxResultRows:         cfg.Database.MaxResultRows,
		MaxCellBytes:          cfg.Database.MaxCellBytes,
	})
	if err != nil {
		std.Fatalf("failed to initialize database service: %v", err)
	}
	defer service.Close()

	sanitizer := observability.NewLogSanitizer(cfg.Observability.LogMaxBodyLength)
	obsLogger := observability.NewLogger(std, cfg.Observability, sanitizer)
	registry, err := tools.NewRegistry(cfg.MCP.ToolsSpecLocationPattern, tools.BuiltinHandlers(service), std)
	if err != nil {
		std.Fatalf("failed to initialize tool registry: %v", err)
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.MCP.Transport))
	if mode == "stdio" {
		stdio := transport.NewStdioController(registry, obsLogger, os.Stdin, os.Stdout)
		std.Printf("event=server.start transport=stdio")
		if err := stdio.Serve(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			std.Fatalf("stdio server failed: %v", err)
		}
		return
	}

	controller := transport.NewController(registry, obsLogger, cfg.MCP.HTTPMaxBodyBytes)
	mcpHandler := transport.WithRateLimit(controller, transport.RateLimitConfig{
		Enabled: cfg.MCP.RateLimit.Enabled,
		RPS:     cfg.MCP.RateLimit.RPS,
		Burst:   cfg.MCP.RateLimit.Burst,
	}, obsLogger)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{Addr: addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	std.Printf("event=server.start port=%d transport=http", cfg.Server.Port)
	if err := runHTTPServer(ctx, server, time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second); err != nil {
		std.Fatalf("server failed: %v", err)
	}
}

func runHTTPServer(ctx context.Context, server *http.Server, shutdownTimeout time.Duration) error {
	if server == nil {
		return fmt.Errorf("http server is nil")
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
