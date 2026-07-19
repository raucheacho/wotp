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

func TestControlStore_ProjectLifecycle(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	p := &Project{ID: uuid.New().String(), Slug: "acme", Name: "Acme Corp", CreatedAt: time.Now()}
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	byID, err := s.GetProjectByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID: %v", err)
	}
	if byID == nil || byID.Slug != "acme" {
		t.Fatalf("GetProjectByID = %+v, want slug acme", byID)
	}

	bySlug, err := s.GetProjectBySlug(ctx, "acme")
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	if bySlug == nil || bySlug.ID != p.ID {
		t.Fatalf("GetProjectBySlug = %+v, want id %s", bySlug, p.ID)
	}

	list, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListProjects returned %d projects, want 1", len(list))
	}

	if err := s.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	gone, err := s.GetProjectByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID after delete: %v", err)
	}
	if gone != nil {
		t.Fatalf("expected project to be deleted, got %+v", gone)
	}
}

func TestControlStore_APIKeysScopedByProject(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	projA := &Project{ID: uuid.New().String(), Slug: "proj-a", Name: "A", CreatedAt: time.Now()}
	projB := &Project{ID: uuid.New().String(), Slug: "proj-b", Name: "B", CreatedAt: time.Now()}
	if err := s.CreateProject(ctx, projA); err != nil {
		t.Fatalf("CreateProject A: %v", err)
	}
	if err := s.CreateProject(ctx, projB); err != nil {
		t.Fatalf("CreateProject B: %v", err)
	}

	keyA := &APIKey{ID: uuid.New().String(), ProjectID: projA.ID, KeyHash: "hashA", KeyPrefix: "wotp_anon_aaaaaaaa", Tier: "anon", CreatedAt: time.Now()}
	keyB := &APIKey{ID: uuid.New().String(), ProjectID: projB.ID, KeyHash: "hashB", KeyPrefix: "wotp_anon_bbbbbbbb", Tier: "anon", CreatedAt: time.Now()}
	if err := s.CreateAPIKey(ctx, keyA); err != nil {
		t.Fatalf("CreateAPIKey A: %v", err)
	}
	if err := s.CreateAPIKey(ctx, keyB); err != nil {
		t.Fatalf("CreateAPIKey B: %v", err)
	}

	got, err := s.GetAPIKeyByPrefix(ctx, "wotp_anon_aaaaaaaa")
	if err != nil {
		t.Fatalf("GetAPIKeyByPrefix: %v", err)
	}
	if got == nil || got.ProjectID != projA.ID {
		t.Fatalf("GetAPIKeyByPrefix = %+v, want project %s", got, projA.ID)
	}

	listA, err := s.ListAPIKeysByProject(ctx, projA.ID)
	if err != nil {
		t.Fatalf("ListAPIKeysByProject A: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != keyA.ID {
		t.Fatalf("ListAPIKeysByProject A = %+v, want only keyA", listA)
	}

	if err := s.DeleteAPIKeysByProjectAndTier(ctx, projA.ID, "anon"); err != nil {
		t.Fatalf("DeleteAPIKeysByProjectAndTier: %v", err)
	}
	afterDelete, err := s.ListAPIKeysByProject(ctx, projA.ID)
	if err != nil {
		t.Fatalf("ListAPIKeysByProject after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("expected no keys left for project A, got %d", len(afterDelete))
	}

	// Project B's key must be untouched by A's deletion.
	listB, err := s.ListAPIKeysByProject(ctx, projB.ID)
	if err != nil {
		t.Fatalf("ListAPIKeysByProject B: %v", err)
	}
	if len(listB) != 1 {
		t.Fatalf("expected project B key to survive, got %d keys", len(listB))
	}
}

// TestControlStore_NumbersUpsertAndStatus covers the numbers table now that
// a project has at most one number (see whatsapp.Pool) — no more
// policy_order/round-robin ordering to test, just upsert-in-place and
// status updates.
func TestControlStore_NumbersUpsertAndStatus(t *testing.T) {
	s := newTestControlStore(t)
	ctx := context.Background()

	proj := &Project{ID: uuid.New().String(), Slug: "acme", Name: "Acme", CreatedAt: time.Now()}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	n0 := &Number{JID: "000@s.whatsapp.net", ProjectID: proj.ID, Phone: "+0000", Status: NumberStatusPending, CreatedAt: time.Now()}
	if err := s.UpsertNumber(ctx, n0); err != nil {
		t.Fatalf("UpsertNumber: %v", err)
	}

	numbers, err := s.ListNumbersByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListNumbersByProject: %v", err)
	}
	if len(numbers) != 1 || numbers[0].JID != n0.JID {
		t.Fatalf("ListNumbersByProject = %+v, want [n0]", numbers)
	}

	if err := s.UpdateNumberStatus(ctx, n0.JID, NumberStatusConnected); err != nil {
		t.Fatalf("UpdateNumberStatus: %v", err)
	}
	updated, err := s.ListNumbersByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListNumbersByProject after status update: %v", err)
	}
	if updated[0].Status != NumberStatusConnected {
		t.Fatalf("n0 status = %q, want connected", updated[0].Status)
	}

	// Upsert on an existing JID updates in place rather than duplicating.
	n0Updated := &Number{JID: n0.JID, ProjectID: proj.ID, Phone: "+0000", Status: NumberStatusDisconnected, CreatedAt: time.Now()}
	if err := s.UpsertNumber(ctx, n0Updated); err != nil {
		t.Fatalf("UpsertNumber (update): %v", err)
	}
	final, err := s.ListNumbersByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListNumbersByProject after upsert: %v", err)
	}
	if len(final) != 1 {
		t.Fatalf("expected upsert to update in place, got %d rows", len(final))
	}
}
