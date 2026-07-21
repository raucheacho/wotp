package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	server     *Server
	router     http.Handler
	control    store.ControlStore
	keyMgr     *keys.Manager
	anonKey    string
	serviceKey string
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
	keyMgr := keys.NewManager(control)

	loadOpts := project.LoadOptions{
		InstanceName:  "Test Instance",
		DataDir:       dir,
		TemplatesPath: templatesPath,
	}
	reload := func(ctx context.Context) (*project.Runtime, error) {
		return project.Load(ctx, control, wsHub, logger, loadOpts)
	}

	cfg := &config.Config{
		API: config.APIConfig{Port: 54321, EnableDashboard: true},
	}

	rt, err := reload(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(rt.Close)

	server := NewServer(cfg, control, rt, reload, keyMgr, wsHub, logger)
	go server.StartEventForwarder(rt)

	anonKey, serviceKey, err := keyMgr.EnsureKeys(context.Background())
	if err != nil {
		t.Fatalf("EnsureKeys: %v", err)
	}

	return &testEnv{
		server:     server,
		router:     server.Router(),
		control:    control,
		keyMgr:     keyMgr,
		anonKey:    anonKey.FullKey,
		serviceKey: serviceKey.FullKey,
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

	// An anon key must not be able to reach service-only endpoints.
	rec := env.do(t, http.MethodGet, "/v1/numbers", env.anonKey, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("anon key on service-only endpoint: status = %d, want 403", rec.Code)
	}
}

func TestOTPSend_SoftFailsWithoutAPairedNumber(t *testing.T) {
	env := newTestEnv(t)

	rec := env.do(t, http.MethodPost, "/v1/otp/send", env.anonKey, OTPSendRequest{Phone: "+15551234567"})
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
// handleOTPSend prefers rt.Cloud (a Meta Cloud API-backed instance) over the
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

	// Enabling Cloud is normally done via PATCH /v1/settings
	// (project.Settings.Cloud) — this test injects rt.Cloud directly since
	// it's only exercising handleOTPSend's preference for Cloud over
	// whatsmeow, not the settings-to-Runtime wiring (covered by
	// core/internal/project/registry_test.go).
	env.server.runtime().Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID:       "test-phone-id",
		AccessToken:         "test-token",
		OTPTemplateName:     "otp_verification",
		OTPTemplateLanguage: "en_US",
		BaseURL:             metaServer.URL,
		HTTPClient:          metaServer.Client(),
	})

	rec := env.do(t, http.MethodPost, "/v1/otp/send", env.anonKey, OTPSendRequest{Phone: "+15551234567"})
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

// TestMessagesSend_PrefersCloudWhenConfigured is a regression test for a
// Cloud-only instance (no whatsmeow number ever paired): before this,
// POST /v1/messages/send always went through rt.WA regardless of rt.Cloud,
// so a Cloud-only instance's OTP sends worked but generic sends didn't —
// this asserts handleMessagesSend picks Cloud the same way handleOTPSend
// already does.
func TestMessagesSend_PrefersCloudWhenConfigured(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.cloud-generic"}},
		})
	}))
	defer metaServer.Close()

	env := newTestEnv(t)
	env.server.runtime().Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID: "test-phone-id",
		AccessToken:   "test-token",
		BaseURL:       metaServer.URL,
		HTTPClient:    metaServer.Client(),
	})

	rec := env.do(t, http.MethodPost, "/v1/messages/send", env.anonKey, SendMessageRequest{
		Phone: "+15551234567", Type: "text", Text: "your order shipped",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("messages send: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if gotPath != "/test-phone-id/messages" {
		t.Fatalf("expected the send to hit /test-phone-id/messages (Cloud), got %q — did it fall through to whatsmeow instead?", gotPath)
	}
	if gotBody["type"] != "text" {
		t.Fatalf("expected a plain text send, got type=%v", gotBody["type"])
	}
}

// TestMessagesSend_MediaKindRoutesToTheRightCloudType is a regression test
// for media parity: a "document" send must hit Cloud with type=document
// (and the filename), not silently fall back to the old image-only shape.
func TestMessagesSend_MediaKindRoutesToTheRightCloudType(t *testing.T) {
	var gotBody map[string]any
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.doc"}},
		})
	}))
	defer metaServer.Close()

	env := newTestEnv(t)
	env.server.runtime().Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID: "test-phone-id", AccessToken: "test-token",
		BaseURL: metaServer.URL, HTTPClient: metaServer.Client(),
	})

	rec := env.do(t, http.MethodPost, "/v1/messages/send", env.anonKey, SendMessageRequest{
		Phone: "+15551234567", Type: "document", URL: "https://example.com/report.pdf", Filename: "report.pdf",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("messages send: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotBody["type"] != "document" {
		t.Fatalf("type = %v, want document", gotBody["type"])
	}
	doc, _ := gotBody["document"].(map[string]any)
	if doc["filename"] != "report.pdf" {
		t.Fatalf("document.filename = %v, want report.pdf", doc["filename"])
	}
}

func TestMessagesSend_RejectsUnknownType(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/v1/messages/send", env.anonKey, SendMessageRequest{
		Phone: "+15551234567", Type: "carrier-pigeon",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unrecognized type", rec.Code)
	}
}

func TestMessagesSend_LocationRequiresCoordinates(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/v1/messages/send", env.anonKey, SendMessageRequest{
		Phone: "+15551234567", Type: "location",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when latitude/longitude are both zero", rec.Code)
	}
}

// TestMessagesSend_LocationRoutesThroughCloudAndStoresContent is a
// regression test for location parity: Cloud must receive the same
// type=location payload OTP/text/media already prefer it for, and the
// generic-message history must store something readable (the place name,
// falling back to raw coordinates) rather than an empty content field.
func TestMessagesSend_LocationRoutesThroughCloudAndStoresContent(t *testing.T) {
	var gotBody map[string]any
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.loc"}},
		})
	}))
	defer metaServer.Close()

	env := newTestEnv(t)
	env.server.runtime().Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID: "test-phone-id", AccessToken: "test-token",
		BaseURL: metaServer.URL, HTTPClient: metaServer.Client(),
	})

	rec := env.do(t, http.MethodPost, "/v1/messages/send", env.anonKey, SendMessageRequest{
		Phone: "+15551234567", Type: "location", Latitude: 33.5731, Longitude: -7.5898, Name: "Casablanca",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("messages send: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotBody["type"] != "location" {
		t.Fatalf("expected the send to hit Cloud with type=location, got %v", gotBody["type"])
	}

	msgs, err := env.server.runtime().Store.GetGenericMessages(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetGenericMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "Casablanca" || msgs[0].MessageType != "location" {
		t.Fatalf("stored message = %+v, want content=Casablanca type=location", msgs)
	}
}

// TestCloudStatus_ReflectsRuntimeState covers the cloud-status endpoint:
// disabled by default, and reporting connected once rt.Cloud is set and
// verified — the same rt.Cloud a real PATCH /v1/settings would populate
// (see registry_test.go for that wiring; this test injects rt.Cloud
// directly, same technique as the OTP send test above).
func TestCloudStatus_ReflectsRuntimeState(t *testing.T) {
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"id": "test-phone-id", "display_phone_number": "+1 555 0100",
		})
	}))
	defer metaServer.Close()

	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/dashboard/api/cloud-status", "", nil)
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

	rt := env.server.runtime()
	rt.Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
		PhoneNumberID: "test-phone-id",
		AccessToken:   "test-token",
		BaseURL:       metaServer.URL,
		HTTPClient:    metaServer.Client(),
	})
	if _, err := rt.Cloud.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// The service-tier API route must reflect the same state as the
	// unauthenticated dashboard one.
	rec = env.do(t, http.MethodGet, "/v1/cloud-status", env.serviceKey, nil)
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

// runtimeSettings reads back the instance's persisted settings directly
// from control.db, bypassing project.Load — which would build a real
// *whatsapp.CloudClient and try to verify it against the actual Meta Graph
// API over the network (slow, and pointless for a test that's only
// checking what got persisted).
func runtimeSettings(t *testing.T, env *testEnv) project.Settings {
	t.Helper()
	settingsJSON, err := env.control.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	settings := project.DefaultSettings()
	if settingsJSON != "" {
		if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
			t.Fatalf("unmarshal settings: %v", err)
		}
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

	rec := env.do(t, http.MethodPost, "/dashboard/api/cloud-settings", "", CloudSettingsRequest{
		Enabled:             true,
		PhoneNumberID:       "test-phone-id",
		AccessToken:         "test-token",
		OTPTemplateName:     "otp_verification",
		OTPTemplateLanguage: "en_US",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("configure cloud settings: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	settings := runtimeSettings(t, env)
	if !settings.Cloud.Enabled || settings.Cloud.PhoneNumberID != "test-phone-id" || settings.Cloud.OTPTemplateName != "otp_verification" {
		t.Fatalf("unexpected settings after enabling: %+v", settings.Cloud)
	}

	// Disable without resending the access token (the dashboard form never
	// gets it back from the status endpoint) — phone_number_id/template
	// fields must survive since the dashboard resubmits them from the
	// status it just displayed.
	rec = env.do(t, http.MethodPost, "/dashboard/api/cloud-settings", "", CloudSettingsRequest{
		Enabled:             false,
		PhoneNumberID:       settings.Cloud.PhoneNumberID,
		OTPTemplateName:     settings.Cloud.OTPTemplateName,
		OTPTemplateLanguage: settings.Cloud.OTPTemplateLanguage,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable cloud settings: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	settings = runtimeSettings(t, env)
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

// The "one number per instance" rule itself is covered at the source in
// core/internal/whatsapp/pool_test.go (TestPool_PairRefusesWhenAlreadyPaired)
// — Pair() needs a real paired device to trigger, which this package's
// httptest-based env can't fake without a network round-trip, so the HTTP
// wiring (errors.Is(err, whatsapp.ErrAlreadyPaired) -> 409 in startPairing)
// is covered by code review rather than an integration test here.

func TestRegenerateKey(t *testing.T) {
	env := newTestEnv(t)
	oldAnonKey := env.anonKey

	rec := env.do(t, http.MethodPost, "/v1/keys/regenerate", env.serviceKey, RegenerateKeyRequest{Tier: "anon"})
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

func TestSettings_UpdateIsPartialMergeAndTakesEffect(t *testing.T) {
	env := newTestEnv(t)

	rt1 := env.server.runtime()
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
	rec := env.do(t, http.MethodPatch, "/v1/settings", env.serviceKey, body)
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

	// The Runtime must have been reloaded in place with the new settings.
	rt2 := env.server.runtime()
	if rt2 == rt1 {
		t.Fatal("expected a fresh Runtime after settings update (old one should have been replaced)")
	}
	if rt2.Settings.Webhooks.Endpoint != "http://localhost:9999/hook" {
		t.Fatalf("reloaded runtime webhook endpoint = %q, want the patched value", rt2.Settings.Webhooks.Endpoint)
	}
}

func TestMetaWebhookVerify_HandshakeMatchesToken(t *testing.T) {
	env := newTestEnv(t)
	env.server.runtime().Settings.Cloud.VerifyToken = "my-verify-token"

	rec := env.do(t, http.MethodGet, "/webhooks/meta?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=xyz123", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct token: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "xyz123" {
		t.Fatalf("body = %q, want the echoed hub.challenge %q", rec.Body.String(), "xyz123")
	}

	rec = env.do(t, http.MethodGet, "/webhooks/meta?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=xyz123", "", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong token: status = %d, want 403", rec.Code)
	}
}

func TestMetaWebhookVerify_UnconfiguredTokenAlwaysFails(t *testing.T) {
	env := newTestEnv(t)
	// VerifyToken left empty (default) — must never succeed, even with an
	// empty token in the request, since that would trivially "match".
	rec := env.do(t, http.MethodGet, "/webhooks/meta?hub.mode=subscribe&hub.verify_token=&hub.challenge=xyz123", "", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 when no verify_token is configured", rec.Code)
	}
}

// postMetaWebhook signs body with secret (or leaves it unsigned if secret
// is "") and POSTs it to /webhooks/meta, mirroring exactly what Meta's own
// delivery does.
func postMetaWebhook(t *testing.T, env *testEnv, body []byte, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		h := hmac.New(sha256.New, []byte(secret))
		h.Write(body)
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(h.Sum(nil)))
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func TestMetaWebhookEvents_RejectsWithoutCloudConfigured(t *testing.T) {
	env := newTestEnv(t)
	// Neither rt.Cloud nor AppSecret set — the default state.
	rec := postMetaWebhook(t, env, []byte(`{"entry":[]}`), "irrelevant")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when cloud inbound isn't configured", rec.Code)
	}
}

func TestMetaWebhookEvents_SignatureVerificationGatesProcessing(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()
	rt.Settings.Cloud.AppSecret = "test-app-secret"
	rt.Cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{PhoneNumberID: "test-phone-id", AccessToken: "test-token"})

	body := []byte(`{
		"entry": [{
			"changes": [{
				"value": {
					"contacts": [{"profile": {"name": "Jane"}, "wa_id": "212600000000"}],
					"messages": [{"from": "212600000000", "id": "wamid.ABC", "timestamp": "1700000000", "type": "text", "text": {"body": "hello"}}]
				}
			}]
		}]
	}`)

	// Wrong secret — rejected, nothing pushed.
	rec := postMetaWebhook(t, env, body, "wrong-secret")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong signature: status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	select {
	case evt := <-rt.Cloud.Events():
		t.Fatalf("expected nothing pushed to Events() after a rejected signature, got %+v", evt)
	default:
	}

	// Correct secret — accepted, the parsed inbound event is pushed onto
	// rt.Cloud.Events() (read directly here rather than depending on the
	// background StartEventForwarder goroutine's timing).
	rec = postMetaWebhook(t, env, body, "test-app-secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("correct signature: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	select {
	case evt := <-rt.Cloud.Events():
		if evt.Type != whatsapp.EventMessageReceived {
			t.Fatalf("pushed event Type = %q, want %q", evt.Type, whatsapp.EventMessageReceived)
		}
		if evt.Phone != "212600000000" || evt.MessageID != "wamid.ABC" {
			t.Fatalf("pushed event = %+v, want phone 212600000000 / message id wamid.ABC", evt)
		}
	default:
		t.Fatal("expected the parsed inbound event to be pushed onto rt.Cloud.Events()")
	}
}

// TestHandleGetMedia_RoundTrip is a regression test for GET
// /v1/media/{message_id}: it must serve back exactly the bytes
// routeInboundMessage wrote to MediaDir, with the mimetype recorded on the
// InboundMessage row.
func TestHandleGetMedia_RoundTrip(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()

	conv, err := rt.Store.GetOrCreateConversation(context.Background(), "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	if err := rt.Store.InsertInboundMessage(context.Background(), &store.InboundMessage{
		ConversationID: conv.ID,
		Phone:          "212600000000",
		Content:        "look at this",
		MessageID:      "wamid.IMG1",
		MediaKind:      string(whatsapp.MediaKindImage),
		MediaMimeType:  "image/jpeg",
	}); err != nil {
		t.Fatalf("InsertInboundMessage: %v", err)
	}
	want := []byte("fake-jpeg-bytes")
	if err := os.WriteFile(filepath.Join(rt.MediaDir, "wamid.IMG1"), want, 0600); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/v1/media/wamid.IMG1", env.anonKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want %q", ct, "image/jpeg")
	}
	if rec.Body.String() != string(want) {
		t.Fatalf("body = %q, want %q", rec.Body.String(), want)
	}
}

// TestHandleGetMedia_NotFound covers both 404 cases: no such message at
// all, and a media message whose row exists but whose file never made it
// to disk (a failed download/write at receive time — documented, not a
// server error).
func TestHandleGetMedia_NotFound(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()

	rec := env.do(t, http.MethodGet, "/v1/media/does-not-exist", env.anonKey, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown message id: status = %d, want 404", rec.Code)
	}

	conv, err := rt.Store.GetOrCreateConversation(context.Background(), "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	if err := rt.Store.InsertInboundMessage(context.Background(), &store.InboundMessage{
		ConversationID: conv.ID,
		Phone:          "212600000000",
		MessageID:      "wamid.IMG2",
		MediaKind:      string(whatsapp.MediaKindImage),
		MediaMimeType:  "image/jpeg",
	}); err != nil {
		t.Fatalf("InsertInboundMessage: %v", err)
	}
	// No file written to MediaDir for wamid.IMG2 — simulates a download
	// failure at receive time.
	rec = env.do(t, http.MethodGet, "/v1/media/wamid.IMG2", env.anonKey, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("row without a file: status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}
