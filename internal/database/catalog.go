package database

import (
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadCatalog(path string) (Catalog, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Catalog{}, fmt.Errorf("connections config path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		return Catalog{}, err
	}

	if info.IsDir() {
		return loadCatalogDir(path)
	}

	return loadCatalogFile(path)
}

func loadCatalogDir(path string) (Catalog, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return Catalog{}, err
	}

	catalog := Catalog{Version: 1}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		if isExampleConfigFile(entry.Name()) {
			continue
		}
		fileCatalog, err := loadCatalogFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return Catalog{}, err
		}
		catalog.Connections = append(catalog.Connections, fileCatalog.Connections...)
	}

	if len(catalog.Connections) == 0 {
		return Catalog{}, fmt.Errorf("connections config has no yaml files: %s", path)
	}

	return validateCatalog(catalog)
}

func loadCatalogFile(path string) (Catalog, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}

	raw := map[string]any{}
	if err := yaml.Unmarshal(bytes, &raw); err != nil {
		return Catalog{}, fmt.Errorf("parse connections config %s: %w", path, err)
	}

	if _, ok := raw["connections"]; ok {
		catalog := Catalog{}
		if err := yaml.Unmarshal(bytes, &catalog); err != nil {
			return Catalog{}, fmt.Errorf("parse connections config %s: %w", path, err)
		}
		return validateCatalog(catalog)
	}

	connection := ConnectionConfig{}
	if err := yaml.Unmarshal(bytes, &connection); err != nil {
		return Catalog{}, fmt.Errorf("parse connection config %s: %w", path, err)
	}
	return validateCatalog(Catalog{
		Version:     1,
		Connections: []ConnectionConfig{connection},
	})
}

func validateCatalog(catalog Catalog) (Catalog, error) {
	if catalog.Version == 0 {
		catalog.Version = 1
	}
	if len(catalog.Connections) == 0 {
		return Catalog{}, fmt.Errorf("connections config has no connections")
	}

	seen := map[string]struct{}{}
	for i := range catalog.Connections {
		item := &catalog.Connections[i]
		item.Name = strings.TrimSpace(item.Name)
		item.Description = strings.TrimSpace(item.Description)
		item.Driver = normalizeDriver(item.Driver)
		item.URL = strings.TrimSpace(item.URL)
		item.Username = strings.TrimSpace(item.Username)
		item.Password = strings.TrimSpace(item.Password)
		item.DSN = strings.TrimSpace(item.DSN)
		if item.Name == "" {
			return Catalog{}, fmt.Errorf("connection name is required")
		}
		driver, err := resolveConnectionDriver(*item)
		if err != nil {
			return Catalog{}, fmt.Errorf("connection %s: %w", item.Name, err)
		}
		item.Driver = driver
		hadURL := item.URL != ""
		hadDSN := item.DSN != ""
		dsn, err := normalizeConnectionDSN(*item)
		if err != nil {
			return Catalog{}, fmt.Errorf("connection %s: %w", item.Name, err)
		}
		item.DSN = dsn
		if hadURL && !hadDSN {
			// The directory loader validates individual files before aggregating them.
			// Clear the external URL after deriving the internal DSN so a second validation
			// pass does not treat the normalized DSN as a user-supplied conflicting field.
			item.URL = ""
		}
		if _, exists := seen[strings.ToLower(item.Name)]; exists {
			return Catalog{}, fmt.Errorf("duplicate connection name: %s", item.Name)
		}
		seen[strings.ToLower(item.Name)] = struct{}{}
	}

	sort.Slice(catalog.Connections, func(i, j int) bool {
		return strings.ToLower(catalog.Connections[i].Name) < strings.ToLower(catalog.Connections[j].Name)
	})

	return catalog, nil
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgresql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return ""
	}
}

func isExampleConfigFile(name string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lowerName, ".example.yml") || strings.HasSuffix(lowerName, ".example.yaml")
}

func normalizeConnectionDSN(cfg ConnectionConfig) (string, error) {
	hasDSN := cfg.DSN != ""
	hasURL := cfg.URL != ""
	if hasDSN && hasURL {
		return "", fmt.Errorf("dsn and url are mutually exclusive")
	}
	if hasDSN {
		return cfg.DSN, nil
	}
	if !hasURL {
		return "", fmt.Errorf("dsn or url is required")
	}

	switch cfg.Driver {
	case "mysql":
		return normalizeMySQLURL(cfg.URL, cfg.Username, cfg.Password)
	case "postgresql":
		return normalizePostgresURL(cfg.URL, cfg.Username, cfg.Password)
	case "sqlite":
		return normalizeSQLiteURL(cfg.URL, cfg.Username, cfg.Password)
	default:
		return "", fmt.Errorf("has unsupported driver")
	}
}

func resolveConnectionDriver(cfg ConnectionConfig) (string, error) {
	if cfg.URL != "" {
		inferredDriver, err := inferDriverFromURL(cfg.URL)
		if err != nil {
			return "", err
		}
		if cfg.Driver != "" && cfg.Driver != inferredDriver {
			return "", fmt.Errorf("driver %q does not match url-inferred driver %q", cfg.Driver, inferredDriver)
		}
		return inferredDriver, nil
	}

	if cfg.DSN == "" {
		return "", fmt.Errorf("dsn or url is required")
	}
	if cfg.Driver != "" {
		return cfg.Driver, nil
	}
	return "", fmt.Errorf("driver is required when dsn is used")
}

func inferDriverFromURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", fmt.Errorf("url is required")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "jdbc:") {
		return "", fmt.Errorf("url does not support jdbc prefix")
	}

	parsed, err := neturl.Parse(trimmed)
	if err == nil {
		switch strings.ToLower(parsed.Scheme) {
		case "mysql":
			return "mysql", nil
		case "postgres", "postgresql":
			return "postgresql", nil
		case "file":
			return "sqlite", nil
		case "":
			// Fall through to local path detection.
		default:
			return "", fmt.Errorf("url has unsupported scheme %q", parsed.Scheme)
		}
	}

	if looksLikeSQLitePath(trimmed) {
		return "sqlite", nil
	}
	return "", fmt.Errorf("url must start with mysql://, postgres://, postgresql://, file:, or be a local sqlite path")
}

func looksLikeSQLitePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "://") {
		return false
	}
	return strings.HasPrefix(trimmed, "/") ||
		strings.HasPrefix(trimmed, "./") ||
		strings.HasPrefix(trimmed, "../") ||
		strings.HasPrefix(trimmed, "~/") ||
		(!strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "@"))
}

func normalizeMySQLURL(rawURL string, username string, password string) (string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "jdbc:") {
		return "", fmt.Errorf("url does not support jdbc prefix")
	}
	if err := validateCredentials(username, password); err != nil {
		return "", err
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("has invalid mysql url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "mysql") {
		return "", fmt.Errorf("url must start with mysql://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("mysql url host is required")
	}
	if parsed.User != nil && parsed.User.String() != "" {
		return "", fmt.Errorf("mysql url must not include username or password")
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("mysql url must not include a fragment")
	}

	var builder strings.Builder
	if username != "" {
		builder.WriteString(username)
		if password != "" {
			builder.WriteString(":")
			builder.WriteString(password)
		}
		builder.WriteString("@")
	}
	builder.WriteString("tcp(")
	builder.WriteString(parsed.Host)
	builder.WriteString(")/")
	builder.WriteString(strings.TrimPrefix(parsed.Path, "/"))
	if parsed.RawQuery != "" {
		builder.WriteString("?")
		builder.WriteString(parsed.RawQuery)
	}
	return builder.String(), nil
}

func normalizePostgresURL(rawURL string, username string, password string) (string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "jdbc:") {
		return "", fmt.Errorf("url does not support jdbc prefix")
	}
	if err := validateCredentials(username, password); err != nil {
		return "", err
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("has invalid postgres url: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "postgres" && scheme != "postgresql" {
		return "", fmt.Errorf("url must start with postgres:// or postgresql://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("postgres url host is required")
	}
	if parsed.User != nil && parsed.User.String() != "" {
		return "", fmt.Errorf("postgres url must not include username or password")
	}
	if username != "" {
		if password != "" {
			parsed.User = neturl.UserPassword(username, password)
		} else {
			parsed.User = neturl.User(username)
		}
	}
	return parsed.String(), nil
}

func normalizeSQLiteURL(rawURL string, username string, password string) (string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "jdbc:") {
		return "", fmt.Errorf("url does not support jdbc prefix")
	}
	if username != "" || password != "" {
		return "", fmt.Errorf("sqlite url does not support username or password")
	}

	parsed, err := neturl.Parse(rawURL)
	if err == nil && parsed.Scheme != "" && parsed.Scheme != "file" {
		return "", fmt.Errorf("sqlite url must be file:... or a local path")
	}
	return rawURL, nil
}

func validateCredentials(username string, password string) error {
	if username == "" && password != "" {
		return fmt.Errorf("password requires username")
	}
	return nil
}
