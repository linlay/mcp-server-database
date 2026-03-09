package database

import (
	"fmt"
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
		item.DSN = strings.TrimSpace(item.DSN)
		if item.Name == "" {
			return Catalog{}, fmt.Errorf("connection name is required")
		}
		if item.Driver == "" {
			return Catalog{}, fmt.Errorf("connection %s has unsupported driver", item.Name)
		}
		if item.DSN == "" {
			return Catalog{}, fmt.Errorf("connection %s dsn is required", item.Name)
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
