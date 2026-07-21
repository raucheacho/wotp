// Package templates loads OTP message templates from templates.toml
// and renders them with code and expiry placeholders.
package templates

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// defaultTemplatesFS embeds a generic en/fr/darija template set, used when
// no templates.toml is mounted at the configured path — same self-host
// philosophy as config.Load falling back to Defaults(): the binary works
// out of the box on a platform like Dokploy/Coolify that can't easily
// inject arbitrary files, no CLI-generated file required.
//
//go:embed default_templates.toml
var defaultTemplatesFS embed.FS

// LocaleTemplates maps locale codes to their template definitions.
type LocaleTemplates map[string]Template

// Template holds the message template for a single locale.
type Template struct {
	OTPMessage string `toml:"otp_message"`
}

// Store holds all loaded templates and the default locale.
type Store struct {
	templates     LocaleTemplates
	defaultLocale string
}

// NewStore creates a template store from a templates.toml file at path. If
// no file exists there, falls back to the built-in en/fr/darija templates
// rather than failing to start — path is still consulted first so an
// operator who does mount a custom templates.toml keeps full control.
func NewStore(path, defaultLocale string) (*Store, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = defaultTemplatesFS.ReadFile("default_templates.toml")
	}
	if err != nil {
		return nil, fmt.Errorf("templates: read %s: %w", path, err)
	}

	var raw map[string]map[string]string
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("templates: parse toml: %w", err)
	}

	templates := make(LocaleTemplates)
	for locale, fields := range raw {
		msg, ok := fields["otp_message"]
		if !ok {
			return nil, fmt.Errorf("templates: locale %q missing otp_message field", locale)
		}
		templates[locale] = Template{OTPMessage: msg}
	}

	if _, ok := templates[defaultLocale]; !ok && len(templates) > 0 {
		// Fall back to first available locale
		for k := range templates {
			defaultLocale = k
			break
		}
	}

	return &Store{
		templates:     templates,
		defaultLocale: defaultLocale,
	}, nil
}

// Render produces the final OTP message for the given locale.
// If the locale is not found, the default locale is used.
// Placeholders: {{code}} and {{expiry}} are replaced.
func (s *Store) Render(locale, code string, expiryMinutes int) (string, error) {
	tmpl, ok := s.templates[locale]
	if !ok {
		tmpl, ok = s.templates[s.defaultLocale]
		if !ok {
			return "", fmt.Errorf("templates: no template found for locale %q or default %q", locale, s.defaultLocale)
		}
	}

	msg := tmpl.OTPMessage
	msg = strings.ReplaceAll(msg, "{{code}}", code)
	msg = strings.ReplaceAll(msg, "{{expiry}}", fmt.Sprintf("%d", expiryMinutes))
	return msg, nil
}

// Locales returns the list of available locale codes.
func (s *Store) Locales() []string {
	var locales []string
	for k := range s.templates {
		locales = append(locales, k)
	}
	return locales
}

// DefaultLocale returns the configured default locale.
func (s *Store) DefaultLocale() string {
	return s.defaultLocale
}
