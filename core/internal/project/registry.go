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

	"github.com/wotp/core/internal/otp"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/templates"
	"github.com/wotp/core/internal/webhooks"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// defaultReconnectBackoff is the WhatsApp reconnect delay schedule (seconds).
// An operational tuning knob, not a business setting, so it isn't part of
// Settings.
var defaultReconnectBackoff = []int{5, 15, 60, 300}

// Runtime bundles everything needed to serve requests for this wotp-core
// instance: its data store, OTP engine, message templates, webhook
// dispatcher, and WhatsApp number pool. wotp-core is mono-tenant, so there
// is exactly one Runtime per running instance.
type Runtime struct {
	// Name is the instance's display name (config.toml [project].name),
	// used for the whatsmeow device name and log lines.
	Name      string
	Settings  Settings
	Store     store.ProjectStore
	Engine    *otp.Engine
	Templates *templates.Store
	Webhooks  *webhooks.Service
	WA        *whatsapp.Pool

	// MediaDir holds downloaded inbound media (image/video/audio/document)
	// — one file per message, named by its WhatsApp message id. See
	// api.routeInboundMessage (writes) and the GET /v1/media/{id} handler
	// (reads).
	MediaDir string

	// Cloud is set only when the instance is configured with a Meta Cloud
	// API backend (see Settings.Cloud) — nil otherwise. Handlers that can
	// use either backend (currently just OTP sending) prefer Cloud when
	// non-nil and fall back to WA/whatsmeow.
	Cloud *whatsapp.CloudClient

	qrMu     sync.RWMutex
	latestQR string

	msgMu     sync.Mutex
	msgCount  int
	msgWindow time.Time
}

// SetLatestQR records the most recent pairing QR code, so it can be
// displayed even if the caller isn't the one that started pairing.
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

// AllowSend enforces the instance's per-minute outbound message rate limit.
// Returns false if the limit (Settings.Messaging.MaxMessagesPerMinute, 0 =
// unlimited) has been exceeded for the current one-minute window.
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

// Close disconnects the WhatsApp pool and closes the data store. Call this
// before discarding a Runtime — on shutdown, or before rebuilding it via
// Load after a settings change (see Server.reloadRuntime).
func (rt *Runtime) Close() {
	rt.WA.Disconnect()
	_ = rt.Store.Close()
}

// LoadOptions configures Load. CloudBaseURLOverride/CloudHTTPClientOverride
// let tests point the Cloud client at an httptest.Server instead of the real
// Meta Graph API — production leaves them zero, which
// whatsapp.NewCloudClient treats as "use the real endpoint /
// http.DefaultClient".
type LoadOptions struct {
	InstanceName            string
	DataDir                 string
	TemplatesPath           string
	CloudBaseURLOverride    string
	CloudHTTPClientOverride *http.Client
}

// Load builds this wotp-core instance's Runtime: reads its settings from
// control, opens (or creates) data.db and session.db directly under
// opts.DataDir, reconnects any already-paired WhatsApp number, and — if the
// instance's settings enable it — verifies the Meta Cloud API credentials.
func Load(ctx context.Context, control store.ControlStore, wsHub *ws.Hub, logger *slog.Logger, opts LoadOptions) (*Runtime, error) {
	settingsJSON, err := control.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: load settings: %w", err)
	}
	settings := DefaultSettings()
	if settingsJSON != "" {
		if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
			return nil, fmt.Errorf("project: parse settings: %w", err)
		}
	}

	ps, err := store.NewSQLiteProjectStore(filepath.Join(opts.DataDir, "data.db"), logger)
	if err != nil {
		return nil, fmt.Errorf("project: open store: %w", err)
	}

	mediaDir := filepath.Join(opts.DataDir, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		ps.Close()
		return nil, fmt.Errorf("project: create media dir: %w", err)
	}

	engine := otp.NewEngine(ps, otp.EngineConfig{
		CodeLength:             settings.OTP.CodeLength,
		ExpiryMinutes:          settings.OTP.ExpiryMinutes,
		MaxAttempts:            settings.OTP.MaxAttempts,
		RateLimitPerPhonePerHr: settings.OTP.RateLimitPerPhonePerHr,
	})

	tmplStore, err := templates.NewStore(opts.TemplatesPath, settings.Templates.DefaultLocale)
	if err != nil {
		ps.Close()
		return nil, fmt.Errorf("project: load templates: %w", err)
	}

	whService := webhooks.NewService(webhooks.Config{
		Endpoint: settings.Webhooks.Endpoint,
		Events:   settings.Webhooks.Events,
		Secret:   settings.Webhooks.Secret,
	}, ps, wsHub)

	pool, err := whatsapp.NewPool(whatsapp.PoolConfig{
		DBPath:         filepath.Join(opts.DataDir, "session.db"),
		DeviceName:     fmt.Sprintf("Wotp - %s", opts.InstanceName),
		Backoff:        defaultReconnectBackoff,
		Logger:         logger,
		SimulateTyping: settings.Messaging.SimulateTyping,
		IgnoreGroups:   settings.WhatsApp.IgnoreGroups,
		IgnoreStatus:   settings.WhatsApp.IgnoreStatus,
	})
	if err != nil {
		ps.Close()
		return nil, fmt.Errorf("project: open whatsapp pool: %w", err)
	}
	if err := pool.LoadExisting(ctx); err != nil {
		logger.Error("project: failed to reconnect existing number", "error", err)
	}
	if err := SyncNumbers(ctx, control, pool); err != nil {
		logger.Error("project: failed to sync numbers registry", "error", err)
	}

	var cloud *whatsapp.CloudClient
	if settings.Cloud.Enabled {
		cloud = whatsapp.NewCloudClient(whatsapp.CloudConfig{
			PhoneNumberID:       settings.Cloud.PhoneNumberID,
			AccessToken:         settings.Cloud.AccessToken,
			OTPTemplateName:     settings.Cloud.OTPTemplateName,
			OTPTemplateLanguage: settings.Cloud.OTPTemplateLanguage,
			WabaID:              settings.Cloud.WabaID,
			AppSecret:           settings.Cloud.AppSecret,
			VerifyToken:         settings.Cloud.VerifyToken,
			Pin:                 settings.Cloud.Pin,
			BaseURL:             opts.CloudBaseURLOverride,
			HTTPClient:          opts.CloudHTTPClientOverride,
			Store:               ps,
		})
		if _, err := cloud.Connect(ctx); err != nil {
			// Soft-fail, matching whatsmeow's LoadExisting above: a bad
			// token/phone_number_id shouldn't block startup. The actual send
			// error surfaces clearly per-call.
			logger.Error("project: failed to verify cloud api credentials", "error", err)
		} else if settings.Cloud.WabaID != "" && settings.Cloud.Pin != "" {
			// Registration is what actually makes Meta deliver inbound
			// events to our webhook — skipped entirely (not attempted, not
			// an error) when WabaID/Pin aren't set, since a send-only (OTP/
			// messages) Cloud setup never needs it and Meta's /register
			// call fails loudly without a PIN.
			if err := cloud.RegisterPhoneNumber(ctx); err != nil {
				logger.Error("project: failed to register cloud number for inbound webhooks", "error", err)
			} else if err := cloud.SubscribeWabaToApp(ctx); err != nil {
				logger.Error("project: failed to subscribe waba to app", "error", err)
			}
		}
	}

	return &Runtime{
		Name:      opts.InstanceName,
		Settings:  settings,
		Store:     ps,
		Engine:    engine,
		Templates: tmplStore,
		Webhooks:  whService,
		WA:        pool,
		Cloud:     cloud,
		MediaDir:  mediaDir,
	}, nil
}

// SyncNumbers reconciles the instance's live WhatsApp pool state with its
// persisted store.Number row. Call after a pairing attempt completes
// (success or not) so a newly linked number is reflected in control.db.
func SyncNumbers(ctx context.Context, control store.ControlStore, pool *whatsapp.Pool) error {
	live := pool.Numbers() // at most one, see whatsapp.Pool
	if len(live) == 0 {
		return nil
	}
	n := live[0]

	status := store.NumberStatusDisconnected
	if n.Connected {
		status = store.NumberStatusConnected
	}

	if err := control.UpsertNumber(ctx, &store.Number{
		JID:       n.JID,
		Phone:     n.Phone,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("upsert number %s: %w", n.JID, err)
	}
	return nil
}
