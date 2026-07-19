// Package config provides TOML-based configuration loading for wotp-core.
// Only instance-wide settings live here — everything that varies per
// project (OTP, messaging, WhatsApp inbound filters, webhooks, templates)
// moved to core/internal/project.Settings, stored per-project in the
// control database rather than a single shared config.toml.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

var version = "dev"

// Config is the root configuration loaded from config.toml.
type Config struct {
	Project ProjectConfig `toml:"project"`
	API     APIConfig     `toml:"api"`
	Storage StorageConfig `toml:"storage"`
}

// ProjectConfig holds instance identification (display metadata only —
// not to be confused with the multi-tenant "project" concept in
// core/internal/project, which this instance can host many of).
type ProjectConfig struct {
	Name string `toml:"name"`
	Ref  string `toml:"ref"`
}

// APIConfig holds HTTP server settings.
type APIConfig struct {
	Port            int  `toml:"port"`
	EnableDashboard bool `toml:"enable_dashboard"`
}

// StorageConfig holds database driver settings.
type StorageConfig struct {
	Driver      string `toml:"driver"`
	PostgresURL string `toml:"postgres_url"`
}

// Defaults returns a Config with sensible default values.
func Defaults() Config {
	return Config{
		Project: ProjectConfig{
			Name: "wotp",
			Ref:  "wotp-default",
		},
		API: APIConfig{
			Port:            54321,
			EnableDashboard: true,
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
	}
}

// Load reads and parses a TOML config file at the given path.
// Missing fields are filled with defaults.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse toml: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation: %w", err)
	}

	return &cfg, nil
}

// validate checks that required fields have valid values.
func (c *Config) validate() error {
	if c.API.Port <= 0 || c.API.Port > 65535 {
		return fmt.Errorf("api.port must be between 1 and 65535, got %d", c.API.Port)
	}
	if c.Storage.Driver != "sqlite" && c.Storage.Driver != "postgres" {
		return fmt.Errorf("storage.driver must be 'sqlite' or 'postgres', got %q", c.Storage.Driver)
	}
	return nil
}
