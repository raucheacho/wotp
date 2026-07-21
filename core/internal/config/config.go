// Package config provides TOML-based configuration loading for wotp-core.
// Only instance-wide settings live here — everything that varies per
// project (OTP, messaging, WhatsApp inbound filters, webhooks, templates)
// moved to core/internal/project.Settings, stored per-project in the
// control database rather than a single shared config.toml.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration loaded from config.toml.
type Config struct {
	Project ProjectConfig `toml:"project"`
	API     APIConfig     `toml:"api"`
	Storage StorageConfig `toml:"storage"`
}

// ProjectConfig holds this instance's display metadata (name, ref) — used
// for the whatsmeow device name and dashboard title.
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

// Load builds a Config starting from Defaults(), applies config.toml at
// path on top if the file exists, then applies WOTP_* environment variable
// overrides on top of that. A missing config.toml is not an error — this
// is what lets wotp-core boot on a platform (Dokploy, Coolify, plain
// `docker run`) that can't easily inject an arbitrary file, mirroring how
// Supabase's self-hosted docker-compose is env-var driven rather than
// requiring a bespoke CLI to generate config files first. An operator who
// does mount a config.toml keeps full control — it's read first, env vars
// only override on top.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("config: parse toml: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		// No config.toml mounted — Defaults() plus any env overrides below.
	default:
		return nil, fmt.Errorf("config: read file %s: %w", path, err)
	}

	if err := cfg.applyEnvOverrides(); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides layers WOTP_* environment variables on top of whatever
// Defaults()/config.toml already set — the same role Supabase's .env plays
// for its self-hosted docker-compose, just scoped to wotp's much smaller
// instance-wide config surface. Storage isn't included: only "sqlite" is
// actually implemented (see main.go), so there's nothing meaningful to
// override yet.
func (c *Config) applyEnvOverrides() error {
	if v := os.Getenv("WOTP_PROJECT_NAME"); v != "" {
		c.Project.Name = v
	}
	if v := os.Getenv("WOTP_PROJECT_REF"); v != "" {
		c.Project.Ref = v
	}
	if v := os.Getenv("WOTP_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("config: WOTP_PORT must be an integer, got %q: %w", v, err)
		}
		c.API.Port = port
	}
	if v := os.Getenv("WOTP_ENABLE_DASHBOARD"); v != "" {
		enabled, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("config: WOTP_ENABLE_DASHBOARD must be a boolean, got %q: %w", v, err)
		}
		c.API.EnableDashboard = enabled
	}
	return nil
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
