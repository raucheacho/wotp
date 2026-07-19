package project

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/ws"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	control, err := store.NewSQLiteControlStore(filepath.Join(dir, "control.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { control.Close() })

	templatesPath := filepath.Join(dir, "templates.toml")
	templatesTOML := "[en]\notp_message = \"code: {{code}}\"\n"
	if err := os.WriteFile(templatesPath, []byte(templatesTOML), 0644); err != nil {
		t.Fatalf("write templates.toml: %v", err)
	}

	hub := ws.NewHub(logger)
	return NewRegistry(control, dir, templatesPath, hub, logger)
}

func TestRegistry_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	p, err := r.Create(ctx, "acme", "Acme Corp")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rt, err := r.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rt.Project.Slug != "acme" {
		t.Fatalf("rt.Project.Slug = %q, want acme", rt.Project.Slug)
	}
	if rt.Settings.OTP.CodeLength != 6 {
		t.Fatalf("rt.Settings.OTP.CodeLength = %d, want 6 (default)", rt.Settings.OTP.CodeLength)
	}
	if !rt.Settings.WhatsApp.IgnoreGroups || !rt.Settings.WhatsApp.IgnoreStatus {
		t.Fatalf("expected safe defaults IgnoreGroups/IgnoreStatus = true, got %+v", rt.Settings.WhatsApp)
	}

	msg, err := rt.Templates.Render("en", "123456", 5)
	if err != nil {
		t.Fatalf("Templates.Render: %v", err)
	}
	if msg != "code: 123456" {
		t.Fatalf("Render = %q, want %q", msg, "code: 123456")
	}
}

func TestRegistry_GetIsCachedAndConcurrencySafe(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	p, err := r.Create(ctx, "acme", "Acme Corp")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rt1, err := r.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get 1: %v", err)
	}
	rt2, err := r.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get 2: %v", err)
	}
	if rt1 != rt2 {
		t.Fatalf("expected Get to return the cached Runtime pointer, got two different instances")
	}
}

func TestRegistry_ProjectsAreDataIsolated(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	projA, err := r.Create(ctx, "proj-a", "A")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	projB, err := r.Create(ctx, "proj-b", "B")
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	rtA, err := r.Get(ctx, projA.ID)
	if err != nil {
		t.Fatalf("Get A: %v", err)
	}
	rtB, err := r.Get(ctx, projB.ID)
	if err != nil {
		t.Fatalf("Get B: %v", err)
	}

	if _, err := rtA.Engine.Send(ctx, "+15551234567"); err != nil {
		t.Fatalf("Send OTP in project A: %v", err)
	}

	otpsA, err := rtA.Store.GetRecentOTPs(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentOTPs A: %v", err)
	}
	if len(otpsA) != 1 {
		t.Fatalf("project A should see 1 OTP, got %d", len(otpsA))
	}

	otpsB, err := rtB.Store.GetRecentOTPs(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentOTPs B: %v", err)
	}
	if len(otpsB) != 0 {
		t.Fatalf("project B should not see project A's OTPs, got %d", len(otpsB))
	}

	// The two projects must be backed by separate files on disk.
	dataA := filepath.Join(r.dataDir, "projects", projA.ID, "data.db")
	dataB := filepath.Join(r.dataDir, "projects", projB.ID, "data.db")
	if _, err := os.Stat(dataA); err != nil {
		t.Fatalf("expected data.db for project A to exist: %v", err)
	}
	if _, err := os.Stat(dataB); err != nil {
		t.Fatalf("expected data.db for project B to exist: %v", err)
	}
}

func TestRegistry_CreateRejectsDuplicateSlug(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	if _, err := r.Create(ctx, "acme", "Acme Corp"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := r.Create(ctx, "acme", "Acme Corp Again"); err == nil {
		t.Fatal("expected error creating a project with a duplicate slug")
	}
}

func TestRegistry_Delete(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	p, err := r.Create(ctx, "acme", "Acme Corp")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := r.Get(ctx, p.ID); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if err := r.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	projectDir := filepath.Join(r.dataDir, "projects", p.ID)
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Fatalf("expected project data dir to be removed, stat err = %v", err)
	}

	gone, err := r.control.GetProjectByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID after delete: %v", err)
	}
	if gone != nil {
		t.Fatalf("expected project row to be deleted, got %+v", gone)
	}
}

// TestRegistry_SyncNumbersRunsAutomaticallyOnLoad is a regression test: a
// project's number must be synced to the control-plane registry as soon as
// its Runtime is loaded, not left for some later step.
func TestRegistry_SyncNumbersRunsAutomaticallyOnLoad(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	p, err := r.Create(ctx, "acme", "Acme Corp")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// No numbers paired yet — Get (and the sync it triggers) must not error
	// just because the pool is empty.
	rt, err := r.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(rt.WA.Numbers()) != 0 {
		t.Fatalf("expected a freshly created project to have no numbers, got %+v", rt.WA.Numbers())
	}

	numbers, err := r.control.ListNumbersByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListNumbersByProject: %v", err)
	}
	if len(numbers) != 0 {
		t.Fatalf("expected no persisted numbers for a pool with none, got %+v", numbers)
	}
}

func TestRegistry_SyncNumbersErrorsWhenProjectNotLoaded(t *testing.T) {
	r := newTestRegistry(t)
	if err := r.SyncNumbers(context.Background(), "not-a-real-project-id"); err == nil {
		t.Fatal("expected an error when syncing a project that was never loaded")
	}
}

// enableProjectCloud enables the Cloud API backend on an existing project
// via its settings blob directly (bypassing the HTTP settings PATCH
// endpoint, which lives in package api) — the same field
// (project.Settings.Cloud) that a real PATCH /v1/projects/{id}/settings
// call would update.
func enableProjectCloud(t *testing.T, r *Registry, projectID, phoneNumberID, accessToken string) {
	t.Helper()
	settings := DefaultSettings()
	settings.Cloud.Enabled = true
	settings.Cloud.PhoneNumberID = phoneNumberID
	settings.Cloud.AccessToken = accessToken
	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := r.control.UpdateProjectSettings(context.Background(), projectID, string(b)); err != nil {
		t.Fatalf("UpdateProjectSettings: %v", err)
	}
}

// TestRegistry_PerProjectCloudSettingsPopulateRuntimeCloud is a regression
// test for the Meta Cloud API backend being per-project (not instance-wide):
// enabling it via one project's Settings.Cloud must not affect another
// project on the same registry, and must make the Runtime carry a working
// Cloud client alongside its (independent, still-whatsmeow) WA pool.
func TestRegistry_PerProjectCloudSettingsPopulateRuntimeCloud(t *testing.T) {
	ctx := context.Background()

	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"id": "test-phone-id", "display_phone_number": "+1 555 0100",
		})
	}))
	defer metaServer.Close()

	r := newTestRegistry(t)
	r.cloudBaseURLOverride = metaServer.URL
	r.cloudHTTPClientOverride = metaServer.Client()

	cloudProj, err := r.Create(ctx, "acme-otp", "Acme OTP")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	enableProjectCloud(t, r, cloudProj.ID, "test-phone-id", "test-token")

	plainProj, err := r.Create(ctx, "acme-chat", "Acme Chat")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rt, err := r.Get(ctx, cloudProj.ID)
	if err != nil {
		t.Fatalf("Get cloud project: %v", err)
	}
	if rt.Cloud == nil {
		t.Fatal("expected rt.Cloud to be set for the project with Cloud enabled")
	}
	if !rt.Cloud.IsConnected() {
		t.Fatal("expected rt.Cloud to have verified its credentials against the (fake) Meta API during load")
	}
	if rt.WA == nil {
		t.Fatal("expected rt.WA (whatsmeow pool) to still be set independently of Cloud")
	}

	otherRt, err := r.Get(ctx, plainProj.ID)
	if err != nil {
		t.Fatalf("Get plain project: %v", err)
	}
	if otherRt.Cloud != nil {
		t.Fatal("expected a different project with Cloud disabled to have a nil rt.Cloud, even though the registry has a cloud test override")
	}
}
