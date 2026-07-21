// Package keys handles API key generation, hashing, and validation.
// Two tiers: anon (wotp_anon_ prefix) for send/verify operations,
// and service (wotp_service_ prefix) for admin operations.
package keys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/wotp/core/internal/store"
)

const (
	// TierAnon is the anon key tier for send/verify only.
	TierAnon = "anon"
	// TierService is the service key tier for admin operations.
	TierService = "service"

	prefixAnon    = "wotp_anon_"
	prefixService = "wotp_service_"
)

// Manager handles API key lifecycle operations. wotp-core is mono-tenant —
// keys are instance-wide, not scoped to a project — and live in the shared
// ControlStore (see core/internal/store/control.go).
type Manager struct {
	control store.ControlStore
}

// NewManager creates a new API key manager.
func NewManager(cs store.ControlStore) *Manager {
	return &Manager{control: cs}
}

// GeneratedKey holds both the full plaintext key (shown once) and metadata.
type GeneratedKey struct {
	FullKey   string    `json:"key"`
	Prefix    string    `json:"prefix"`
	Tier      string    `json:"tier"`
	CreatedAt time.Time `json:"created_at"`
}

// Generate creates a new API key for the given tier.
// The full plaintext key is returned exactly once — only the hash is stored.
func (m *Manager) Generate(ctx context.Context, tier string) (*GeneratedKey, error) {
	var prefix string
	switch tier {
	case TierAnon:
		prefix = prefixAnon
	case TierService:
		prefix = prefixService
	default:
		return nil, fmt.Errorf("keys: unknown tier %q", tier)
	}

	// Generate 24 random bytes = 48 hex chars
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("keys: random bytes: %w", err)
	}
	secret := hex.EncodeToString(b)
	fullKey := prefix + secret

	// The "prefix" we store for lookup is the tier prefix + first 8 chars of the secret.
	keyPrefix := prefix + secret[:8]

	hash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("keys: hash key: %w", err)
	}

	now := time.Now().UTC()
	apiKey := &store.APIKey{
		ID:        uuid.New().String(),
		KeyHash:   string(hash),
		KeyPrefix: keyPrefix,
		Tier:      tier,
		CreatedAt: now,
	}

	if err := m.control.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("keys: store key: %w", err)
	}

	return &GeneratedKey{
		FullKey:   fullKey,
		Prefix:    keyPrefix,
		Tier:      tier,
		CreatedAt: now,
	}, nil
}

// Validate checks an API key against stored keys. Returns the key's tier if
// valid, or an error.
func (m *Manager) Validate(ctx context.Context, fullKey string) (tier string, err error) {
	var prefix string
	switch {
	case strings.HasPrefix(fullKey, prefixAnon):
		prefix = prefixAnon
	case strings.HasPrefix(fullKey, prefixService):
		prefix = prefixService
	default:
		return "", fmt.Errorf("keys: invalid key format")
	}

	// Extract the lookup prefix (tier prefix + first 8 chars of secret)
	secret := strings.TrimPrefix(fullKey, prefix)
	if len(secret) < 8 {
		return "", fmt.Errorf("keys: key too short")
	}
	keyPrefix := prefix + secret[:8]

	apiKey, err := m.control.GetAPIKeyByPrefix(ctx, keyPrefix)
	if err != nil {
		return "", fmt.Errorf("keys: lookup: %w", err)
	}
	if apiKey == nil {
		return "", fmt.Errorf("keys: unknown key")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(fullKey)); err != nil {
		return "", fmt.Errorf("keys: invalid key")
	}

	return apiKey.Tier, nil
}

// RegenerateAll deletes all keys of the given tier and generates a new one.
func (m *Manager) RegenerateAll(ctx context.Context, tier string) (*GeneratedKey, error) {
	if err := m.control.DeleteAPIKeysByTier(ctx, tier); err != nil {
		return nil, fmt.Errorf("keys: delete old keys: %w", err)
	}
	return m.Generate(ctx, tier)
}

// EnsureKeys checks if keys exist for both tiers. If not, generates fresh
// ones. Returns (anonKey, serviceKey, error). nil means a key already
// existed for that tier. Never consults WOTP_ANON_KEY/WOTP_SERVICE_KEY — see
// EnsureKeysWithEnvFallback for that.
func (m *Manager) EnsureKeys(ctx context.Context) (anon *GeneratedKey, service *GeneratedKey, err error) {
	keys, err := m.control.ListAPIKeys(ctx)
	if err != nil {
		return nil, nil, err
	}

	hasAnon, hasService := false, false
	for _, k := range keys {
		if k.Tier == TierAnon {
			hasAnon = true
		}
		if k.Tier == TierService {
			hasService = true
		}
	}

	if !hasAnon {
		anon, err = m.Generate(ctx, TierAnon)
		if err != nil {
			return nil, nil, err
		}
	}
	if !hasService {
		service, err = m.Generate(ctx, TierService)
		if err != nil {
			return nil, nil, err
		}
	}

	return anon, service, nil
}

// EnsureKeysWithEnvFallback behaves like EnsureKeys, but imports
// WOTP_ANON_KEY/WOTP_SERVICE_KEY from the environment when set instead of
// generating fresh keys — used once, by wotp-core's own startup bootstrap.
func (m *Manager) EnsureKeysWithEnvFallback(ctx context.Context) (anon *GeneratedKey, service *GeneratedKey, err error) {
	keys, err := m.control.ListAPIKeys(ctx)
	if err != nil {
		return nil, nil, err
	}

	hasAnon, hasService := false, false
	for _, k := range keys {
		if k.Tier == TierAnon {
			hasAnon = true
		}
		if k.Tier == TierService {
			hasService = true
		}
	}

	if !hasAnon {
		if envAnon := os.Getenv("WOTP_ANON_KEY"); envAnon != "" {
			anon, err = m.importKey(ctx, envAnon, TierAnon)
			if err != nil {
				return nil, nil, fmt.Errorf("import WOTP_ANON_KEY: %w", err)
			}
		} else {
			anon, err = m.Generate(ctx, TierAnon)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if !hasService {
		if envService := os.Getenv("WOTP_SERVICE_KEY"); envService != "" {
			service, err = m.importKey(ctx, envService, TierService)
			if err != nil {
				return nil, nil, fmt.Errorf("import WOTP_SERVICE_KEY: %w", err)
			}
		} else {
			service, err = m.Generate(ctx, TierService)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return anon, service, nil
}

// importKey imports an existing full key (like from an env var) into the database.
func (m *Manager) importKey(ctx context.Context, fullKey, tier string) (*GeneratedKey, error) {
	var prefix string
	switch tier {
	case TierAnon:
		prefix = prefixAnon
	case TierService:
		prefix = prefixService
	default:
		return nil, fmt.Errorf("keys: unknown tier %q", tier)
	}

	if !strings.HasPrefix(fullKey, prefix) {
		return nil, fmt.Errorf("keys: key %q does not match tier prefix %q", fullKey, prefix)
	}

	secret := strings.TrimPrefix(fullKey, prefix)
	if len(secret) < 8 {
		return nil, fmt.Errorf("keys: key too short")
	}
	keyPrefix := prefix + secret[:8]

	hash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("keys: hash key: %w", err)
	}

	now := time.Now().UTC()
	apiKey := &store.APIKey{
		ID:        uuid.New().String(),
		KeyHash:   string(hash),
		KeyPrefix: keyPrefix,
		Tier:      tier,
		CreatedAt: now,
	}

	if err := m.control.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("keys: store key: %w", err)
	}

	return &GeneratedKey{
		FullKey:   fullKey,
		Prefix:    keyPrefix,
		Tier:      tier,
		CreatedAt: now,
	}, nil
}
