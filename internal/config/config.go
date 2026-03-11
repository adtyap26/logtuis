package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SSHSource defines a remote server to scan for logs.
type SSHSource struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Identity string `json:"identity"` // path to private key; empty = try default keys
	Password string `json:"password"` // plain password; used if identity is empty
	Path     string `json:"path"`
}

// Config holds application configuration.
type Config struct {
	SSHSources []SSHSource `json:"ssh_sources"`
}

// Load reads config from ~/.config/logtuis/config.json.
// Returns empty Config (no error) if the file does not exist.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("home dir: %w", err)
	}
	path := filepath.Join(home, ".config", "logtuis", "config.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
