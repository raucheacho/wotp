package keys

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/wotp/core/internal/store"
)

func newTestControlStore(t *testing.T) store.ControlStore {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.NewSQLiteControlStore(filepath.Join(t.TempDir(), "control.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestProject(t *testing.T, cs store.ControlStore, slug string) string {
	t.Helper()
	p := &store.Project{ID: uuid.New().String(), Slug: slug, Name: slug, CreatedAt: time.Now()}
	if err := cs.CreateProject(context.Background(), p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p.ID
}

func TestGenerateAndValidate_RoundTrip(t *testing.T) {
	cs := newTestControlStore(t)
	projectID := newTestProject(t, cs, "acme")
	mgr := NewManager(cs)
	ctx := context.Background()

	key, err := mgr.Generate(ctx, projectID, TierAnon)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	gotProjectID, tier, err := mgr.Validate(ctx, key.FullKey)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if gotProjectID != projectID {
		t.Errorf("projectID = %q, want %q", gotProjectID, projectID)
	}
	if tier != TierAnon {
		t.Errorf("tier = %q, want %q", tier, TierAnon)
	}
}

func TestValidate_UnknownKeyErrors(t *testing.T) {
	cs := newTestControlStore(t)
	mgr := NewManager(cs)
	if _, _, err := mgr.Validate(context.Background(), "wotp_anon_deadbeefdeadbeef"); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}

// TestEnsureKeys_DoesNotCollideAcrossProjects is a regression test for a
// real bug found via live testing: EnsureKeys used to always consult
// WOTP_ANON_KEY/WOTP_SERVICE_KEY, which are process-wide env vars set once
// for the instance's "default" project. Every subsequent project created
// via `wotp project create` (POST /v1/projects -> EnsureKeys) tried to
// import that same env-provided key again, hitting a UNIQUE constraint on
// api_keys.key_prefix. EnsureKeys must never look at the environment —
// only EnsureKeysWithEnvFallback (used solely for the default project's
// startup bootstrap) may do that.
func TestEnsureKeys_DoesNotCollideAcrossProjects(t *testing.T) {
	t.Setenv("WOTP_ANON_KEY", "wotp_anon_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("WOTP_SERVICE_KEY", "wotp_service_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	cs := newTestControlStore(t)
	mgr := NewManager(cs)
	ctx := context.Background()

	defaultProjectID := newTestProject(t, cs, "default")
	anon1, service1, err := mgr.EnsureKeysWithEnvFallback(ctx, defaultProjectID)
	if err != nil {
		t.Fatalf("EnsureKeysWithEnvFallback (default project): %v", err)
	}
	if anon1.FullKey != "wotp_anon_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("expected the default project to import WOTP_ANON_KEY, got %q", anon1.FullKey)
	}
	if service1.FullKey != "wotp_service_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("expected the default project to import WOTP_SERVICE_KEY, got %q", service1.FullKey)
	}

	// A second, dynamically created project must NOT try to reimport the
	// same env-provided keys (that's exactly the collision this test guards
	// against) — EnsureKeys must generate fresh ones instead.
	acmeProjectID := newTestProject(t, cs, "acme")
	anon2, service2, err := mgr.EnsureKeys(ctx, acmeProjectID)
	if err != nil {
		t.Fatalf("EnsureKeys (second project): %v", err)
	}
	if anon2.FullKey == anon1.FullKey {
		t.Fatal("second project's anon key must not equal the first project's env-imported key")
	}
	if service2.FullKey == service1.FullKey {
		t.Fatal("second project's service key must not equal the first project's env-imported key")
	}

	// Both projects' keys must resolve back to their own project.
	if pid, _, err := mgr.Validate(ctx, anon1.FullKey); err != nil || pid != defaultProjectID {
		t.Fatalf("validate default anon key: pid=%q err=%v", pid, err)
	}
	if pid, _, err := mgr.Validate(ctx, anon2.FullKey); err != nil || pid != acmeProjectID {
		t.Fatalf("validate acme anon key: pid=%q err=%v", pid, err)
	}
}

func TestEnsureKeys_IsIdempotent(t *testing.T) {
	cs := newTestControlStore(t)
	projectID := newTestProject(t, cs, "acme")
	mgr := NewManager(cs)
	ctx := context.Background()

	anon1, service1, err := mgr.EnsureKeys(ctx, projectID)
	if err != nil {
		t.Fatalf("EnsureKeys (first call): %v", err)
	}
	if anon1 == nil || service1 == nil {
		t.Fatalf("expected both keys to be generated on first call, got anon=%v service=%v", anon1, service1)
	}

	anon2, service2, err := mgr.EnsureKeys(ctx, projectID)
	if err != nil {
		t.Fatalf("EnsureKeys (second call): %v", err)
	}
	if anon2 != nil || service2 != nil {
		t.Fatalf("expected nil (already exists) on second call, got anon=%v service=%v", anon2, service2)
	}
}

func TestRegenerateAll_InvalidatesOldKey(t *testing.T) {
	cs := newTestControlStore(t)
	projectID := newTestProject(t, cs, "acme")
	mgr := NewManager(cs)
	ctx := context.Background()

	oldKey, err := mgr.Generate(ctx, projectID, TierAnon)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	newKey, err := mgr.RegenerateAll(ctx, projectID, TierAnon)
	if err != nil {
		t.Fatalf("RegenerateAll: %v", err)
	}
	if newKey.FullKey == oldKey.FullKey {
		t.Fatal("expected a fresh key after regeneration")
	}

	if _, _, err := mgr.Validate(ctx, oldKey.FullKey); err == nil {
		t.Fatal("old key should no longer validate after regeneration")
	}
	if _, tier, err := mgr.Validate(ctx, newKey.FullKey); err != nil || tier != TierAnon {
		t.Fatalf("new key should validate: tier=%q err=%v", tier, err)
	}
}
