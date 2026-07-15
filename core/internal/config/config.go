// Package config provides TOML-based configuration loading for wotp-core.
// The configuration structure matches the spec §4.1 exactly.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

var version = "dev"

// Config is the root configuration loaded from config.toml.
type Config struct {
	Project   ProjectConfig   `toml:"project"`
	API       APIConfig       `toml:"api"`
	OTP       OTPConfig       `toml:"otp"`
	WhatsApp  WhatsAppConfig  `toml:"whatsapp"`
	Storage   StorageConfig   `toml:"storage"`
	Templates TemplatesConfig `toml:"templates"`
	Messaging MessagingConfig `toml:"messaging"`
	Webhooks  WebhooksConfig  `toml:"webhooks"`
}

// ProjectConfig holds project identification.
type ProjectConfig struct {
	Name string `toml:"name"`
	Ref  string `toml:"ref"`
}

// APIConfig holds HTTP server settings.
type APIConfig struct {
	Port            int  `toml:"port"`
	EnableDashboard bool `toml:"enable_dashboard"`
}

// OTPConfig holds OTP generation and verification parameters.
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
	Driver      string `toml:"driver"`
	PostgresURL string `toml:"postgres_url"`
}

// TemplatesConfig holds message template settings.
type TemplatesConfig struct {
	DefaultLocale string `toml:"default_locale"`
}

// MessagingConfig holds general messaging settings.
type MessagingConfig struct {
	MaxMessagesPerMinute int  `toml:"max_messages_per_minute"`
	SimulateTyping       bool `toml:"simulate_typing"`
}

// WebhooksConfig holds webhook settings.
type WebhooksConfig struct {
	Endpoint string   `toml:"endpoint"`
	Events   []string `toml:"events"`
	Secret   string   `toml:"secret"`
}

// Defaults returns a Config with sensible default values matching spec §4.1.
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
		OTP: OTPConfig{
			CodeLength:              6,
			ExpiryMinutes:           5,
			MaxAttempts:             5,
			RateLimitPerPhonePerHr:  3,
		},
		WhatsApp: WhatsAppConfig{
			DeviceName:              "Wotp",
			ReconnectBackoffSeconds: []int{5, 15, 60, 300},
		},
		Storage: StorageConfig{
			Driver: "sqlite",
		},
		Templates: TemplatesConfig{
			DefaultLocale: "fr",
		},
		Messaging: MessagingConfig{
			MaxMessagesPerMinute: 60,
			SimulateTyping:       false,
		},
		Webhooks: WebhooksConfig{
			Endpoint: "",
			Events:   []string{},
			Secret:   "",
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
	if c.OTP.CodeLength < 4 || c.OTP.CodeLength > 10 {
		return fmt.Errorf("otp.code_length must be between 4 and 10, got %d", c.OTP.CodeLength)
	}
	if c.OTP.ExpiryMinutes <= 0 {
		return fmt.Errorf("otp.expiry_minutes must be positive, got %d", c.OTP.ExpiryMinutes)
	}
	if c.OTP.MaxAttempts <= 0 {
		return fmt.Errorf("otp.max_attempts must be positive, got %d", c.OTP.MaxAttempts)
	}
	if c.OTP.RateLimitPerPhonePerHr <= 0 {
		return fmt.Errorf("otp.rate_limit_per_phone_per_hour must be positive, got %d", c.OTP.RateLimitPerPhonePerHr)
	}
	if c.Storage.Driver != "sqlite" && c.Storage.Driver != "postgres" {
		return fmt.Errorf("storage.driver must be 'sqlite' or 'postgres', got %q", c.Storage.Driver)
	}
	return nil
}
