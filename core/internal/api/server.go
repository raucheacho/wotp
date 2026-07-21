package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/wotp/core/dashboard"
	"github.com/wotp/core/internal/config"
	"github.com/wotp/core/internal/keys"
	"github.com/wotp/core/internal/otp"
	"github.com/wotp/core/internal/project"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// Server holds all dependencies for the API server. wotp-core is
// mono-tenant: all per-instance state (store, OTP engine, WhatsApp pool,
// templates, webhooks, rate limiting) lives on the single project.Runtime
// held in rt, swapped in place by reloadRuntime when settings change.
type Server struct {
	config    *config.Config
	control   store.ControlStore
	keyMgr    *keys.Manager
	wsHub     *ws.Hub
	logger    *slog.Logger
	startTime time.Time

	// reload rebuilds the Runtime from current settings — set once at
	// startup by main.go (it closes over dataDir/templatesPath, which the
	// API layer has no other reason to know about).
	reload func(ctx context.Context) (*project.Runtime, error)

	rtMu sync.RWMutex
	rt   *project.Runtime
}

// NewServer creates a new API server with all dependencies. rt is the
// instance's initial Runtime (already loaded by the caller); reload rebuilds
// it from scratch after a settings change (see Server.reloadRuntime).
func NewServer(
	cfg *config.Config,
	control store.ControlStore,
	rt *project.Runtime,
	reload func(ctx context.Context) (*project.Runtime, error),
	keyMgr *keys.Manager,
	wsHub *ws.Hub,
	logger *slog.Logger,
) *Server {
	return &Server{
		config:    cfg,
		control:   control,
		rt:        rt,
		reload:    reload,
		keyMgr:    keyMgr,
		wsHub:     wsHub,
		logger:    logger,
		startTime: time.Now(),
	}
}

// runtime returns the current Runtime. Safe for concurrent use with
// reloadRuntime.
func (s *Server) runtime() *project.Runtime {
	s.rtMu.RLock()
	defer s.rtMu.RUnlock()
	return s.rt
}

// reloadRuntime rebuilds the Runtime (via s.reload) and swaps it in,
// closing the old one. Call after any change to persisted settings so it
// takes effect immediately, without a process restart — e.g. after editing
// the Cloud API config from the dashboard.
//
// The old Runtime's WhatsApp event channel is left open (Pool.Disconnect
// doesn't close it) — StartEventForwarder's goroutine for it just parks
// forever on an empty channel. Harmless (settings changes are rare, the
// leaked goroutine costs nothing but its stack) and far simpler than
// plumbing a cancellation signal through for this one case.
func (s *Server) reloadRuntime(ctx context.Context) (*project.Runtime, error) {
	newRt, err := s.reload(ctx)
	if err != nil {
		return nil, err
	}

	s.rtMu.Lock()
	old := s.rt
	s.rt = newRt
	s.rtMu.Unlock()

	old.Close()
	go s.StartEventForwarder(newRt)
	return newRt, nil
}

// Router builds and returns the chi router with all routes and middleware.
func (s *Server) Router() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(s.loggerMiddleware)

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "apikey"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * 3600,
	}))

	// Public endpoints
	r.Get("/v1/health", s.handleHealth)

	// WebSocket for real-time events — one instance, every connected client
	// sees every event.
	r.Get("/v1/ws/events", s.handleWebSocket)

	// Meta Cloud API inbound webhook — unauthenticated by necessity (Meta
	// can't send an apikey header); authenticity comes from VerifyToken on
	// the GET handshake and the X-Hub-Signature-256/AppSecret check on
	// every POST. See meta_webhook.go.
	r.Get("/webhooks/meta", s.handleMetaWebhookVerify)
	r.Post("/webhooks/meta", s.handleMetaWebhookEvents)

	// Dashboard — unauthenticated, "local-only by default" trust model (see
	// README security section).
	if s.config.API.EnableDashboard {
		r.Get("/dashboard/api/messages", s.handleDashboardMessages)
		r.Get("/dashboard/api/generic-messages", s.handleDashboardGenericMessages)
		r.Get("/dashboard/api/webhooks", s.handleDashboardWebhooks)
		r.Get("/dashboard/api/config", s.handleDashboardConfig)
		r.Get("/dashboard/api/numbers", s.handleDashboardNumbers)
		r.Post("/dashboard/api/numbers/pair", s.handleDashboardPairNumber)
		r.Get("/dashboard/api/numbers/qr", s.handleDashboardNumberQR)
		r.Get("/dashboard/api/cloud-status", s.handleDashboardCloudStatus)
		r.Post("/dashboard/api/cloud-settings", s.handleDashboardUpdateCloudSettings)

		distFS, err := fs.Sub(dashboard.DistFS, "dist")
		if err == nil {
			fileServer := http.FileServer(http.FS(distFS))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
			})
			r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
			})
			r.Get("/dashboard/*", func(w http.ResponseWriter, req *http.Request) {
				path := chi.URLParam(req, "*")
				if path == "" {
					req.URL.Path = "/"
				} else {
					req.URL.Path = "/" + path
				}

				// Check if the file exists in the filesystem
				f, err := distFS.Open(strings.TrimPrefix(req.URL.Path, "/"))
				if err != nil {
					// Fallback to index.html for React SPA
					req.URL.Path = "/"
				} else {
					f.Close()
				}
				fileServer.ServeHTTP(w, req)
			})
		} else {
			s.logger.Warn("dashboard enabled but dist not found in embedded fs")
		}
	}

	// Client-facing endpoints (anon or service key)
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware(keys.TierAnon, keys.TierService))
		r.Post("/v1/otp/send", s.handleOTPSend)
		r.Post("/v1/otp/verify", s.handleOTPVerify)
		r.Post("/v1/messages/send", s.handleMessagesSend)
		r.Post("/v1/messages/presence", s.handlePresence)
		r.Get("/v1/messages", s.handleGetMessages)
		r.Get("/v1/chats", s.handleChats)
		r.Get("/v1/conversations", s.handleListConversations)
		r.Get("/v1/conversations/{id}", s.handleGetConversation)
		r.Get("/v1/conversations/{id}/messages", s.handleGetConversationMessages)
		r.Post("/v1/conversations/{id}/takeover", s.handleTakeoverConversation)
		r.Post("/v1/conversations/{id}/resume", s.handleResumeConversation)
		r.Get("/v1/media/{message_id}", s.handleGetMedia)
	})

	// Instance admin endpoints (service key only) — number pairing, key
	// rotation, settings.
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware(keys.TierService))
		r.Post("/v1/numbers/pair", s.handleNumbersPair)
		r.Get("/v1/numbers", s.handleNumbersList)
		r.Get("/v1/numbers/qr", s.handleNumbersQR)
		r.Get("/v1/cloud-status", s.handleCloudStatusAPI)
		r.Post("/v1/keys/regenerate", s.handleRegenerateKey)
		r.Get("/v1/settings", s.handleGetSettings)
		r.Patch("/v1/settings", s.handleUpdateSettings)
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"uptime_seconds": int(time.Since(s.startTime).Seconds()),
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.wsHub.HandleWS(w, r)
}

func (s *Server) handleDashboardMessages(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	otps, err := rt.Store.GetRecentOTPs(r.Context(), 100)
	if err != nil {
		s.logger.Error("failed to get recent otps", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load history"})
		return
	}
	if otps == nil {
		otps = []store.OTPRequest{}
	}
	writeJSON(w, http.StatusOK, otps)
}

func (s *Server) handleDashboardGenericMessages(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	msgs, err := rt.Store.GetGenericMessages(r.Context(), 100)
	if err != nil {
		s.logger.Error("failed to get generic messages", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get messages"})
		return
	}
	if msgs == nil {
		msgs = []store.GenericMessage{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) handleDashboardWebhooks(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	logs, err := rt.Store.GetWebhookLogs(r.Context(), 100)
	if err != nil {
		s.logger.Error("failed to get webhook logs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load logs"})
		return
	}
	if logs == nil {
		logs = []store.WebhookLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleDashboardConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.config)
}

func (s *Server) handleDashboardNumbers(w http.ResponseWriter, r *http.Request) {
	s.listNumbers(w, r)
}

func (s *Server) handleDashboardCloudStatus(w http.ResponseWriter, r *http.Request) {
	s.cloudStatus(w, r)
}

func (s *Server) handleDashboardUpdateCloudSettings(w http.ResponseWriter, r *http.Request) {
	s.updateCloudSettings(w, r)
}

func (s *Server) handleDashboardPairNumber(w http.ResponseWriter, r *http.Request) {
	s.startPairing(w, r)
}

func (s *Server) handleDashboardNumberQR(w http.ResponseWriter, r *http.Request) {
	s.renderQR(w, r)
}

func (s *Server) handleNumbersPair(w http.ResponseWriter, r *http.Request) {
	s.startPairing(w, r)
}

func (s *Server) handleNumbersList(w http.ResponseWriter, r *http.Request) {
	s.listNumbers(w, r)
}

func (s *Server) handleNumbersQR(w http.ResponseWriter, r *http.Request) {
	s.renderQR(w, r)
}

func (s *Server) handleCloudStatusAPI(w http.ResponseWriter, r *http.Request) {
	s.cloudStatus(w, r)
}

// --- Numbers / pairing / Cloud config — shared by the service-tier
// /v1/numbers*, /v1/cloud-status routes above and the unauthenticated
// /dashboard/api/* routes, which trust project_id-free requests under the
// same "local-only by default" model. ---

// startPairing begins pairing a new WhatsApp number and returns
// immediately; QR codes stream out via Runtime.SetLatestQR (polled through
// renderQR) and the WS hub (event type "number.qr"). Once pairing finishes
// (success, timeout, or error), the control-plane numbers registry is
// synced so a newly linked number is reflected there.
func (s *Server) startPairing(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()

	// Pairing outlives this request by minutes (WhatsApp rotates the QR
	// roughly every 20s until scanned or it times out) — r.Context() would
	// get canceled the moment this handler returns, killing the QR-rotation
	// relay goroutine below after the very first code. Use a background
	// context bounded by whatsmeow's own pairing timeout instead.
	pairCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	qrChan, err := rt.WA.Pair(pairCtx)
	if err != nil {
		cancel()
		if errors.Is(err, whatsapp.ErrAlreadyPaired) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		s.logger.Error("failed to start pairing", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start pairing"})
		return
	}

	go func() {
		defer cancel()
		for qr := range qrChan {
			rt.SetLatestQR(qr)
			s.wsHub.Broadcast(ws.Event{
				Type:    "number.qr",
				Payload: qr,
			})
		}
		// The channel only closes once pairing is done one way or another
		// (success, timeout, error) — clear the QR so renderQR stops serving
		// a stale/expired code indefinitely (whatsmeow's own codes expire
		// after ~20-60s, but nothing here was telling callers that).
		rt.SetLatestQR("")
		s.wsHub.Broadcast(ws.Event{
			Type:    "number.qr",
			Payload: "",
		})

		syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer syncCancel()
		if err := project.SyncNumbers(syncCtx, s.control, rt.WA); err != nil {
			s.logger.Error("failed to sync numbers after pairing", "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "pairing_started"})
}

// listNumbers returns the live state of every number in the WhatsApp pool
// (not the control-plane numbers table — see startPairing).
func (s *Server) listNumbers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.runtime().WA.Numbers())
}

// CloudStatus reports whether the instance's Cloud API backend (see
// project.Settings.Cloud) is enabled, and if so whether its credentials
// have been verified — the dashboard shows this alongside the whatsmeow
// number, since an instance can use either or both (Cloud for OTP,
// whatsmeow for everything else).
type CloudStatus struct {
	Enabled             bool   `json:"enabled"`
	Connected           bool   `json:"connected"`
	PhoneNumberID       string `json:"phone_number_id,omitempty"`
	DisplayPhone        string `json:"display_phone,omitempty"`
	OTPTemplateName     string `json:"otp_template_name,omitempty"`
	OTPTemplateLanguage string `json:"otp_template_language,omitempty"`
	// WabaID/VerifyToken are not secret — returned as-is so the dashboard
	// form can round-trip them without the "leave blank to keep" dance
	// AccessToken/AppSecret/Pin need.
	WabaID string `json:"waba_id,omitempty"`
	// VerifyToken is echoed back so the operator can re-copy it into the
	// Meta app dashboard's webhook subscription form without having to
	// remember what they set here.
	VerifyToken string `json:"verify_token,omitempty"`
	// WebhookURL is this instance's inbound webhook endpoint, for pasting
	// into Meta's app dashboard — computed from the request, not stored.
	WebhookURL string `json:"webhook_url,omitempty"`
	// AccessToken/AppSecret/Pin are deliberately never returned — the
	// dashboard's edit form leaves them blank and only overwrites when the
	// operator types a new value (see updateCloudSettings).
}

// CloudSettingsRequest is the JSON body for the dashboard's Cloud API
// configuration form — a focused subset of project.Settings.Cloud, so the
// dashboard (unauthenticated, local-only trust model) can only touch Cloud
// config through this endpoint, not the rest of the instance's settings.
type CloudSettingsRequest struct {
	Enabled             bool   `json:"enabled"`
	PhoneNumberID       string `json:"phone_number_id"`
	AccessToken         string `json:"access_token"`
	OTPTemplateName     string `json:"otp_template_name"`
	OTPTemplateLanguage string `json:"otp_template_language"`
	WabaID              string `json:"waba_id"`
	Pin                 string `json:"pin"`
	AppSecret           string `json:"app_secret"`
	VerifyToken         string `json:"verify_token"`
}

// updateCloudSettings lets the dashboard configure the Cloud API backend
// without a service key — same trust model as pairing a whatsmeow number
// from the dashboard already has (unauthenticated, local-only by default).
func (s *Server) updateCloudSettings(w http.ResponseWriter, r *http.Request) {
	var req CloudSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	ctx := r.Context()
	settings := s.runtime().Settings

	settings.Cloud.Enabled = req.Enabled
	settings.Cloud.PhoneNumberID = req.PhoneNumberID
	// An empty secret-like field means "keep the existing one" — the
	// status endpoint never returns these, so the dashboard form can't
	// round-trip them; only overwrite when the operator actually typed a
	// new value.
	if req.AccessToken != "" {
		settings.Cloud.AccessToken = req.AccessToken
	}
	if req.Pin != "" {
		settings.Cloud.Pin = req.Pin
	}
	if req.AppSecret != "" {
		settings.Cloud.AppSecret = req.AppSecret
	}
	settings.Cloud.OTPTemplateName = req.OTPTemplateName
	settings.Cloud.OTPTemplateLanguage = req.OTPTemplateLanguage
	settings.Cloud.WabaID = req.WabaID
	settings.Cloud.VerifyToken = req.VerifyToken

	if err := s.saveSettingsAndReload(ctx, settings); err != nil {
		s.logger.Error("failed to save cloud settings", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) cloudStatus(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	status := CloudStatus{
		Enabled:     rt.Cloud != nil,
		WabaID:      rt.Settings.Cloud.WabaID,
		VerifyToken: rt.Settings.Cloud.VerifyToken,
		WebhookURL:  webhookURLFromRequest(r),
	}
	if rt.Cloud != nil {
		status.Connected = rt.Cloud.IsConnected()
		status.PhoneNumberID = rt.Settings.Cloud.PhoneNumberID
		status.DisplayPhone = rt.Cloud.GetPhoneNumber()
		status.OTPTemplateName = rt.Settings.Cloud.OTPTemplateName
		status.OTPTemplateLanguage = rt.Settings.Cloud.OTPTemplateLanguage
	}
	writeJSON(w, http.StatusOK, status)
}

// webhookURLFromRequest builds the absolute URL of the inbound Meta webhook
// receiver from the incoming request's own host — so the dashboard can show
// the exact URL to paste into Meta's app console without wotp needing to
// know its own public hostname/scheme ahead of time (it's whatever the
// operator is reaching the dashboard through, which is also what Meta needs
// to reach, once that's exposed publicly).
func webhookURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/webhooks/meta"
}

func (s *Server) renderQR(w http.ResponseWriter, r *http.Request) {
	qr := s.runtime().LatestQR()
	if qr == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no QR code available, waiting for WhatsApp..."})
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		writeJSON(w, http.StatusOK, map[string]string{"qr": qr})
		return
	}

	png, err := qrcode.Encode(qr, qrcode.Medium, 512)
	if err != nil {
		s.logger.Error("qr encode error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate QR image"})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	w.Write(png)
}

// RegenerateKeyRequest is the JSON body for POST /v1/keys/regenerate.
type RegenerateKeyRequest struct {
	Tier string `json:"tier"` // "anon" or "service"
}

func (s *Server) handleRegenerateKey(w http.ResponseWriter, r *http.Request) {
	var req RegenerateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Tier != keys.TierAnon && req.Tier != keys.TierService) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tier must be \"anon\" or \"service\""})
		return
	}

	newKey, err := s.keyMgr.RegenerateAll(r.Context(), req.Tier)
	if err != nil {
		s.logger.Error("failed to regenerate key", "tier", req.Tier, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to regenerate key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"key": newKey.FullKey, "tier": newKey.Tier})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.runtime().Settings)
}

// handleUpdateSettings applies a partial update to the instance's settings:
// JSON keys present in the request body overwrite the corresponding
// existing values (encoding/json merges into an existing struct
// field-by-field), anything omitted is left untouched. The Runtime is
// reloaded so the new values take effect immediately, without disconnecting
// an already-paired WhatsApp number (whatsmeow reconnects via
// Pool.LoadExisting during the reload).
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	settings := s.runtime().Settings

	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid settings payload"})
		return
	}

	if err := s.saveSettingsAndReload(r.Context(), settings); err != nil {
		s.logger.Error("failed to save settings", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) saveSettingsAndReload(ctx context.Context, settings project.Settings) error {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := s.control.UpdateSettings(ctx, string(settingsJSON)); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if _, err := s.reloadRuntime(ctx); err != nil {
		return fmt.Errorf("reload runtime: %w", err)
	}
	return nil
}

// OTPSendRequest is the JSON body for POST /otp/send.
type OTPSendRequest struct {
	Phone  string `json:"phone"`
	Locale string `json:"locale,omitempty"`
}

func (s *Server) handleOTPSend(w http.ResponseWriter, r *http.Request) {
	var req OTPSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone is required"})
		return
	}

	rt := s.runtime()
	ctx := r.Context()

	// Generate OTP
	result, err := rt.Engine.Send(ctx, req.Phone)
	if err != nil {
		if errors.Is(err, otp.ErrRateLimited) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limit_exceeded"})
			return
		}
		s.logger.Error("otp send error", "error", err, "phone", req.Phone)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate OTP"})
		return
	}

	// Send via WhatsApp. A Cloud API-backed instance must send OTPs through
	// a pre-approved template (see whatsapp.TemplateOTPSender) — it can
	// never use free text, since an OTP is always the first message in a
	// conversation and so always outside the 24h window free text needs.
	// whatsmeow has no such restriction, so it keeps using the locale
	// template rendered to plain text.
	var sendResult *whatsapp.SendResult
	if rt.Cloud != nil {
		sendResult, err = rt.Cloud.SendOTPTemplate(ctx, req.Phone, result.Code, rt.Settings.OTP.ExpiryMinutes)
	} else {
		locale := req.Locale
		if locale == "" {
			locale = rt.Settings.Templates.DefaultLocale
		}
		var message string
		message, err = rt.Templates.Render(locale, result.Code, rt.Settings.OTP.ExpiryMinutes)
		if err != nil {
			s.logger.Error("template render error", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to render message"})
			return
		}
		sendResult, err = rt.WA.SendMessage(ctx, req.Phone, message)
	}
	if err != nil {
		s.logger.Error("whatsapp send error", "error", err, "phone", req.Phone)
		// Still return the token — the OTP is created, just failed to send
		writeJSON(w, http.StatusOK, map[string]string{
			"token":      result.Token,
			"expires_at": result.ExpiresAt.Format(time.RFC3339),
			"warning":    "message_send_failed",
		})
		return
	}

	// Mark as sent with message ID
	if err := rt.Engine.MarkSent(ctx, result.Token, sendResult.MessageID); err != nil {
		s.logger.Error("mark sent error", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":      result.Token,
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
	})
}

// OTPVerifyRequest is the JSON body for POST /otp/verify.
type OTPVerifyRequest struct {
	Token string `json:"token"`
	Code  string `json:"code"`
}

func (s *Server) handleOTPVerify(w http.ResponseWriter, r *http.Request) {
	var req OTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token and code are required"})
		return
	}

	rt := s.runtime()
	ctx := r.Context()
	result, err := rt.Engine.Verify(ctx, req.Token, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, otp.ErrTokenNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "token_not_found"})
		case errors.Is(err, otp.ErrTokenExpired):
			writeJSON(w, http.StatusGone, map[string]string{"error": "token_expired"})
		case errors.Is(err, otp.ErrMaxAttempts):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "max_attempts_exceeded"})
		case errors.Is(err, otp.ErrAlreadyVerified):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "already_verified"})
		case errors.Is(err, otp.ErrInvalidCode):
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"verified":           false,
				"error":              "invalid_code",
				"attempts_remaining": result.AttemptsRemaining,
			})
		default:
			s.logger.Error("otp verify error", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "verification_failed"})
		}
		return
	}

	// Broadcast verification success to dashboard clients
	s.wsHub.Broadcast(ws.Event{
		Type:      "otp.verified",
		Phone:     result.Phone,
		MessageID: result.MessageID,
		At:        time.Now().Format(time.RFC3339),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"verified": true,
		"phone":    result.Phone,
	})
}

// --- Middleware ---

func (s *Server) authMiddleware(allowedTiers ...string) func(http.Handler) http.Handler {
	tierSet := make(map[string]bool)
	for _, t := range allowedTiers {
		tierSet[t] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("apikey")
			if apiKey == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing apikey header"})
				return
			}

			tier, err := s.keyMgr.Validate(r.Context(), apiKey)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
				return
			}

			if !tierSet[tier] {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		s.logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", r.RemoteAddr,
		)
	})
}

// StartEventForwarder forwards a Runtime's WhatsApp events — from both the
// whatsmeow pool and, when configured, the Cloud API client — to its
// webhook dispatcher and to dashboard clients over the WS hub. Called once
// at startup for the initial Runtime, and again by reloadRuntime for each
// replacement.
//
// Fans in both event channels rather than ranging over just rt.WA.Events()
// — before this, anything CloudClient emitted (message.sent/failed on every
// Cloud-routed send, and eventually inbound/delivery events once Cloud
// webhooks are wired up) was silently dropped: nothing ever read it.
func (s *Server) StartEventForwarder(rt *project.Runtime) {
	waEvents := rt.WA.Events()
	var cloudEvents <-chan whatsapp.Event
	if rt.Cloud != nil {
		cloudEvents = rt.Cloud.Events()
	}

	for {
		var evt whatsapp.Event
		var ok bool
		select {
		case evt, ok = <-waEvents:
			if !ok {
				waEvents = nil
				continue
			}
		case evt, ok = <-cloudEvents:
			if !ok {
				cloudEvents = nil
				continue
			}
		}
		s.forwardEvent(rt, evt)
	}
}

func (s *Server) forwardEvent(rt *project.Runtime, evt whatsapp.Event) {
	if evt.Type == whatsapp.EventMessageReceived {
		// Persist the message to its conversation and enrich the event
		// with conversation_id/conversation_state before forwarding —
		// wotp always forwards; it's up to the receiving app's own bot
		// logic to skip replying when the conversation is human-owned.
		// See routeInboundMessage in conversations.go.
		enriched, err := routeInboundMessage(context.Background(), rt.Store, rt.MediaDir, evt)
		if err != nil {
			s.logger.Error("failed to route inbound message", "phone", evt.Phone, "error", err)
		}
		evt = enriched
	}
	rt.Webhooks.ProcessEvent(evt)

	// Update DB status for OTP and Generic Messages
	if evt.MessageID != "" {
		var status string
		switch evt.Type {
		case "message.sent":
			status = string(store.StatusSent)
		case "message.delivered":
			status = string(store.StatusDelivered)
		case "message.read":
			status = string(store.StatusRead)
		case "message.failed":
			status = string(store.StatusFailed)
		}

		if status != "" {
			_ = rt.Store.UpdateOTPStatusByMessageID(context.Background(), evt.MessageID, store.OTPStatus(status))
			_ = rt.Store.UpdateGenericMessageStatus(context.Background(), evt.MessageID, status, evt.Error)
		}
	}

	s.wsHub.Broadcast(ws.Event{
		Type:      evt.Type,
		Phone:     evt.Phone,
		MessageID: evt.MessageID,
		Error:     evt.Error,
		From:      evt.From,
		At:        evt.At.Format(time.RFC3339),
	})
}

// ListenAndServe starts the HTTP server on the configured port.
func (s *Server) ListenAndServe() *http.Server {
	router := s.Router()
	addr := fmt.Sprintf(":%d", s.config.API.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		s.logger.Info("api server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("api server error", "error", err)
		}
	}()

	return srv
}
