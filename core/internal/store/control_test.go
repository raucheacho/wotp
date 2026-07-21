package store

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestControlStore(t *testing.T) *SQLiteControlStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "control.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteControlStore(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestControlStore_APIKeyLifecycle(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	key := &APIKey{ID: uuid.New().String(), KeyHash: "hash1", KeyPrefix: "wotp_anon_aaaaaaaa", Tier: "anon", CreatedAt: time.Now()}
	if err := s.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	got, err := s.GetAPIKeyByPrefix(ctx, "wotp_anon_aaaaaaaa")
	if err != nil {
		t.Fatalf("GetAPIKeyByPrefix: %v", err)
	}
	if got == nil || got.ID != key.ID {
		t.Fatalf("GetAPIKeyByPrefix = %+v, want id %s", got, key.ID)
	}

	list, err := s.ListAPIKeys(ctx)
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(list) != 1 || list[0].ID != key.ID {
		t.Fatalf("ListAPIKeys = %+v, want only key", list)
	}

	if err := s.DeleteAPIKeysByTier(ctx, "anon"); err != nil {
		t.Fatalf("DeleteAPIKeysByTier: %v", err)
	}
	afterDelete, err := s.ListAPIKeys(ctx)
	if err != nil {
		t.Fatalf("ListAPIKeys after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("expected no keys left, got %d", len(afterDelete))
	}
}

func TestControlStore_APIKeysScopedByTier(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	anonKey := &APIKey{ID: uuid.New().String(), KeyHash: "hashA", KeyPrefix: "wotp_anon_aaaaaaaa", Tier: "anon", CreatedAt: time.Now()}
	serviceKey := &APIKey{ID: uuid.New().String(), KeyHash: "hashB", KeyPrefix: "wotp_service_bbbbbbbb", Tier: "service", CreatedAt: time.Now()}
	if err := s.CreateAPIKey(ctx, anonKey); err != nil {
		t.Fatalf("CreateAPIKey anon: %v", err)
	}
	if err := s.CreateAPIKey(ctx, serviceKey); err != nil {
		t.Fatalf("CreateAPIKey service: %v", err)
	}

	if err := s.DeleteAPIKeysByTier(ctx, "anon"); err != nil {
		t.Fatalf("DeleteAPIKeysByTier: %v", err)
	}

	// The service key must be untouched by deleting the anon tier.
	list, err := s.ListAPIKeys(ctx)
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(list) != 1 || list[0].ID != serviceKey.ID {
		t.Fatalf("ListAPIKeys after deleting anon tier = %+v, want only the service key", list)
	}
}

// TestControlStore_NumbersUpsertAndStatus covers the numbers table now that
// this instance has at most one number (see whatsapp.Pool) — no more
// project scoping, just upsert-in-place and status updates.
func TestControlStore_NumbersUpsertAndStatus(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	n0 := &Number{JID: "000@s.whatsapp.net", Phone: "+0000", Status: NumberStatusPending, CreatedAt: time.Now()}
	if err := s.UpsertNumber(ctx, n0); err != nil {
		t.Fatalf("UpsertNumber: %v", err)
	}

	numbers, err := s.ListNumbers(ctx)
	if err != nil {
		t.Fatalf("ListNumbers: %v", err)
	}
	if len(numbers) != 1 || numbers[0].JID != n0.JID {
		t.Fatalf("ListNumbers = %+v, want [n0]", numbers)
	}

	if err := s.UpdateNumberStatus(ctx, n0.JID, NumberStatusConnected); err != nil {
		t.Fatalf("UpdateNumberStatus: %v", err)
	}
	updated, err := s.ListNumbers(ctx)
	if err != nil {
		t.Fatalf("ListNumbers after status update: %v", err)
	}
	if updated[0].Status != NumberStatusConnected {
		t.Fatalf("n0 status = %q, want connected", updated[0].Status)
	}

	// Upsert on an existing JID updates in place rather than duplicating.
	n0Updated := &Number{JID: n0.JID, Phone: "+0000", Status: NumberStatusDisconnected, CreatedAt: time.Now()}
	if err := s.UpsertNumber(ctx, n0Updated); err != nil {
		t.Fatalf("UpsertNumber (update): %v", err)
	}
	final, err := s.ListNumbers(ctx)
	if err != nil {
		t.Fatalf("ListNumbers after upsert: %v", err)
	}
	if len(final) != 1 {
		t.Fatalf("expected upsert to update in place, got %d rows", len(final))
	}
}

func TestControlStore_SettingsRoundTrip(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	empty, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings (unset): %v", err)
	}
	if empty != "" {
		t.Fatalf("GetSettings before any save = %q, want \"\"", empty)
	}

	if err := s.UpdateSettings(ctx, `{"otp":{"code_length":8}}`); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	got, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got != `{"otp":{"code_length":8}}` {
		t.Fatalf("GetSettings = %q, want the saved JSON", got)
	}

	// A second update must overwrite in place (singleton row), not error or duplicate.
	if err := s.UpdateSettings(ctx, `{"otp":{"code_length":4}}`); err != nil {
		t.Fatalf("UpdateSettings (second): %v", err)
	}
	got, err = s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings (second): %v", err)
	}
	if got != `{"otp":{"code_length":4}}` {
		t.Fatalf("GetSettings after second update = %q, want the newer JSON", got)
	}
}
