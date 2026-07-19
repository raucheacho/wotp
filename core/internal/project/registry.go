package project

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/wotp/core/internal/otp"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/templates"
	"github.com/wotp/core/internal/webhooks"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// defaultReconnectBackoff is the WhatsApp reconnect delay schedule (seconds)
// applied to every project's numbers. It's an operational tuning knob, not a
// per-tenant business setting, so it isn't part of Settings.
var defaultReconnectBackoff = []int{5, 15, 60, 300}

// Runtime bundles everything needed to serve requests for a single project:
// its own data store, OTP engine, message templates, webhook dispatcher, and
// WhatsApp number pool.
type Runtime struct {
	Project   *store.Project
	Settings  Settings
	Store     store.ProjectStore
	Engine    *otp.Engine
	Templates *templates.Store
	Webhooks  *webhooks.Service
	WA        *whatsapp.Pool

	// Cloud is set only when the instance is configured with a Meta Cloud
	// API backend (see Registry.SetCloudConfig) — nil for every project
	// otherwise. Handlers that can use either backend (currently just OTP
	// sending) prefer Cloud when non-nil and fall back to WA/whatsmeow.
	Cloud *whatsapp.CloudClient

	qrMu     sync.RWMutex
	latestQR string

	msgMu     sync.Mutex
	msgCount  int
	msgWindow time.Time
}

// SetLatestQR records the most recent pairing QR code for this project, so
// it can be displayed even if the caller isn't the one that started pairing.
func (rt *Runtime) SetLatestQR(qr string) {
	rt.qrMu.Lock()
	rt.latestQR = qr
	rt.qrMu.Unlock()
}

// LatestQR returns the most recently issued pairing QR code, or "" if none.
func (rt *Runtime) LatestQR() string {
	rt.qrMu.RLock()
	defer rt.qrMu.RUnlock()
	return rt.latestQR
}

// AllowSend enforces this project's per-minute outbound message rate limit.
// Returns false if the limit (Settings.Messaging.MaxMessagesPerMinute, 0 = unlimited)
// has been exceeded for the current one-minute window.
func (rt *Runtime) AllowSend() bool {
	rt.msgMu.Lock()
	defer rt.msgMu.Unlock()

	now := time.Now()
	if now.Sub(rt.msgWindow) >= time.Minute {
		rt.msgWindow = now
		rt.msgCount = 0
	}
	rt.msgCount++
	limit := rt.Settings.Messaging.MaxMessagesPerMinute
	return limit <= 0 || rt.msgCount <= limit
}

// Registry lazily opens and caches a Runtime per project. There is exactly
// one Registry per wotp-core instance.
type Registry struct {
	control       store.ControlStore
	dataDir       string
	templatesPath string
	wsHub         *ws.Hub
	logger        *slog.Logger

	// onRuntimeLoaded, if set, is called once (in its own goroutine) right
	// after a Runtime is first built — the API server uses this to start
	// forwarding the project's WhatsApp events to webhooks/the WS hub
	// without this package needing to know about the API layer.
	onRuntimeLoaded func(*Runtime)

	// cloudBaseURLOverride/cloudHTTPClientOverride let tests in this package
	// point a project's Cloud client at an httptest.Server instead of the
	// real Meta Graph API. Never set outside tests — production always
	// leaves these zero, which whatsapp.NewCloudClient treats as "use the
	// real endpoint / http.DefaultClient".
	cloudBaseURLOverride    string
	cloudHTTPClientOverride *http.Client

	mu       sync.RWMutex
	runtimes map[string]*Runtime
}

// NewRegistry creates a Registry. templatesPath points at the instance's
// shared templates.toml (per-project editable templates are a future
// roadmap item, not yet supported — see project.Settings.Templates).
func NewRegistry(control store.ControlStore, dataDir, templatesPath string, wsHub *ws.Hub, logger *slog.Logger) *Registry {
	return &Registry{
		control:       control,
		dataDir:       dataDir,
		templatesPath: templatesPath,
		wsHub:         wsHub,
		logger:        logger,
		runtimes:      make(map[string]*Runtime),
	}
}

// SetOnRuntimeLoaded registers a callback invoked the first time each
// project's Runtime is built. Must be called before the first Get.
func (r *Registry) SetOnRuntimeLoaded(fn func(*Runtime)) {
	r.onRuntimeLoaded = fn
}

// Create registers a new project with default settings and returns it. It
// does not eagerly build a Runtime — call Get for that.
func (r *Registry) Create(ctx context.Context, slug, name string) (*store.Project, error) {
	existing, err := r.control.GetProjectBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("project: check slug %q: %w", slug, err)
	}
	if existing != nil {
		return nil, fmt.Errorf("project: slug %q already exists", slug)
	}

	settingsJSON, err := json.Marshal(DefaultSettings())
	if err != nil {
		return nil, fmt.Errorf("project: marshal default settings: %w", err)
	}

	p := &store.Project{
		ID:           uuid.New().String(),
		Slug:         slug,
		Name:         name,
		SettingsJSON: string(settingsJSON),
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.control.CreateProject(ctx, p); err != nil {
		return nil, fmt.Errorf("project: create %q: %w", slug, err)
	}
	return p, nil
}

// Get returns the Runtime for projectID, opening and caching it on first
// access. Safe for concurrent use.
func (r *Registry) Get(ctx context.Context, projectID string) (*Runtime, error) {
	r.mu.RLock()
	if rt, ok := r.runtimes[projectID]; ok {
		r.mu.RUnlock()
		return rt, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	// Another goroutine may have loaded it while we waited for the write lock.
	if rt, ok := r.runtimes[projectID]; ok {
		r.mu.Unlock()
		return rt, nil
	}

	rt, err := r.load(ctx, projectID)
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.runtimes[projectID] = rt
	r.mu.Unlock()

	if r.onRuntimeLoaded != nil {
		go r.onRuntimeLoaded(rt)
	}
	return rt, nil
}

// LoadedRuntimes returns every Runtime currently cached in memory. Projects
// that haven't been accessed yet (no API request, no explicit Get) aren't
// included — there's nothing to do maintenance on for them.
func (r *Registry) LoadedRuntimes() []*Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Runtime, 0, len(r.runtimes))
	for _, rt := range r.runtimes {
		out = append(out, rt)
	}
	return out
}

func (r *Registry) load(ctx context.Context, projectID string) (*Runtime, error) {
	proj, err := r.control.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project: lookup %s: %w", projectID, err)
	}
	if proj == nil {
		return nil, fmt.Errorf("project: %s not found", projectID)
	}

	settings := DefaultSettings()
	if proj.SettingsJSON != "" {
		if err := json.Unmarshal([]byte(proj.SettingsJSON), &settings); err != nil {
			return nil, fmt.Errorf("project: parse settings for %s: %w", projectID, err)
		}
	}

	projectDir := filepath.Join(r.dataDir, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("project: create data dir for %s: %w", projectID, err)
	}

	ps, err := store.NewSQLiteProjectStore(filepath.Join(projectDir, "data.db"), r.logger)
	if err != nil {
		return nil, fmt.Errorf("project: open store for %s: %w", projectID, err)
	}

	engine := otp.NewEngine(ps, otp.EngineConfig{
		CodeLength:             settings.OTP.CodeLength,
		ExpiryMinutes:          settings.OTP.ExpiryMinutes,
		MaxAttempts:            settings.OTP.MaxAttempts,
		RateLimitPerPhonePerHr: settings.OTP.RateLimitPerPhonePerHr,
	})

	tmplStore, err := templates.NewStore(r.templatesPath, settings.Templates.DefaultLocale)
	if err != nil {
		ps.Close()
		return nil, fmt.Errorf("project: load templates for %s: %w", projectID, err)
	}

	whService := webhooks.NewService(webhooks.Config{
		Endpoint: settings.Webhooks.Endpoint,
		Events:   settings.Webhooks.Events,
		Secret:   settings.Webhooks.Secret,
	}, ps, r.wsHub)

	pool, err := whatsapp.NewPool(whatsapp.PoolConfig{
		DBPath:         filepath.Join(projectDir, "session.db"),
		DeviceName:     fmt.Sprintf("Wotp - %s", proj.Name),
		Backoff:        defaultReconnectBackoff,
		Logger:         r.logger,
		SimulateTyping: settings.Messaging.SimulateTyping,
		IgnoreGroups:   settings.WhatsApp.IgnoreGroups,
		IgnoreStatus:   settings.WhatsApp.IgnoreStatus,
	})
	if err != nil {
		ps.Close()
		return nil, fmt.Errorf("project: open whatsapp pool for %s: %w", projectID, err)
	}
	if err := pool.LoadExisting(ctx); err != nil {
		r.logger.Error("project: failed to reconnect existing numbers", "project_id", projectID, "error", err)
	}
	if err := r.syncNumbers(ctx, projectID, pool); err != nil {
		r.logger.Error("project: failed to sync numbers registry", "project_id", projectID, "error", err)
	}

	var cloud *whatsapp.CloudClient
	if settings.Cloud.Enabled {
		cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
			PhoneNumberID:       settings.Cloud.PhoneNumberID,
			AccessToken:         settings.Cloud.AccessToken,
			OTPTemplateName:     settings.Cloud.OTPTemplateName,
			OTPTemplateLanguage: settings.Cloud.OTPTemplateLanguage,
			BaseURL:             r.cloudBaseURLOverride,
			HTTPClient:          r.cloudHTTPClientOverride,
		})
		if _, err := cloud.Connect(ctx); err != nil {
			// Soft-fail, matching whatsmeow's LoadExisting above: a bad
			// token/phone_number_id shouldn't block the project from
			// loading. The actual send error surfaces clearly per-call.
			r.logger.Error("project: failed to verify cloud api credentials", "project_id", projectID, "error", err)
		}
	}

	return &Runtime{
		Project:   proj,
		Settings:  settings,
		Store:     ps,
		Engine:    engine,
		Templates: tmplStore,
		Webhooks:  whService,
		WA:        pool,
		Cloud:     cloud,
	}, nil
}

// Delete closes and removes a project's runtime (if loaded), its on-disk
// data, and its control-plane rows (project + api_keys + numbers cascade
// via the caller — see store.ControlStore.DeleteProject).
func (r *Registry) Delete(ctx context.Context, projectID string) error {
	r.mu.Lock()
	if rt, ok := r.runtimes[projectID]; ok {
		rt.WA.Disconnect()
		_ = rt.Store.Close()
		delete(r.runtimes, projectID)
	}
	r.mu.Unlock()

	if err := r.control.DeleteProject(ctx, projectID); err != nil {
		return fmt.Errorf("project: delete %s: %w", projectID, err)
	}

	projectDir := filepath.Join(r.dataDir, "projects", projectID)
	if err := os.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("project: remove data dir for %s: %w", projectID, err)
	}
	return nil
}

// Evict closes and drops a project's cached Runtime, if loaded, without
// touching its control-plane row or on-disk data. The next Get rebuilds it
// fresh — existing WhatsApp numbers reconnect automatically via
// LoadExisting, no new pairing needed. Call this after changing a
// project's settings so the new values take effect.
func (r *Registry) Evict(projectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rt, ok := r.runtimes[projectID]; ok {
		rt.WA.Disconnect()
		_ = rt.Store.Close()
		delete(r.runtimes, projectID)
	}
}

// SyncNumbers reconciles a loaded project's live WhatsApp pool state with
// its persisted store.Number row. Call this after a pairing attempt
// completes (success or not) so a newly linked number is reflected in
// control.db.
func (r *Registry) SyncNumbers(ctx context.Context, projectID string) error {
	r.mu.RLock()
	rt, ok := r.runtimes[projectID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("project: %s is not loaded", projectID)
	}
	return r.syncNumbers(ctx, projectID, rt.WA)
}

func (r *Registry) syncNumbers(ctx context.Context, projectID string, pool *whatsapp.Pool) error {
	live := pool.Numbers() // at most one, see whatsapp.Pool
	if len(live) == 0 {
		return nil
	}
	n := live[0]

	status := store.NumberStatusDisconnected
	if n.Connected {
		status = store.NumberStatusConnected
	}

	existing, err := r.control.ListNumbersByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list existing numbers: %w", err)
	}
	row := store.Number{CreatedAt: time.Now().UTC()}
	if len(existing) > 0 {
		row = existing[0]
	}
	row.JID = n.JID
	row.ProjectID = projectID
	row.Phone = n.Phone
	row.Status = status

	if err := r.control.UpsertNumber(ctx, &row); err != nil {
		return fmt.Errorf("upsert number %s: %w", n.JID, err)
	}
	return nil
}
