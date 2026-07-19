// Package keys handles generation and display of wotp API keys.
package keys

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

const (
	// AnonPrefix is the prefix for anonymous API keys.
	AnonPrefix = "wotp_anon_"
	// ServicePrefix is the prefix for service (admin) API keys.
	ServicePrefix = "wotp_service_"
	// RootPrefix is the prefix for the instance-admin root key, which
	// authorizes project management (see wotp-core's /v1/projects* routes).
	RootPrefix = "wotp_root_"
	// keyBytes is the number of random bytes used for key generation (24 hex chars).
	keyBytes = 12
)

// GenerateKey generates a cryptographically random hex key with the given prefix.
func GenerateKey(prefix string) (string, error) {
	b := make([]byte, keyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random key: %w", err)
	}
	return prefix + hex.EncodeToString(b), nil
}

// GenerateKeyPair generates both anon and service API keys.
func GenerateKeyPair() (anonKey, serviceKey string, err error) {
	anonKey, err = GenerateKey(AnonPrefix)
	if err != nil {
		return "", "", err
	}
	serviceKey, err = GenerateKey(ServicePrefix)
	if err != nil {
		return "", "", err
	}
	return anonKey, serviceKey, nil
}

// WriteEnvFile writes the .env file with the generated API keys. rootKey
// authorizes wotp-core's instance-admin endpoints (project create/list/rm,
// number pairing) — see wotp-core's keys.EnsureRootKey, which imports
// WOTP_ROOT_KEY from this file on first boot exactly like the anon/service
// keys below.
func WriteEnvFile(path, anonKey, serviceKey, rootKey string) error {
	content := fmt.Sprintf(`# Wotp API Keys — DO NOT COMMIT THIS FILE
# These keys authenticate requests to the Wotp API.
# anon key:    client-facing, rate-limited (send/verify only), scoped to the "default" project
# service key: project admin access (key regeneration, config, disconnect), scoped to the "default" project
# root key:    instance admin — create/list/delete projects, pair numbers (wotp project ...)

WOTP_ANON_KEY=%s
WOTP_SERVICE_KEY=%s
WOTP_ROOT_KEY=%s
`, anonKey, serviceKey, rootKey)

	return os.WriteFile(path, []byte(content), 0o600)
}

// ReadEnvFile reads the .env file and returns the anon, service, and root keys.
func ReadEnvFile(path string) (anonKey, serviceKey, rootKey string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", fmt.Errorf("reading .env file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "WOTP_ANON_KEY":
			anonKey = val
		case "WOTP_SERVICE_KEY":
			serviceKey = val
		case "WOTP_ROOT_KEY":
			rootKey = val
		}
	}

	if anonKey == "" || serviceKey == "" {
		return "", "", "", fmt.Errorf("missing API keys in %s", path)
	}
	// rootKey may be absent for .env files written before it existed —
	// callers that need it (the `wotp project` commands) check separately.
	return anonKey, serviceKey, rootKey, nil
}
