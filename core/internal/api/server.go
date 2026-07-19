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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/wotp/core/dashboard"
	"github.com/wotp/core/internal/config"
	"github.com/wotp/core/internal/keys"
	"github.com/wotp/core/internal/otp"
	"github.com/wotp/core/internal/project"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// ctxKey namespaces values stored on the request context by authMiddleware.
type ctxKey int

const (
	ctxKeyRuntime ctxKey = iota
	ctxKeyTier
)

// runtimeFromContext returns the project.Runtime resolved by authMiddleware
// for anon/service-tier requests. nil for root-tier requests, which aren't
// scoped to a single project.
func runtimeFromContext(ctx context.Context) *project.Runtime {
	rt, _ := ctx.Value(ctxKeyRuntime).(*project.Runtime)
	return rt
}

// Server holds all dependencies for the API server. Per-project state
// (store, OTP engine, WhatsApp pool, templates, webhooks, rate limiting)
// lives on project.Runtime instead, resolved per-request via the registry.
type Server struct {
	config    *config.Config
	control   store.ControlStore
	registry  *project.Registry
	keyMgr    *keys.Manager
	wsHub     *ws.Hub
	logger    *slog.Logger
	startTime time.Time
}

// NewServer creates a new API server with all dependencies.
func NewServer(
	cfg *config.Config,
	control store.ControlStore,
	registry *project.Registry,
	keyMgr *keys.Manager,
	wsHub *ws.Hub,
	logger *slog.Logger,
) *Server {
	return &Server{
		config:    cfg,
		control:   control,
		registry:  registry,
		keyMgr:    keyMgr,
		wsHub:     wsHub,
		logger:    logger,
		startTime: time.Now(),
	}
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

	// WebSocket for real-time events. Optionally scoped to a project via
	// ?apikey=... (browsers can't set custom headers on a WS handshake).
	r.Get("/v1/ws/events", s.handleWebSocket)

	// Dashboard — unauthenticated, "local-only by default" trust model (see
	// README security section). Requests are scoped by a project_id query
	// parameter rather than an apikey.
	if s.config.API.EnableDashboard {
		r.Get("/dashboard/api/projects", s.handleDashboardProjects)
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

	// Project-scoped endpoints (anon or service key)
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware(keys.TierAnon, keys.TierService))
		r.Post("/v1/otp/send", s.handleOTPSend)
		r.Post("/v1/otp/verify", s.handleOTPVerify)
		r.Post("/v1/messages/send", s.handleMessagesSend)
		r.Post("/v1/messages/presence", s.handlePresence)
		r.Get("/v1/messages", s.handleGetMessages)
		r.Get("/v1/chats", s.handleChats)
	})

	// Instance-admin endpoints (root key only) — project lifecycle and
	// number pairing, not scoped to any single project.
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware(keys.TierRoot))
		r.Post("/v1/projects", s.handleCreateProject)
		r.Get("/v1/projects", s.handleListProjects)
		r.Delete("/v1/projects/{id}", s.handleDeleteProject)
		r.Post("/v1/projects/{id}/numbers/pair", s.handlePairNumber)
		r.Get("/v1/projects/{id}/numbers", s.handleListNumbers)
		r.Get("/v1/projects/{id}/numbers/qr", s.handleNumberQR)
		r.Get("/v1/projects/{id}/cloud-status", s.handleCloudStatus)
		r.Post("/v1/projects/{id}/keys/regenerate", s.handleRegenerateProjectKey)
		r.Get("/v1/projects/{id}/settings", s.handleGetProjectSettings)
		r.Patch("/v1/projects/{id}/settings", s.handleUpdateProjectSettings)
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
	// project_id is trusted directly here (unauthenticated dashboard, same
	// trust model as the /dashboard/api/* routes); apikey is for external
	// API consumers, who can't set custom headers on a WS handshake.
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		if apiKey := r.URL.Query().Get("apikey"); apiKey != "" {
			if pid, _, err := s.keyMgr.Validate(r.Context(), apiKey); err == nil {
				projectID = pid
			}
		}
	}
	s.wsHub.HandleWS(w, r, projectID)
}

// dashboardRuntime resolves the project.Runtime for a dashboard request from
// its project_id query parameter, writing an error response and returning
// false if it's missing or the project can't be loaded.
func (s *Server) dashboardRuntime(w http.ResponseWriter, r *http.Request) (*project.Runtime, bool) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id query parameter is required"})
		return nil, false
	}
	rt, err := s.registry.Get(r.Context(), projectID)
	if err != nil {
		s.logger.Error("dashboard: failed to load project", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return nil, false
	}
	return rt, true
}

func (s *Server) handleDashboardProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.control.ListProjects(r.Context())
	if err != nil {
		s.logger.Error("failed to list projects", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleDashboardMessages(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.dashboardRuntime(w, r)
	if !ok {
		return
	}
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
	rt, ok := s.dashboardRuntime(w, r)
	if !ok {
		return
	}
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
	rt, ok := s.dashboardRuntime(w, r)
	if !ok {
		return
	}
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
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id query parameter is required"})
		return
	}
	s.listNumbers(w, r, projectID)
}

func (s *Server) handleDashboardCloudStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id query parameter is required"})
		return
	}
	s.cloudStatus(w, r, projectID)
}

func (s *Server) handleDashboardUpdateCloudSettings(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id query parameter is required"})
		return
	}
	s.updateCloudSettings(w, r, projectID)
}

func (s *Server) handleDashboardPairNumber(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id query parameter is required"})
		return
	}
	s.startPairing(w, r, projectID)
}

func (s *Server) handleDashboardNumberQR(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id query parameter is required"})
		return
	}
	s.renderQR(w, r, projectID)
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

	rt := runtimeFromContext(r.Context())
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

	// Send via WhatsApp. A Cloud API-backed project must send OTPs through
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

	rt := runtimeFromContext(r.Context())
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

	// Broadcast verification success to this project's dashboard clients
	s.wsHub.Broadcast(ws.Event{
		Type:      "otp.verified",
		ProjectID: rt.Project.ID,
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

			projectID, tier, err := s.keyMgr.Validate(r.Context(), apiKey)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
				return
			}

			if !tierSet[tier] {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyTier, tier)
			if tier != keys.TierRoot {
				rt, err := s.registry.Get(ctx, projectID)
				if err != nil {
					s.logger.Error("failed to load project runtime", "error", err, "project_id", projectID)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "project unavailable"})
					return
				}
				ctx = context.WithValue(ctx, ctxKeyRuntime, rt)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
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

// StartEventForwarder forwards a single project's WhatsApp events to its
// webhook dispatcher and to this project's dashboard clients over the WS
// hub. Registered via project.Registry.SetOnRuntimeLoaded, so it runs once
// per project, in its own goroutine, from the moment that project's Runtime
// is first built.
func (s *Server) StartEventForwarder(rt *project.Runtime) {
	for evt := range rt.WA.Events() {
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
			ProjectID: rt.Project.ID,
			Phone:     evt.Phone,
			MessageID: evt.MessageID,
			Error:     evt.Error,
			From:      evt.From,
			At:        evt.At.Format(time.RFC3339),
		})
	}
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
