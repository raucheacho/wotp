// Package config handles loading and writing wotp config.toml files.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the full wotp config.toml structure (spec §4.1).
type Config struct {
	Project   ProjectConfig   `toml:"project"`
	API       APIConfig       `toml:"api"`
	OTP       OTPConfig       `toml:"otp"`
	WhatsApp  WhatsAppConfig  `toml:"whatsapp"`
	Storage   StorageConfig   `toml:"storage"`
	Templates TemplatesConfig `toml:"templates"`
}

// ProjectConfig holds project identification.
type ProjectConfig struct {
	Name string `toml:"name"`
	Ref  string `toml:"ref"`
}

// APIConfig holds API server settings.
type APIConfig struct {
	Port            int  `toml:"port"`
	EnableDashboard bool `toml:"enable_dashboard"`
}

// OTPConfig holds OTP generation/verification settings.
type OTPConfig struct {
	CodeLength              int `toml:"code_length"`
	ExpiryMinutes           int `toml:"expiry_minutes"`
	MaxAttempts             int `toml:"max_attempts"`
	RateLimitPerPhonePerHr  int `toml:"rate_limit_per_phone_per_hour"`
}

// WhatsAppConfig holds WhatsApp connection settings.
type WhatsAppConfig struct {
	DeviceName              string `toml:"device_name"`
	ReconnectBackoffSeconds []int  `toml:"reconnect_backoff_seconds"`
}

// StorageConfig holds database driver settings.
type StorageConfig struct {
	Driver string `toml:"driver"`
}

// TemplatesConfig holds template locale settings.
type TemplatesConfig struct {
	DefaultLocale string `toml:"default_locale"`
}

// DefaultConfig returns a Config with spec-compliant defaults for the given project name and ref.
func DefaultConfig(projectName, projectRef string) Config {
	return Config{
		Project: ProjectConfig{
			Name: projectName,
			Ref:  projectRef,
		},
		API: APIConfig{
			Port:            54321,
			EnableDashboard: true,
		},
		OTP: OTPConfig{
			CodeLength:             6,
			ExpiryMinutes:          5,
			MaxAttempts:            5,
			RateLimitPerPhonePerHr: 3,
		},
		WhatsApp: WhatsAppConfig{
			DeviceName:              fmt.Sprintf("Wotp - %s", projectName),
			ReconnectBackoffSeconds: []int{5, 15, 60, 300},
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
		Templates: TemplatesConfig{
			DefaultLocale: "fr",
		},
	}
}

// Write serializes the config to a TOML file at the given path.
func Write(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return nil
}

// Load reads and parses a config.toml file from the given path.
func Load(path string) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}
	return cfg, nil
}

// FindProjectDir walks up from cwd to find the directory containing wotp/.
// Returns the project root (parent of wotp/).
func FindProjectDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	dir := cwd
	for {
		wotpDir := filepath.Join(dir, "wotp")
		if info, err := os.Stat(wotpDir); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find wotp/ directory (run this command from within a wotp project, or run 'wotp init' first)")
}

// WotpDir returns the path to the wotp/ directory inside the project.
func WotpDir(projectDir string) string {
	return filepath.Join(projectDir, "wotp")
}

// RuntimeDir returns the path to the .wotp/ runtime directory.
func RuntimeDir(projectDir string) string {
	return filepath.Join(projectDir, "wotp", ".wotp")
}

// ConfigPath returns the path to config.toml.
func ConfigPath(projectDir string) string {
	return filepath.Join(projectDir, "wotp", "config.toml")
}

// EnvPath returns the path to the hidden .env file used by docker-compose internally.
func EnvPath(projectDir string) string {
	return filepath.Join(projectDir, "wotp", ".wotp", ".env")
}

// ComposePath returns the path to the generated docker-compose.yml.
func ComposePath(projectDir string) string {
	return filepath.Join(projectDir, "wotp", ".wotp", "docker-compose.yml")
}

// DataDir returns the path to the .wotp/data/ directory.
func DataDir(projectDir string) string {
	return filepath.Join(projectDir, "wotp", ".wotp", "data")
}

// SessionDir returns the path to the WhatsApp session data directory.
func SessionDir(projectDir string) string {
	return filepath.Join(projectDir, "wotp", ".wotp", "data", "session")
}
