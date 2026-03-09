package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const JSONRPCVersion = "2.0"

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type RPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type ToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func NewSuccess(id any, result any) RPCResponse {
	if result == nil {
		result = map[string]any{}
	}
	return RPCResponse{JSONRPC: JSONRPCVersion, ID: id, Result: result}
}

func NewError(id any, code int, message string) RPCResponse {
	if strings.TrimSpace(message) == "" {
		message = ""
	}
	return RPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}

func DecodeRequest(body []byte) (RPCRequest, error) {
	var req RPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return RPCRequest{}, err
	}
	return req, nil
}

func DecodeParams(raw json.RawMessage, out any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func ValidateRequest(req RPCRequest) error {
	if strings.TrimSpace(req.JSONRPC) != JSONRPCVersion {
		return fmt.Errorf("jsonrpc must be %s", JSONRPCVersion)
	}
	if strings.TrimSpace(req.Method) == "" {
		return fmt.Errorf("method is required")
	}
	return nil
}
