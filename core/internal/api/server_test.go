package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wotp/core/internal/config"
	"github.com/wotp/core/internal/keys"
	"github.com/wotp/core/internal/project"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

type testEnv struct {
	server  *Server
	router  http.Handler
	control store.ControlStore
	keyMgr  *keys.Manager
	rootKey string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	control, err := store.NewSQLiteControlStore(filepath.Join(dir, "control.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteControlStore: %v", err)
	}
	t.Cleanup(func() { control.Close() })

	templatesPath := filepath.Join(dir, "templates.toml")
	if err := os.WriteFile(templatesPath, []byte("[en]\notp_message = \"code: {{code}}\"\n"), 0644); err != nil {
		t.Fatalf("write templates.toml: %v", err)
	}

	wsHub := ws.NewHub(logger)
	registry := project.NewRegistry(control, dir, templatesPath, wsHub, logger)
	keyMgr := keys.NewManager(control)

	cfg := &config.Config{
		API: config.APIConfig{Port: 54321, EnableDashboard: true},
	}
	server := NewServer(cfg, control, registry, keyMgr, wsHub, logger)
	registry.SetOnRuntimeLoaded(server.StartEventForwarder)

	rootKey, err := keyMgr.EnsureRootKey(context.Background())
	if err != nil {
		t.Fatalf("EnsureRootKey: %v", err)
	}

	return &testEnv{
		server:  server,
		router:  server.Router(),
		control: control,
		keyMgr:  keyMgr,
		rootKey: rootKey.FullKey,
	}
}

func (e *testEnv) do(t *testing.T, method, path, apiKey string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if apiKey != "" {
		req.Header.Set("apikey", apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) createProject(t *testing.T, slug string) (projectID, anonKey string) {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/v1/projects", e.rootKey, CreateProjectRequest{Slug: slug, Name: slug})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project %q: status = %d, body = %s", slug, rec.Code, rec.Body.String())
	}
	var resp struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		AnonKey string `json:"anon_key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode create project response: %v", err)
	}
	return resp.Project.ID, resp.AnonKey
}

func TestHealth_PublicNoAuth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/v1/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status field = %v, want \"ok\"", resp["status"])
	}
	if _, ok := resp["phone"]; ok {
		t.Fatal("health response should no longer carry a single global phone field")
	}
}

func TestAuth_MissingAndInvalidKey(t *testing.T) {
	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/v1/messages", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no apikey: status = %d, want 401", rec.Code)
	}

	rec = env.do(t, http.MethodGet, "/v1/messages", "wotp_anon_garbage", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("garbage apikey: status = %d, want 401", rec.Code)
	}
}

func TestAuth_WrongTierIsForbidden(t *testing.T) {
	env := newTestEnv(t)
	_, anonKey := env.createProject(t, "acme")

	// An anon key must not be able to reach root-only endpoints.
	rec := env.do(t, http.MethodGet, "/v1/projects", anonKey, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("anon key on root endpoint: status = %d, want 403", rec.Code)
	}
}

func TestRootKey_CreateAndListProjects(t *testing.T) {
	env := newTestEnv(t)
	projectID, anonKey := env.createProject(t, "acme")
	if projectID == "" || anonKey == "" {
		t.Fatal("expected a project id and anon key from project creation")
	}

	rec := env.do(t, http.MethodGet, "/v1/projects", env.rootKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list projects: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var projects []store.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &projects); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	found := false
	for _, p := range projects {
		if p.ID == projectID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created project %s not found in list: %+v", projectID, projects)
	}
}

func TestProjectDataIsolation(t *testing.T) {
	env := newTestEnv(t)
	projA, keyA := env.createProject(t, "proj-a")
	_, keyB := env.createProject(t, "proj-b")

	// Bypass WhatsApp entirely (no numbers paired in tests) and seed a
	// generic message directly into project A's store.
	rt, err := env.server.registry.Get(context.Background(), projA)
	if err != nil {
		t.Fatalf("registry.Get: %v", err)
	}
	if err := rt.Store.SaveGenericMessage(context.Background(), &store.GenericMessage{
		ID:          "msg-1",
		Phone:       "+15551234567",
		MessageType: "text",
		Content:     "hello",
		Status:      "sent",
	}); err != nil {
		t.Fatalf("SaveGenericMessage: %v", err)
	}

	recA := env.do(t, http.MethodGet, "/v1/messages", keyA, nil)
	if recA.Code != http.StatusOK {
		t.Fatalf("list messages as A: status = %d, body = %s", recA.Code, recA.Body.String())
	}
	var msgsA []store.GenericMessage
	if err := json.Unmarshal(recA.Body.Bytes(), &msgsA); err != nil {
		t.Fatalf("decode messages A: %v", err)
	}
	if len(msgsA) != 1 {
		t.Fatalf("project A should see its own 1 message, got %d", len(msgsA))
	}

	recB := env.do(t, http.MethodGet, "/v1/messages", keyB, nil)
	if recB.Code != http.StatusOK {
		t.Fatalf("list messages as B: status = %d, body = %s", recB.Code, recB.Body.String())
	}
	var msgsB []store.GenericMessage
	if err := json.Unmarshal(recB.Body.Bytes(), &msgsB); err != nil {
		t.Fatalf("decode messages B: %v", err)
	}
	if len(msgsB) != 0 {
		t.Fatalf("project B must not see project A's messages, got %d", len(msgsB))
	}
}

func TestOTPSend_SoftFailsWithoutAPairedNumber(t *testing.T) {
	env := newTestEnv(t)
	_, anonKey := env.createProject(t, "acme")

	rec := env.do(t, http.MethodPost, "/v1/otp/send", anonKey, OTPSendRequest{Phone: "+15551234567"})
	if rec.Code != http.StatusOK {
		t.Fatalf("otp send: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["token"] == "" {
		t.Fatal("expected an OTP token even when the WhatsApp send itself fails")
	}
	if resp["warning"] != "message_send_failed" {
		t.Fatalf("warning = %q, want message_send_failed (no number is paired in this test)", resp["warning"])
	}
}

// TestOTPSend_UsesCloudBackendTemplateWhenConfigured verifies that
// handleOTPSend prefers rt.Cloud (a Meta Cloud API-backed project) over the
// whatsmeow pool when both are configured, sending the OTP as a template
// (the only valid way to send one on that backend, see
// whatsapp.TemplateOTPSender) rather than trying to render + send free
// text, which would fail on Cloud API outside the 24h window.
func TestOTPSend_UsesCloudBackendTemplateWhenConfigured(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.cloud-otp"}},
		})
	}))
	defer metaServer.Close()

	env := newTestEnv(t)
	projectID, anonKey := env.createProject(t, "acme")

	// Enabling Cloud is normally done via PATCH /v1/projects/{id}/settings
	// (project.Settings.Cloud) — this test injects rt.Cloud directly since
	// it's only exercising handleOTPSend's preference for Cloud over
	// whatsmeow, not the settings-to-Runtime wiring (covered by
	// core/internal/project/registry_test.go).
	rt, err := env.server.registry.Get(context.Background(), projectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	rt.Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID:       "test-phone-id",
		AccessToken:         "test-token",
		OTPTemplateName:     "otp_verification",
		OTPTemplateLanguage: "en_US",
		BaseURL:             metaServer.URL,
		HTTPClient:          metaServer.Client(),
	})

	rec := env.do(t, http.MethodPost, "/v1/otp/send", anonKey, OTPSendRequest{Phone: "+15551234567"})
	if rec.Code != http.StatusOK {
		t.Fatalf("otp send: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["token"] == "" {
		t.Fatal("expected an OTP token")
	}
	if resp["warning"] != "" {
		t.Fatalf("warning = %q, want none — the cloud send should have succeeded", resp["warning"])
	}

	if gotPath != "/test-phone-id/messages" {
		t.Fatalf("expected the send to hit /test-phone-id/messages, got %q", gotPath)
	}
	if gotBody["type"] != "template" {
		t.Fatalf("expected a template send (Cloud API can't send OTP as free text), got type=%v", gotBody["type"])
	}
	tpl, _ := gotBody["template"].(map[string]any)
	if tpl["name"] != "otp_verification" {
		t.Fatalf("template.name = %v, want otp_verification", tpl["name"])
	}
}

// TestCloudStatus_ReflectsPerProjectState covers the dashboard's new
// cloud-status endpoint: disabled by default, and reporting connected once
// rt.Cloud is set and verified — the same rt.Cloud a real PATCH
// /v1/projects/{id}/settings would populate (see registry_test.go for that
// wiring; this test injects rt.Cloud directly, same technique as the OTP
// send test above).
func TestCloudStatus_ReflectsPerProjectState(t *testing.T) {
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"id": "test-phone-id", "display_phone_number": "+1 555 0100",
		})
	}))
	defer metaServer.Close()

	env := newTestEnv(t)
	projectID, _ := env.createProject(t, "acme")

	rec := env.do(t, http.MethodGet, "/dashboard/api/cloud-status?project_id="+projectID, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("cloud-status: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var status CloudStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Enabled {
		t.Fatal("expected Cloud to be disabled by default")
	}

	rt, err := env.server.registry.Get(context.Background(), projectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	rt.Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID: "test-phone-id",
		AccessToken:   "test-token",
		BaseURL:       metaServer.URL,
		HTTPClient:    metaServer.Client(),
	})
	if _, err := rt.Cloud.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	rec = env.do(t, http.MethodGet, "/dashboard/api/cloud-status?project_id="+projectID, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("cloud-status: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !status.Enabled || !status.Connected {
		t.Fatalf("expected enabled+connected once rt.Cloud is set and verified, got %+v", status)
	}
	if status.DisplayPhone != "+1 555 0100" {
		t.Fatalf("DisplayPhone = %q, want +1 555 0100", status.DisplayPhone)
	}
}

// projectCloudSettings reads back a project's persisted Cloud settings
// directly from control.db, bypassing Registry.Get/load — which would
// build a real *whatsapp.CloudClient and try to verify it against the
// actual Meta Graph API over the network (slow, and pointless for a test
// that's only checking what got persisted).
func projectCloudSettings(t *testing.T, env *testEnv, projectID string) project.Settings {
	t.Helper()
	p, err := env.control.GetProjectByID(context.Background(), projectID)
	if err != nil || p == nil {
		t.Fatalf("GetProjectByID: %v", err)
	}
	settings := project.DefaultSettings()
	if err := json.Unmarshal([]byte(p.SettingsJSON), &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return settings
}

// TestDashboardCloudSettings_ConfigureAndDisablePreservesFields covers the
// dashboard's Cloud API configuration form end to end: enabling it persists
// the non-secret fields, and disabling it — the "uncheck Enabled and save"
// flow, resubmitting the same fields — must not wipe
// phone_number_id/template config a re-enable would need.
func TestDashboardCloudSettings_ConfigureAndDisablePreservesFields(t *testing.T) {
	env := newTestEnv(t)
	projectID, _ := env.createProject(t, "acme")

	rec := env.do(t, http.MethodPost, "/dashboard/api/cloud-settings?project_id="+projectID, "", CloudSettingsRequest{
		Enabled:             true,
		PhoneNumberID:       "test-phone-id",
		AccessToken:         "test-token",
		OTPTemplateName:     "otp_verification",
		OTPTemplateLanguage: "en_US",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("configure cloud settings: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	settings := projectCloudSettings(t, env, projectID)
	if !settings.Cloud.Enabled || settings.Cloud.PhoneNumberID != "test-phone-id" || settings.Cloud.OTPTemplateName != "otp_verification" {
		t.Fatalf("unexpected settings after enabling: %+v", settings.Cloud)
	}

	// Disable without resending the access token (the dashboard form never
	// gets it back from the status endpoint) — phone_number_id/template
	// fields must survive since the dashboard resubmits them from the
	// status it just displayed.
	rec = env.do(t, http.MethodPost, "/dashboard/api/cloud-settings?project_id="+projectID, "", CloudSettingsRequest{
		Enabled:             false,
		PhoneNumberID:       settings.Cloud.PhoneNumberID,
		OTPTemplateName:     settings.Cloud.OTPTemplateName,
		OTPTemplateLanguage: settings.Cloud.OTPTemplateLanguage,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable cloud settings: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	settings = projectCloudSettings(t, env, projectID)
	if settings.Cloud.Enabled {
		t.Fatal("expected Cloud.Enabled to be false after disabling")
	}
	if settings.Cloud.PhoneNumberID != "test-phone-id" {
		t.Fatalf("PhoneNumberID = %q, want it preserved across disable", settings.Cloud.PhoneNumberID)
	}
	if settings.Cloud.AccessToken != "test-token" {
		t.Fatal("expected the access token to survive disabling even though it wasn't resent")
	}
}

// The "one number per project" rule itself is covered at the source in
// core/internal/whatsapp/pool_test.go (TestPool_PairRefusesWhenAlreadyPaired)
// — Pair() needs a real paired device to trigger, which this package's
// httptest-based env can't fake without a network round-trip, so the HTTP
// wiring (errors.Is(err, whatsapp.ErrAlreadyPaired) -> 409 in startPairing)
// is covered by code review rather than an integration test here.

func TestRegenerateProjectKey(t *testing.T) {
	env := newTestEnv(t)
	projA, oldAnonKey := env.createProject(t, "acme")

	rec := env.do(t, http.MethodPost, "/v1/projects/"+projA+"/keys/regenerate", env.rootKey, RegenerateKeyRequest{Tier: "anon"})
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate key: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["key"] == "" || resp["key"] == oldAnonKey {
		t.Fatalf("expected a fresh anon key, got %q (old was %q)", resp["key"], oldAnonKey)
	}

	// The old key must no longer work.
	rec = env.do(t, http.MethodGet, "/v1/messages", oldAnonKey, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old key after regenerate: status = %d, want 401", rec.Code)
	}

	// The new key must work.
	rec = env.do(t, http.MethodGet, "/v1/messages", resp["key"], nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("new key: status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestProjectSettings_UpdateIsPartialMergeAndTakesEffect(t *testing.T) {
	env := newTestEnv(t)
	projID, _ := env.createProject(t, "acme")

	// Load once so a Runtime is cached, to prove Evict() picks up the change.
	rt1, err := env.server.registry.Get(context.Background(), projID)
	if err != nil {
		t.Fatalf("registry.Get: %v", err)
	}
	if rt1.Settings.Webhooks.Endpoint != "" {
		t.Fatalf("expected no webhook endpoint by default, got %q", rt1.Settings.Webhooks.Endpoint)
	}
	if rt1.Settings.OTP.CodeLength != 6 {
		t.Fatalf("expected default code_length 6, got %d", rt1.Settings.OTP.CodeLength)
	}

	// Patch only the webhook endpoint.
	body := map[string]any{
		"webhooks": map[string]any{"endpoint": "http://localhost:9999/hook", "events": []string{"message.received"}},
	}
	rec := env.do(t, http.MethodPatch, "/v1/projects/"+projID+"/settings", env.rootKey, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("update settings: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// OTP settings (not part of the patch) must survive untouched.
	var updated struct {
		OTP struct {
			CodeLength int `json:"code_length"`
		} `json:"otp"`
		Webhooks struct {
			Endpoint string `json:"endpoint"`
		} `json:"webhooks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.OTP.CodeLength != 6 {
		t.Fatalf("partial update clobbered otp.code_length: got %d, want 6", updated.OTP.CodeLength)
	}
	if updated.Webhooks.Endpoint != "http://localhost:9999/hook" {
		t.Fatalf("webhooks.endpoint = %q, want the patched value", updated.Webhooks.Endpoint)
	}

	// The cached Runtime must have been evicted — Get() rebuilds with the new settings.
	rt2, err := env.server.registry.Get(context.Background(), projID)
	if err != nil {
		t.Fatalf("registry.Get after update: %v", err)
	}
	if rt2 == rt1 {
		t.Fatal("expected a fresh Runtime after settings update (old one should have been evicted)")
	}
	if rt2.Settings.Webhooks.Endpoint != "http://localhost:9999/hook" {
		t.Fatalf("reloaded runtime webhook endpoint = %q, want the patched value", rt2.Settings.Webhooks.Endpoint)
	}
}

