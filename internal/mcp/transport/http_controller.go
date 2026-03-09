package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mcp-server-database/internal/mcp/jsonutil"
	"mcp-server-database/internal/mcp/protocol"
	"mcp-server-database/internal/mcp/tools"
	"mcp-server-database/internal/observability"
)

type Controller struct {
	dispatcher   *Dispatcher
	logger       *observability.Logger
	maxBodyBytes int64
}

func NewController(registry *tools.Registry, logger *observability.Logger, maxBodyBytes int64) *Controller {
	if maxBodyBytes <= 0 {
		maxBodyBytes = 1024 * 1024
	}
	if logger == nil {
		logger = observability.NopLogger()
	}
	return &Controller{
		dispatcher:   NewDispatcher(registry, logger),
		logger:       logger,
		maxBodyBytes: maxBodyBytes,
	}
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, c.maxBodyBytes))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		c.writeResponse(w, r, protocol.NewError(nil, protocol.ErrCodeInvalidRequest, "invalid request: empty body"), start, "")
		return
	}

	req, err := protocol.DecodeRequest(body)
	if err != nil {
		c.writeResponse(w, r, protocol.NewError(nil, protocol.ErrCodeParseError, "parse error: invalid json"), start, "")
		return
	}
	if err := protocol.ValidateRequest(req); err != nil {
		c.writeResponse(w, r, protocol.NewError(req.ID, protocol.ErrCodeInvalidRequest, "invalid request: "+err.Error()), start, req.Method)
		return
	}

	accept := r.Header.Get("Accept")
	stream := wantsStream(accept)
	params := decodeParamsSummary(req.Params)
	headers := singleValueHeaders(r.Header)
	c.logger.LogMCPRequest(req.ID, req.Method, params, accept, stream, headers)

	defer func() {
		if recovered := recover(); recovered != nil {
			c.logger.LogMCPError(req.ID, req.Method, time.Since(start), "panic", fmt.Sprint(recovered))
			c.writeResponse(w, r, protocol.NewError(req.ID, protocol.ErrCodeInternal, "internal server error"), start, req.Method)
		}
	}()

	resp := c.dispatcher.Dispatch(r.Context(), req, start)
	c.writeResponse(w, r, resp, start, req.Method)
}

func (c *Controller) writeResponse(w http.ResponseWriter, r *http.Request, response protocol.RPCResponse, start time.Time, method string) {
	encoded, err := json.Marshal(response)
	if err != nil {
		c.logger.LogMCPError(response.ID, method, time.Since(start), "marshal_error", err.Error())
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	stream := wantsStream(r.Header.Get("Accept"))
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: " + string(encoded) + "\n\n"))
		c.logger.LogMCPResponse(response.ID, method, jsonutil.ToMap(response), time.Since(start), "text/event-stream")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
	c.logger.LogMCPResponse(response.ID, method, jsonutil.ToMap(response), time.Since(start), "application/json")
}

func wantsStream(acceptHeader string) bool {
	if strings.TrimSpace(acceptHeader) == "" {
		return false
	}
	return strings.Contains(strings.ToLower(acceptHeader), "text/event-stream")
}

func singleValueHeaders(header http.Header) map[string]string {
	single := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			single[key] = ""
			continue
		}
		single[key] = values[0]
	}
	return single
}

func decodeParamsSummary(raw json.RawMessage) any {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
