package transport

import (
	"context"
	"errors"
	"strings"
	"time"

	"mcp-server-database/internal/mcp"
	"mcp-server-database/internal/mcp/protocol"
	"mcp-server-database/internal/mcp/tools"
	"mcp-server-database/internal/observability"
)

type Dispatcher struct {
	registry *tools.Registry
	logger   *observability.Logger
}

func NewDispatcher(registry *tools.Registry, logger *observability.Logger) *Dispatcher {
	if logger == nil {
		logger = observability.NopLogger()
	}
	return &Dispatcher{registry: registry, logger: logger}
}

func (d *Dispatcher) Dispatch(ctx context.Context, req protocol.RPCRequest, start time.Time) protocol.RPCResponse {
	switch req.Method {
	case "initialize":
		return protocol.NewSuccess(req.ID, map[string]any{
			"protocolVersion": mcp.MCPProtocolVersion,
			"serverInfo": map[string]any{
				"name":    mcp.ServerName,
				"version": mcp.ServerVersion,
			},
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
		})
	case "tools/list":
		if d.registry == nil {
			return protocol.NewSuccess(req.ID, map[string]any{"tools": []map[string]any{}})
		}
		return protocol.NewSuccess(req.ID, map[string]any{"tools": d.registry.ListTools()})
	case "tools/call":
		return d.dispatchToolsCall(ctx, req, start)
	default:
		return protocol.NewError(req.ID, protocol.ErrCodeMethodNotFound, "method not found: "+req.Method)
	}
}

func (d *Dispatcher) dispatchToolsCall(ctx context.Context, req protocol.RPCRequest, start time.Time) protocol.RPCResponse {
	params := protocol.ToolsCallParams{}
	if err := protocol.DecodeParams(req.Params, &params); err != nil {
		return protocol.NewError(req.ID, protocol.ErrCodeInvalidParams, "invalid params: expected object")
	}
	if strings.TrimSpace(params.Name) == "" {
		return protocol.NewError(req.ID, protocol.ErrCodeInvalidParams, "invalid params: name is required")
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	canonicalName := ""
	if d.registry != nil {
		if item, ok := d.registry.Find(params.Name); ok {
			canonicalName = item.Spec.Name
		}
	}
	d.logger.LogToolRequest(params.Name, canonicalName, params.Arguments)

	if d.registry == nil {
		result := tools.ErrorResult("unknown tool: " + strings.TrimSpace(params.Name))
		d.logger.LogToolError(params.Name, canonicalName, time.Since(start), result["error"].(string))
		return protocol.NewSuccess(req.ID, result)
	}

	result, err := d.registry.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		var validationErr *tools.ValidationError
		if errors.As(err, &validationErr) {
			d.logger.LogToolError(params.Name, canonicalName, time.Since(start), validationErr.Error())
			message := "invalid params"
			if validationErr.Unwrap() != nil {
				message += ": " + validationErr.Unwrap().Error()
			}
			return protocol.NewError(req.ID, protocol.ErrCodeInvalidParams, message)
		}

		var unknownErr *tools.UnknownToolError
		if errors.As(err, &unknownErr) {
			toolErr := tools.ErrorResult(unknownErr.Error())
			d.logger.LogToolError(params.Name, canonicalName, time.Since(start), unknownErr.Error())
			return protocol.NewSuccess(req.ID, toolErr)
		}

		toolErr := tools.ErrorResult(err.Error())
		d.logger.LogToolError(params.Name, canonicalName, time.Since(start), err.Error())
		return protocol.NewSuccess(req.ID, toolErr)
	}

	d.logger.LogToolResponse(canonicalName, result, time.Since(start))
	return protocol.NewSuccess(req.ID, result)
}
