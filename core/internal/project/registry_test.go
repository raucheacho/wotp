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
	"strings"
	"testing"

	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/ws"
)

func newTestRuntime(t *testing.T) (*Runtime, store.ControlStore, string) {
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

	rt, err := Load(context.Background(), control, hub, logger, LoadOptions{
		InstanceName:  "Acme Corp",
		DataDir:       dir,
		TemplatesPath: templatesPath,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(rt.Close)

	return rt, control, dir
}

func TestLoad_BuildsRuntimeWithDefaults(t *testing.T) {
	rt, _, _ := newTestRuntime(t)

	if rt.Name != "Acme Corp" {
		t.Fatalf("rt.Name = %q, want Acme Corp", rt.Name)
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

func TestLoad_DataFilesLiveDirectlyUnderDataDir(t *testing.T) {
	rt, _, dir := newTestRuntime(t)

	if _, err := rt.Engine.Send(context.Background(), "+15551234567"); err != nil {
		t.Fatalf("Send OTP: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "data.db")); err != nil {
		t.Fatalf("expected data.db directly under DataDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "session.db")); err != nil {
		t.Fatalf("expected session.db directly under DataDir: %v", err)
	}
}

// TestLoad_SyncNumbersRunsAutomaticallyOnLoad is a regression test: the
// instance's number must be synced to the control-plane registry as soon as
// Load builds the Runtime, not left for some later step.
func TestLoad_SyncNumbersRunsAutomaticallyOnLoad(t *testing.T) {
	rt, control, _ := newTestRuntime(t)

	// No number paired yet — Load (and the sync it triggers) must not error
	// just because the pool is empty.
	if len(rt.WA.Numbers()) != 0 {
		t.Fatalf("expected a freshly loaded instance to have no numbers, got %+v", rt.WA.Numbers())
	}

	numbers, err := control.ListNumbers(context.Background())
	if err != nil {
		t.Fatalf("ListNumbers: %v", err)
	}
	if len(numbers) != 0 {
		t.Fatalf("expected no persisted numbers for a pool with none, got %+v", numbers)
	}
}

func TestSyncNumbers_ErrorsAreNotFatalOnEmptyPool(t *testing.T) {
	rt, control, _ := newTestRuntime(t)
	if err := SyncNumbers(context.Background(), control, rt.WA); err != nil {
		t.Fatalf("SyncNumbers on an empty pool should not error: %v", err)
	}
}

// enableCloudSettings enables the Cloud API backend directly on control's
// settings blob (bypassing the HTTP settings PATCH endpoint, which lives in
// package api) — the same field (project.Settings.Cloud) a real
// PATCH /v1/settings call would update.
func enableCloudSettings(t *testing.T, control store.ControlStore, phoneNumberID, accessToken string) {
	t.Helper()
	settings := DefaultSettings()
	settings.Cloud.Enabled = true
	settings.Cloud.PhoneNumberID = phoneNumberID
	settings.Cloud.AccessToken = accessToken
	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := control.UpdateSettings(context.Background(), string(b)); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
}

// TestLoad_CloudSettingsPopulateRuntimeCloud is a regression test: enabling
// Settings.Cloud must make the Runtime carry a working Cloud client
// alongside its (independent, still-whatsmeow) WA pool.
func TestLoad_CloudSettingsPopulateRuntimeCloud(t *testing.T) {
	ctx := context.Background()

	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"id": "test-phone-id", "display_phone_number": "+1 555 0100",
		})
	}))
	defer metaServer.Close()

	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	control, err := store.NewSQLiteControlStore(filepath.Join(dir, "control.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { control.Close() })
	enableCloudSettings(t, control, "test-phone-id", "test-token")

	templatesPath := filepath.Join(dir, "templates.toml")
	if err := os.WriteFile(templatesPath, []byte("[en]\notp_message = \"code: {{code}}\"\n"), 0644); err != nil {
		t.Fatalf("write templates.toml: %v", err)
	}

	hub := ws.NewHub(logger)
	rt, err := Load(ctx, control, hub, logger, LoadOptions{
		InstanceName:            "Acme OTP",
		DataDir:                 dir,
		TemplatesPath:           templatesPath,
		CloudBaseURLOverride:    metaServer.URL,
		CloudHTTPClientOverride: metaServer.Client(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(rt.Close)

	if rt.Cloud == nil {
		t.Fatal("expected rt.Cloud to be set when Settings.Cloud.Enabled is true")
	}
	if !rt.Cloud.IsConnected() {
		t.Fatal("expected rt.Cloud to have verified its credentials against the (fake) Meta API during load")
	}
	if rt.WA == nil {
		t.Fatal("expected rt.WA (whatsmeow pool) to still be set independently of Cloud")
	}
}

// TestLoad_RegistersForInboundWhenWabaIDAndPinSet is a regression test for
// the registration step Meta requires before it'll actually deliver
// inbound webhooks: Load must call /register then /subscribed_apps when
// WabaID+Pin are configured, and must NOT call either for a send-only
// (OTP/messages) Cloud setup that leaves them empty.
func TestLoad_RegistersForInboundWhenWabaIDAndPinSet(t *testing.T) {
	ctx := context.Background()

	var registerCalled, subscribeCalled bool
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/register"):
			registerCalled = true
			if r.Method != http.MethodPost {
				t.Errorf("register: method = %s, want POST", r.Method)
			}
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["pin"] != "123456" {
				t.Errorf("register: pin = %q, want 123456", body["pin"])
			}
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
			subscribeCalled = true
			if r.Method != http.MethodPost {
				t.Errorf("subscribed_apps: method = %s, want POST", r.Method)
			}
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			json.NewEncoder(w).Encode(map[string]string{
				"id": "test-phone-id", "display_phone_number": "+1 555 0100",
			})
		}
	}))
	defer metaServer.Close()

	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	control, err := store.NewSQLiteControlStore(filepath.Join(dir, "control.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { control.Close() })

	settings := DefaultSettings()
	settings.Cloud.Enabled = true
	settings.Cloud.PhoneNumberID = "test-phone-id"
	settings.Cloud.AccessToken = "test-token"
	settings.Cloud.WabaID = "test-waba-id"
	settings.Cloud.Pin = "123456"
	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := control.UpdateSettings(ctx, string(b)); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	templatesPath := filepath.Join(dir, "templates.toml")
	if err := os.WriteFile(templatesPath, []byte("[en]\notp_message = \"code: {{code}}\"\n"), 0644); err != nil {
		t.Fatalf("write templates.toml: %v", err)
	}

	hub := ws.NewHub(logger)
	rt, err := Load(ctx, control, hub, logger, LoadOptions{
		InstanceName:            "Acme Inbound",
		DataDir:                 dir,
		TemplatesPath:           templatesPath,
		CloudBaseURLOverride:    metaServer.URL,
		CloudHTTPClientOverride: metaServer.Client(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(rt.Close)

	if !registerCalled {
		t.Error("expected Load to call /register when WabaID+Pin are set")
	}
	if !subscribeCalled {
		t.Error("expected Load to call /subscribed_apps when WabaID+Pin are set")
	}
}

// TestLoad_SkipsRegistrationWithoutPin is a regression test for the other
// half of the same behavior: a send-only Cloud setup (no Pin) must not
// attempt registration at all — Meta's /register fails loudly without one,
// and OTP/message sending never needed it.
func TestLoad_SkipsRegistrationWithoutPin(t *testing.T) {
	ctx := context.Background()

	var registerCalled bool
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/register") {
			registerCalled = true
		}
		json.NewEncoder(w).Encode(map[string]string{
			"id": "test-phone-id", "display_phone_number": "+1 555 0100",
		})
	}))
	defer metaServer.Close()

	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	control, err := store.NewSQLiteControlStore(filepath.Join(dir, "control.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { control.Close() })
	enableCloudSettings(t, control, "test-phone-id", "test-token")

	templatesPath := filepath.Join(dir, "templates.toml")
	if err := os.WriteFile(templatesPath, []byte("[en]\notp_message = \"code: {{code}}\"\n"), 0644); err != nil {
		t.Fatalf("write templates.toml: %v", err)
	}

	hub := ws.NewHub(logger)
	rt, err := Load(ctx, control, hub, logger, LoadOptions{
		InstanceName:            "Acme SendOnly",
		DataDir:                 dir,
		TemplatesPath:           templatesPath,
		CloudBaseURLOverride:    metaServer.URL,
		CloudHTTPClientOverride: metaServer.Client(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(rt.Close)

	if registerCalled {
		t.Error("expected Load to skip /register entirely when Pin is empty")
	}
}
