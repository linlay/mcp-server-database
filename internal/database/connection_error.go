package database

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var (
	urlUserInfoPattern        = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:/\s?#]+):([^@/\s?#]+)@`)
	mysqlUserInfoPattern      = regexp.MustCompile(`([^\s:@]+):(.+?)@((?:tcp|unix)\()`)
	passwordAssignmentPattern = regexp.MustCompile(`(?i)\b(password|passwd|pwd)\s*[:=]\s*([^\s,;]+)`)
)

func sanitizeConnectionError(cfg ConnectionConfig, err error) string {
	if err == nil {
		return ""
	}
	return sanitizeConnectionErrorMessage(cfg, err.Error())
}

func sanitizeConnectionErrorMessage(cfg ConnectionConfig, message string) string {
	safe := strings.TrimSpace(message)
	if safe == "" {
		return ""
	}

	if cfg.DSN != "" {
		safe = strings.ReplaceAll(safe, cfg.DSN, "[dsn]")
	}

	safe = passwordAssignmentPattern.ReplaceAllString(safe, "$1=***")
	safe = urlUserInfoPattern.ReplaceAllString(safe, `$1:***@`)
	safe = mysqlUserInfoPattern.ReplaceAllString(safe, `$1:***@$3`)

	switch cfg.Driver {
	case "mysql":
		safe = sanitizeMySQLError(cfg.DSN, safe)
	case "postgresql":
		safe = sanitizePostgresError(cfg.DSN, safe)
	}

	return safe
}

func sanitizeMySQLError(dsn string, message string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return message
	}

	token := cfg.User
	if cfg.Passwd != "" {
		token += ":" + cfg.Passwd
	}
	if token != "" {
		message = strings.ReplaceAll(message, token+"@", cfg.User+":***@")
	}
	if cfg.Passwd != "" {
		message = strings.ReplaceAll(message, "password "+cfg.Passwd, "password ***")
		message = strings.ReplaceAll(message, "password="+cfg.Passwd, "password=***")
	}
	return message
}

func sanitizePostgresError(dsn string, message string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.User == nil {
		return message
	}

	password, hasPassword := parsed.User.Password()
	if !hasPassword {
		return message
	}

	username := parsed.User.Username()
	message = strings.ReplaceAll(message, username+":"+password+"@", username+":***@")
	message = strings.ReplaceAll(message, parsed.User.String()+"@", username+":***@")
	return message
}
