package observability

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"mcp-server-database/internal/config"
)

type Logger struct {
	std       *log.Logger
	props     config.ObservabilityConfig
	sanitizer *LogSanitizer
}

func NewLogger(std *log.Logger, props config.ObservabilityConfig, sanitizer *LogSanitizer) *Logger {
	if std == nil {
		std = log.Default()
	}
	if sanitizer == nil {
		sanitizer = NewLogSanitizer(props.LogMaxBodyLength)
	}
	return &Logger{std: std, props: props, sanitizer: sanitizer}
}

func NopLogger() *Logger {
	return &Logger{
		std:       log.New(io.Discard, "", 0),
		props:     config.ObservabilityConfig{LogEnabled: false},
		sanitizer: NewLogSanitizer(0),
	}
}

func (l *Logger) LogMCPRequest(id any, method string, params any, acceptHeader string, stream bool, headers map[string]string) {
	if !l.props.LogEnabled {
		return
	}
	if l.props.LogIncludeHeaders {
		l.std.Printf(
			"event=mcp.request rpcId=%s method=%s accept=%s isSse=%t paramsSummary=%s headersSummary=%s",
			nodeText(id),
			emptyIfBlank(method),
			emptyIfBlank(acceptHeader),
			stream,
			l.sanitizer.SummarizeJSON(params),
			l.sanitizer.SummarizeObject(headers),
		)
		return
	}
	l.std.Printf(
		"event=mcp.request rpcId=%s method=%s accept=%s isSse=%t paramsSummary=%s",
		nodeText(id),
		emptyIfBlank(method),
		emptyIfBlank(acceptHeader),
		stream,
		l.sanitizer.SummarizeJSON(params),
	)
}

func (l *Logger) LogMCPResponse(id any, method string, response any, duration time.Duration, contentType string) {
	if !l.props.LogEnabled {
		return
	}
	l.std.Printf(
		"event=mcp.response rpcId=%s method=%s success=%t durationMs=%d contentType=%s responseSummary=%s",
		nodeText(id),
		emptyIfBlank(method),
		isResponseSuccess(response),
		duration.Milliseconds(),
		contentType,
		l.sanitizer.SummarizeJSON(response),
	)
}

func (l *Logger) LogMCPError(id any, method string, duration time.Duration, errorType, errorMessage string) {
	if !l.props.LogEnabled {
		return
	}
	l.std.Printf(
		"event=mcp.error rpcId=%s method=%s durationMs=%d errorType=%s errorMessage=%s",
		nodeText(id),
		emptyIfBlank(method),
		duration.Milliseconds(),
		emptyIfBlank(errorType),
		emptyIfBlank(errorMessage),
	)
}

func (l *Logger) LogToolRequest(rawToolName, toolName string, args map[string]any) {
	if !l.props.LogEnabled {
		return
	}
	l.std.Printf(
		"event=tool.call.request toolRawName=%s toolCanonicalName=%s argsSummary=%s",
		emptyIfBlank(rawToolName),
		emptyIfBlank(toolName),
		l.sanitizer.SummarizeObject(args),
	)
}

func (l *Logger) LogToolResponse(toolName string, result map[string]any, duration time.Duration) {
	if !l.props.LogEnabled {
		return
	}
	structured := map[string]any{}
	if result != nil {
		if raw, ok := result["structuredContent"]; ok {
			if parsed, ok := raw.(map[string]any); ok {
				structured = parsed
			}
		}
	}
	l.std.Printf(
		"event=tool.call.response toolCanonicalName=%s isError=%t durationMs=%d structuredSummary=%s",
		emptyIfBlank(toolName),
		result != nil && asBool(result["isError"]),
		duration.Milliseconds(),
		l.sanitizer.SummarizeJSON(structured),
	)
}

func (l *Logger) LogToolError(rawToolName, toolName string, duration time.Duration, err string) {
	if !l.props.LogEnabled {
		return
	}
	l.std.Printf(
		"event=tool.call.error toolRawName=%s toolCanonicalName=%s durationMs=%d error=%s",
		emptyIfBlank(rawToolName),
		emptyIfBlank(toolName),
		duration.Milliseconds(),
		emptyIfBlank(err),
	)
}

func nodeText(value any) string {
	if value == nil {
		return "null"
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func emptyIfBlank(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

func isResponseSuccess(response any) bool {
	root, ok := response.(map[string]any)
	if !ok {
		return false
	}
	if _, hasError := root["error"]; hasError {
		return false
	}
	rawResult, hasResult := root["result"]
	if !hasResult {
		return false
	}
	result, ok := rawResult.(map[string]any)
	if !ok {
		return true
	}
	if rawIsError, hasIsError := result["isError"]; hasIsError {
		return !asBool(rawIsError)
	}
	return true
}

func asBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}
