package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

const currentConfigVersion = 2

var configIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

type legacyConfigV1 struct {
	Version             int             `json:"version"`
	MutationToken       string          `json:"mutation_token"`
	ScanIntervalSeconds int             `json:"scan_interval_seconds"`
	Workbenches         []legacyProject `json:"workbenches"`
}

type legacyProject struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Repos []string `json:"repos"`
}

func loadConfig(home string) (Config, error) {
	path := filepath.Join(home, "config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		config := defaultConfig()
		return config, saveConfig(home, config)
	}
	if err != nil {
		return Config{}, err
	}

	var version struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	switch version.Version {
	case 1:
		config, err := migrateConfigV1(data)
		if err != nil {
			return Config{}, err
		}
		if err := saveConfig(home, config); err != nil {
			return Config{}, fmt.Errorf("save migrated config: %w", err)
		}
		return config, nil
	case currentConfigVersion:
		var config Config
		if err := json.Unmarshal(data, &config); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
		if err := validateConfig(config); err != nil {
			return Config{}, err
		}
		return config, nil
	default:
		return Config{}, fmt.Errorf("unsupported config version %d", version.Version)
	}
}

func defaultConfig() Config {
	return Config{
		Version:             currentConfigVersion,
		ScanIntervalSeconds: 300,
		Projects:            []domain.Project{},
	}
}

func validateConfig(config Config) error {
	if config.Version != currentConfigVersion {
		return fmt.Errorf("unsupported config version %d", config.Version)
	}
	if config.ScanIntervalSeconds < 10 {
		return errors.New("scan interval must be at least 10 seconds")
	}
	seen := make(map[string]struct{}, len(config.Projects))
	for _, project := range config.Projects {
		if err := project.Validate(); err != nil {
			return fmt.Errorf("project %q: %w", project.Metadata.ID, err)
		}
		if _, ok := seen[project.Metadata.ID]; ok {
			return fmt.Errorf("duplicate project id %q", project.Metadata.ID)
		}
		seen[project.Metadata.ID] = struct{}{}
	}
	return nil
}

func saveConfig(home string, config Config) error {
	if err := validateConfig(config); err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(home, "config.json.tmp")
	target := filepath.Join(home, "config.json")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func migrateConfigV1(data []byte) (Config, error) {
	var legacy legacyConfigV1
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Config{}, fmt.Errorf("decode version 1 config: %w", err)
	}
	config := defaultConfig()
	if legacy.ScanIntervalSeconds >= 10 {
		config.ScanIntervalSeconds = legacy.ScanIntervalSeconds
	}
	for _, item := range legacy.Workbenches {
		id := normalizeID(item.ID)
		name := strings.TrimSpace(item.Name)
		if id == "" {
			id = normalizeID(name)
		}
		if id == "" || name == "" {
			return Config{}, errors.New("version 1 project requires an id and name")
		}
		repositories := make([]domain.Repository, 0, len(item.Repos))
		for index, path := range item.Repos {
			path = strings.TrimSpace(path)
			if path == "" {
				return Config{}, fmt.Errorf("version 1 project %q contains an empty repository path", id)
			}
			repositoryID := fmt.Sprintf("repo-%d", index+1)
			repositories = append(repositories, domain.NewRepository(repositoryID, filepath.Base(path), path))
		}
		config.Projects = append(config.Projects, domain.NewProject(id, name, repositories))
	}
	if err := validateConfig(config); err != nil {
		return Config{}, fmt.Errorf("migrated config: %w", err)
	}
	// legacy.MutationToken is intentionally not copied. It was an ephemeral
	// local UI token and must not become a persisted secret in the new schema.
	return config, nil
}

func normalizeID(value string) string {
	return strings.Trim(configIDPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
}

func randomToken() string {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buffer)
}
