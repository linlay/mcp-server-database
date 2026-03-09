package database

import (
	"fmt"
	"strings"
)

type StatementKind string

const (
	StatementQuery StatementKind = "query"
	StatementExec  StatementKind = "exec"
	StatementDDL   StatementKind = "ddl"
)

type StatementInfo struct {
	Normalized string
	Keyword    string
	Kind       StatementKind
}

func ClassifyStatement(input string) (StatementInfo, error) {
	normalized, err := normalizeStatement(input)
	if err != nil {
		return StatementInfo{}, err
	}

	keyword := firstKeyword(normalized)
	if keyword == "" {
		return StatementInfo{}, fmt.Errorf("sql statement is empty")
	}

	info := StatementInfo{
		Normalized: normalized,
		Keyword:    keyword,
	}

	switch keyword {
	case "SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "PRAGMA", "WITH":
		info.Kind = StatementQuery
	case "INSERT", "UPDATE", "DELETE", "REPLACE":
		info.Kind = StatementExec
	case "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME":
		info.Kind = StatementDDL
	default:
		return StatementInfo{}, fmt.Errorf("unsupported sql statement type: %s", strings.ToLower(keyword))
	}
	return info, nil
}

func normalizeStatement(input string) (string, error) {
	var builder strings.Builder

	const (
		stateDefault = iota
		stateSingle
		stateDouble
		stateBacktick
		stateLineComment
		stateBlockComment
	)

	state := stateDefault
	for i := 0; i < len(input); i++ {
		ch := input[i]
		var next byte
		if i+1 < len(input) {
			next = input[i+1]
		}

		switch state {
		case stateDefault:
			switch {
			case ch == '\'':
				state = stateSingle
				builder.WriteByte(ch)
			case ch == '"':
				state = stateDouble
				builder.WriteByte(ch)
			case ch == '`':
				state = stateBacktick
				builder.WriteByte(ch)
			case ch == '-' && next == '-':
				state = stateLineComment
				i++
			case ch == '#':
				state = stateLineComment
			case ch == '/' && next == '*':
				state = stateBlockComment
				i++
			default:
				builder.WriteByte(ch)
			}
		case stateSingle:
			builder.WriteByte(ch)
			if ch == '\\' && i+1 < len(input) {
				i++
				builder.WriteByte(input[i])
				continue
			}
			if ch == '\'' {
				state = stateDefault
			}
		case stateDouble:
			builder.WriteByte(ch)
			if ch == '\\' && i+1 < len(input) {
				i++
				builder.WriteByte(input[i])
				continue
			}
			if ch == '"' {
				state = stateDefault
			}
		case stateBacktick:
			builder.WriteByte(ch)
			if ch == '`' {
				state = stateDefault
			}
		case stateLineComment:
			if ch == '\n' {
				state = stateDefault
				builder.WriteByte(ch)
			}
		case stateBlockComment:
			if ch == '*' && next == '/' {
				state = stateDefault
				i++
			}
		}
	}

	if state == stateBlockComment {
		return "", fmt.Errorf("sql contains unterminated block comment")
	}
	if state == stateSingle || state == stateDouble || state == stateBacktick {
		return "", fmt.Errorf("sql contains unterminated quoted string")
	}

	trimmed := strings.TrimSpace(builder.String())
	trimmed = strings.TrimRight(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", fmt.Errorf("sql statement is empty")
	}
	if hasMultipleStatements(trimmed) {
		return "", fmt.Errorf("multiple sql statements are not allowed")
	}
	return trimmed, nil
}

func hasMultipleStatements(input string) bool {
	state := byte(0)
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch state {
		case 0:
			switch ch {
			case '\'':
				state = '\''
			case '"':
				state = '"'
			case '`':
				state = '`'
			case ';':
				return true
			}
		case '\'', '"':
			if ch == '\\' && i+1 < len(input) {
				i++
				continue
			}
			if ch == state {
				state = 0
			}
		case '`':
			if ch == '`' {
				state = 0
			}
		}
	}
	return false
}

func firstKeyword(input string) string {
	input = strings.TrimSpace(input)
	start := -1
	end := -1
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if start == -1 {
			if isAlpha(ch) {
				start = i
			}
			continue
		}
		if !isAlpha(ch) {
			end = i
			break
		}
	}
	if start == -1 {
		return ""
	}
	if end == -1 {
		end = len(input)
	}
	return strings.ToUpper(input[start:end])
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
