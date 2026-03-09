package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"mcp-server-database/internal/mcp/jsonutil"
	"mcp-server-database/internal/mcp/protocol"
	"mcp-server-database/internal/mcp/tools"
	"mcp-server-database/internal/observability"
)

type StdioController struct {
	dispatcher *Dispatcher
	logger     *observability.Logger
	reader     *bufio.Reader
	writer     io.Writer
	writeMu    sync.Mutex
}

func NewStdioController(registry *tools.Registry, logger *observability.Logger, input io.Reader, output io.Writer) *StdioController {
	if logger == nil {
		logger = observability.NopLogger()
	}
	if input == nil {
		input = strings.NewReader("")
	}
	if output == nil {
		output = io.Discard
	}
	return &StdioController{
		dispatcher: NewDispatcher(registry, logger),
		logger:     logger,
		reader:     bufio.NewReader(input),
		writer:     output,
	}
}

func (c *StdioController) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()
		payload, err := c.readFrame()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		resp := c.handlePayload(ctx, payload, start)
		if err := c.writeFrame(resp); err != nil {
			return err
		}
	}
}

func (c *StdioController) handlePayload(ctx context.Context, payload []byte, start time.Time) (resp protocol.RPCResponse) {
	if len(strings.TrimSpace(string(payload))) == 0 {
		return protocol.NewError(nil, protocol.ErrCodeInvalidRequest, "invalid request: empty body")
	}

	req, err := protocol.DecodeRequest(payload)
	if err != nil {
		return protocol.NewError(nil, protocol.ErrCodeParseError, "parse error: invalid json")
	}
	if err := protocol.ValidateRequest(req); err != nil {
		return protocol.NewError(req.ID, protocol.ErrCodeInvalidRequest, "invalid request: "+err.Error())
	}

	params := decodeParamsSummary(req.Params)
	c.logger.LogMCPRequest(req.ID, req.Method, params, "stdio", false, map[string]string{})

	defer func() {
		if recovered := recover(); recovered != nil {
			c.logger.LogMCPError(req.ID, req.Method, time.Since(start), "panic", fmt.Sprint(recovered))
			resp = protocol.NewError(req.ID, protocol.ErrCodeInternal, "internal server error")
		}
	}()

	resp = c.dispatcher.Dispatch(ctx, req, start)
	c.logger.LogMCPResponse(resp.ID, req.Method, jsonutil.ToMap(resp), time.Since(start), "application/json")
	return resp
}

func (c *StdioController) readFrame() ([]byte, error) {
	contentLength := -1

	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid stdio header: %q", trimmed)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if strings.EqualFold(name, "Content-Length") {
			size, err := strconv.Atoi(value)
			if err != nil || size < 0 {
				return nil, fmt.Errorf("invalid Content-Length: %q", value)
			}
			contentLength = size
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *StdioController) writeFrame(resp protocol.RPCResponse) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if _, err := fmt.Fprintf(c.writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = c.writer.Write(payload)
	return err
}
