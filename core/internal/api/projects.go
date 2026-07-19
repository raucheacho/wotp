package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/wotp/core/internal/keys"
	"github.com/wotp/core/internal/project"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// CreateProjectRequest is the JSON body for POST /v1/projects.
type CreateProjectRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slug is required"})
		return
	}
	if req.Name == "" {
		req.Name = req.Slug
	}

	ctx := r.Context()
	p, err := s.registry.Create(ctx, req.Slug, req.Name)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	anonKey, serviceKey, err := s.keyMgr.EnsureKeys(ctx, p.ID)
	if err != nil {
		s.logger.Error("failed to generate keys for new project", "project_id", p.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "project created but key generation failed"})
		return
	}

	resp := map[string]any{"project": p}
	if anonKey != nil {
		resp["anon_key"] = anonKey.FullKey
	}
	if serviceKey != nil {
		resp["service_key"] = serviceKey.FullKey
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.registry.Delete(r.Context(), id); err != nil {
		s.logger.Error("failed to delete project", "project_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete project"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePairNumber(w http.ResponseWriter, r *http.Request) {
	s.startPairing(w, r, chi.URLParam(r, "id"))
}

func (s *Server) handleListNumbers(w http.ResponseWriter, r *http.Request) {
	s.listNumbers(w, r, chi.URLParam(r, "id"))
}

func (s *Server) handleNumberQR(w http.ResponseWriter, r *http.Request) {
	s.renderQR(w, r, chi.URLParam(r, "id"))
}

func (s *Server) handleCloudStatus(w http.ResponseWriter, r *http.Request) {
	s.cloudStatus(w, r, chi.URLParam(r, "id"))
}

// --- shared helpers used by both the root-tier /v1/projects* routes above
// and the unauthenticated /dashboard/api/* routes in server.go ---

// startPairing begins pairing a new WhatsApp number for projectID and
// returns immediately; QR codes stream out via Runtime.SetLatestQR (polled
// through renderQR) and the WS hub (event type "number.qr"). Once pairing
// finishes (success, timeout, or error), the control-plane numbers
// registry is synced so a newly linked number is reflected there — see
// Registry.SyncNumbers.
func (s *Server) startPairing(w http.ResponseWriter, r *http.Request, projectID string) {
	rt, err := s.registry.Get(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

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
		s.logger.Error("failed to start pairing", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start pairing"})
		return
	}

	go func() {
		defer cancel()
		for qr := range qrChan {
			rt.SetLatestQR(qr)
			s.wsHub.Broadcast(ws.Event{
				Type:      "number.qr",
				ProjectID: rt.Project.ID,
				Payload:   qr,
			})
		}
		// The channel only closes once pairing is done one way or another
		// (success, timeout, error) — clear the QR so renderQR stops serving
		// a stale/expired code indefinitely (whatsmeow's own codes expire
		// after ~20-60s, but nothing here was telling callers that).
		rt.SetLatestQR("")
		s.wsHub.Broadcast(ws.Event{
			Type:      "number.qr",
			ProjectID: rt.Project.ID,
			Payload:   "",
		})

		syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer syncCancel()
		if err := s.registry.SyncNumbers(syncCtx, projectID); err != nil {
			s.logger.Error("failed to sync numbers after pairing", "project_id", projectID, "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "pairing_started"})
}

// listNumbers returns the live state of every number in a project's
// WhatsApp pool (not the control-plane numbers table — see startPairing).
func (s *Server) listNumbers(w http.ResponseWriter, r *http.Request, projectID string) {
	rt, err := s.registry.Get(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	writeJSON(w, http.StatusOK, rt.WA.Numbers())
}

// CloudStatus reports whether a project's Cloud API backend (see
// project.Settings.Cloud) is enabled, and if so whether its credentials
// have been verified — the dashboard shows this alongside the whatsmeow
// number, since a project can use either or both (Cloud for OTP, whatsmeow
// for everything else).
type CloudStatus struct {
	Enabled             bool   `json:"enabled"`
	Connected           bool   `json:"connected"`
	PhoneNumberID       string `json:"phone_number_id,omitempty"`
	DisplayPhone        string `json:"display_phone,omitempty"`
	OTPTemplateName     string `json:"otp_template_name,omitempty"`
	OTPTemplateLanguage string `json:"otp_template_language,omitempty"`
	// AccessToken is deliberately never returned — the dashboard's edit
	// form leaves it blank and only overwrites it when the operator types
	// a new value (see updateCloudSettings).
}

// CloudSettingsRequest is the JSON body for the dashboard's Cloud API
// configuration form — a focused subset of project.Settings.Cloud, so the
// dashboard (unauthenticated, local-only trust model) can only touch Cloud
// config through this endpoint, not the rest of a project's settings.
type CloudSettingsRequest struct {
	Enabled             bool   `json:"enabled"`
	PhoneNumberID       string `json:"phone_number_id"`
	AccessToken         string `json:"access_token"`
	OTPTemplateName     string `json:"otp_template_name"`
	OTPTemplateLanguage string `json:"otp_template_language"`
}

// updateCloudSettings lets the dashboard configure a project's Cloud API
// backend without needing the root key that PATCH /v1/projects/{id}/settings
// requires — same trust model as pairing a whatsmeow number from the
// dashboard already has (unauthenticated, scoped by project_id).
func (s *Server) updateCloudSettings(w http.ResponseWriter, r *http.Request, projectID string) {
	var req CloudSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	ctx := r.Context()
	p, settings, err := s.loadSettings(ctx, projectID)
	if err != nil {
		s.logger.Error("failed to load project settings", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	settings.Cloud.Enabled = req.Enabled
	settings.Cloud.PhoneNumberID = req.PhoneNumberID
	// An empty AccessToken means "keep the existing one" — the status
	// endpoint never returns it, so the dashboard form can't round-trip it;
	// only overwrite when the operator actually typed a new value.
	if req.AccessToken != "" {
		settings.Cloud.AccessToken = req.AccessToken
	}
	settings.Cloud.OTPTemplateName = req.OTPTemplateName
	settings.Cloud.OTPTemplateLanguage = req.OTPTemplateLanguage

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode settings"})
		return
	}
	if err := s.control.UpdateProjectSettings(ctx, projectID, string(settingsJSON)); err != nil {
		s.logger.Error("failed to save project settings", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
		return
	}
	s.registry.Evict(projectID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) cloudStatus(w http.ResponseWriter, r *http.Request, projectID string) {
	rt, err := s.registry.Get(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	status := CloudStatus{Enabled: rt.Cloud != nil}
	if rt.Cloud != nil {
		status.Connected = rt.Cloud.IsConnected()
		status.PhoneNumberID = rt.Settings.Cloud.PhoneNumberID
		status.DisplayPhone = rt.Cloud.GetPhoneNumber()
		status.OTPTemplateName = rt.Settings.Cloud.OTPTemplateName
		status.OTPTemplateLanguage = rt.Settings.Cloud.OTPTemplateLanguage
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) renderQR(w http.ResponseWriter, r *http.Request, projectID string) {
	rt, err := s.registry.Get(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	qr := rt.LatestQR()
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

// RegenerateKeyRequest is the JSON body for POST /v1/projects/{id}/keys/regenerate.
type RegenerateKeyRequest struct {
	Tier string `json:"tier"` // "anon" or "service"
}

func (s *Server) handleRegenerateProjectKey(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	var req RegenerateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Tier != keys.TierAnon && req.Tier != keys.TierService) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tier must be \"anon\" or \"service\""})
		return
	}

	if p, err := s.control.GetProjectByID(r.Context(), projectID); err != nil || p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	newKey, err := s.keyMgr.RegenerateAll(r.Context(), projectID, req.Tier)
	if err != nil {
		s.logger.Error("failed to regenerate key", "project_id", projectID, "tier", req.Tier, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to regenerate key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"key": newKey.FullKey, "tier": newKey.Tier})
}

func (s *Server) loadSettings(ctx context.Context, projectID string) (*store.Project, project.Settings, error) {
	p, err := s.control.GetProjectByID(ctx, projectID)
	if err != nil || p == nil {
		return nil, project.Settings{}, err
	}
	settings := project.DefaultSettings()
	if p.SettingsJSON != "" {
		if err := json.Unmarshal([]byte(p.SettingsJSON), &settings); err != nil {
			return nil, project.Settings{}, err
		}
	}
	return p, settings, nil
}

func (s *Server) handleGetProjectSettings(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	p, settings, err := s.loadSettings(r.Context(), projectID)
	if err != nil {
		s.logger.Error("failed to load project settings", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// handleUpdateProjectSettings applies a partial update to a project's
// settings: JSON keys present in the request body overwrite the
// corresponding existing values (encoding/json merges into an existing
// struct field-by-field), anything omitted is left untouched. The
// project's cached Runtime is evicted so the new values take effect on
// next access, without disconnecting already-paired WhatsApp numbers.
func (s *Server) handleUpdateProjectSettings(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	p, settings, err := s.loadSettings(r.Context(), projectID)
	if err != nil {
		s.logger.Error("failed to load project settings", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid settings payload"})
		return
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode settings"})
		return
	}

	if err := s.control.UpdateProjectSettings(r.Context(), projectID, string(settingsJSON)); err != nil {
		s.logger.Error("failed to save project settings", "project_id", projectID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
		return
	}

	s.registry.Evict(projectID)

	writeJSON(w, http.StatusOK, settings)
}
