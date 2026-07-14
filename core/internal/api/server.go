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
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/templates"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// Server holds all dependencies for the API server.
type Server struct {
	config    *config.Config
	engine    *otp.Engine
	keyMgr    *keys.Manager
	waClient  whatsapp.Client
	templates *templates.Store
	wsHub     *ws.Hub
	logger    *slog.Logger
	startTime time.Time
	latestQR  string // latest QR code string for rendering
	qrMu      sync.RWMutex
}

// NewServer creates a new API server with all dependencies.
func NewServer(
	cfg *config.Config,
	otpEngine *otp.Engine,
	keyMgr *keys.Manager,
	waClient whatsapp.Client,
	tmplStore *templates.Store,
	wsHub *ws.Hub,
	logger *slog.Logger,
) *Server {
	return &Server{
		config:    cfg,
		engine:    otpEngine,
		keyMgr:    keyMgr,
		waClient:  waClient,
		templates: tmplStore,
		wsHub:     wsHub,
		logger:    logger,
		startTime: time.Now(),
	}
}

// SetLatestQR updates the latest QR code string for the /qr endpoint.
func (s *Server) SetLatestQR(qr string) {
	s.qrMu.Lock()
	s.latestQR = qr
	s.qrMu.Unlock()
}

// Router builds and returns the chi router with all routes and middleware.
func (s *Server) Router() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(s.loggerMiddleware)

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "apikey"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * 3600,
	}))

	// Public endpoints
	r.Get("/health", s.handleHealth)
	r.Get("/qr", s.handleQR)

	// WebSocket
	r.Get("/ws/events", s.handleWebSocket)

	// Dashboard
	if s.config.API.EnableDashboard {
		r.Get("/dashboard/api/messages", s.handleDashboardMessages)

		distFS, err := fs.Sub(dashboard.DistFS, "dist")
		if err == nil {
			fileServer := http.FileServer(http.FS(distFS))
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

	// Authenticated endpoints (anon or service key)
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware(keys.TierAnon, keys.TierService))
		r.Post("/otp/send", s.handleOTPSend)
		r.Post("/otp/verify", s.handleOTPVerify)
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
	status := "disconnected"
	phone := ""
	if s.waClient.IsConnected() {
		status = "connected"
		phone = s.waClient.GetPhoneNumber()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":         status,
		"phone":          phone,
		"uptime_seconds": int(time.Since(s.startTime).Seconds()),
	})
}

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	if s.waClient.IsConnected() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_connected"})
		return
	}

	s.qrMu.RLock()
	qr := s.latestQR
	s.qrMu.RUnlock()

	if qr == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no QR code available, waiting for WhatsApp..."})
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		writeJSON(w, http.StatusOK, map[string]string{"qr": qr})
		return
	}

	// Default: return PNG image
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

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.wsHub.HandleWS(w, r)
}

func (s *Server) handleDashboardMessages(w http.ResponseWriter, r *http.Request) {
	otps, err := s.engine.Store().GetRecentOTPs(r.Context(), 100)
	if err != nil {
		s.logger.Error("failed to get recent otps", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load history"})
		return
	}
	
	// If otps is nil, return empty array instead of null
	if otps == nil {
		otps = []store.OTPRequest{}
	}
	writeJSON(w, http.StatusOK, otps)
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

	ctx := r.Context()

	// Generate OTP
	result, err := s.engine.Send(ctx, req.Phone)
	if err != nil {
		if errors.Is(err, otp.ErrRateLimited) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limit_exceeded"})
			return
		}
		s.logger.Error("otp send error", "error", err, "phone", req.Phone)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate OTP"})
		return
	}

	// Render template
	locale := req.Locale
	if locale == "" {
		locale = s.config.Templates.DefaultLocale
	}
	message, err := s.templates.Render(locale, result.Code, s.config.OTP.ExpiryMinutes)
	if err != nil {
		s.logger.Error("template render error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to render message"})
		return
	}

	// Send via WhatsApp
	sendResult, err := s.waClient.SendMessage(ctx, req.Phone, message)
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
	if err := s.engine.MarkSent(ctx, result.Token, sendResult.MessageID); err != nil {
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

	ctx := r.Context()
	result, err := s.engine.Verify(ctx, req.Token, req.Code)
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
				"verified":            false,
				"error":               "invalid_code",
				"attempts_remaining":  result.AttemptsRemaining,
			})
		default:
			s.logger.Error("otp verify error", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "verification_failed"})
		}
		return
	}

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

// StartEventForwarder forwards WhatsApp events to the WebSocket hub.
// Run this in a goroutine.
func (s *Server) StartEventForwarder(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-s.waClient.Events():
			if !ok {
				return
			}
			s.wsHub.Broadcast(ws.Event{
				Type:      evt.Type,
				Phone:     evt.Phone,
				MessageID: evt.MessageID,
				Error:     evt.Error,
				At:        evt.At.Format(time.RFC3339),
			})
		}
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
