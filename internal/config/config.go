package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ProjectRoots []string `json:"projectRoots,omitempty"`
	ExcludeGlobs []string `json:"excludeGlobs,omitempty"`
}

func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
