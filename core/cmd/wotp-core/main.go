// wotp-core is the main entry point for the WhatsApp OTP verification server.
// It loads configuration, initializes all subsystems, and runs the API server
// with graceful shutdown support.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wotp/core/internal/api"
	"github.com/wotp/core/internal/config"
	"github.com/wotp/core/internal/keys"
	"github.com/wotp/core/internal/project"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/ws"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config.toml")
	templatesPath := flag.String("templates", "templates.toml", "path to templates.toml")
	dataDir := flag.String("data", "./data", "path to data directory (control.db + data.db + session.db)")
	flag.Parse()

	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("wotp-core starting",
		"config", *configPath,
		"templates", *templatesPath,
		"data_dir", *dataDir,
	)

	// Load config (instance-wide settings only — see core/internal/config).
	// *configPath is optional: if it's not there (e.g. deploying straight
	// from the image on Dokploy/Coolify without the wotp CLI), Load falls
	// back to defaults + WOTP_* env var overrides instead of failing to
	// start.
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		logger.Info("no config.toml found, using defaults + WOTP_* env overrides", "path", *configPath)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	logger.Info("config loaded",
		"instance", cfg.Project.Name,
		"port", cfg.API.Port,
		"driver", cfg.Storage.Driver,
	)

	if cfg.Storage.Driver != "sqlite" {
		logger.Error("unsupported storage driver", "driver", cfg.Storage.Driver)
		os.Exit(1)
	}

	ctx := context.Background()

	// wotp-core is mono-tenant: control.db, data.db, and session.db all live
	// directly under dataDir (see core/internal/project.Load) — the
	// directory itself must exist before we can open control.db in it.
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		logger.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Control store: api keys, number registry, settings — see
	// core/internal/store/control.go.
	controlStore, err := store.NewSQLiteControlStore(*dataDir+"/control.db", logger)
	if err != nil {
		logger.Error("failed to init control store", "error", err)
		os.Exit(1)
	}
	defer controlStore.Close()

	// WebSocket hub broadcasts to every connected dashboard client — one
	// instance, nothing to scope events by.
	wsHub := ws.NewHub(logger)

	keyMgr := keys.NewManager(controlStore)

	loadOpts := project.LoadOptions{
		InstanceName:  cfg.Project.Name,
		DataDir:       *dataDir,
		TemplatesPath: *templatesPath,
	}
	reload := func(ctx context.Context) (*project.Runtime, error) {
		return project.Load(ctx, controlStore, wsHub, logger, loadOpts)
	}

	// Ensure anon/service keys exist (importing WOTP_ANON_KEY/
	// WOTP_SERVICE_KEY from the environment if set) before the first
	// request can arrive.
	anonKey, serviceKey, err := keyMgr.EnsureKeysWithEnvFallback(ctx)
	if err != nil {
		logger.Error("failed to ensure api keys", "error", err)
		os.Exit(1)
	}
	if anonKey != nil {
		fmt.Printf("✔ Anon key:    %s\n", anonKey.FullKey)
	}
	if serviceKey != nil {
		fmt.Printf("✔ Service key: %s\n", serviceKey.FullKey)
	}

	// Eagerly load and connect the instance so a fresh install immediately
	// shows a QR to scan, matching today's zero-config UX.
	rt, err := reload(ctx)
	if err != nil {
		logger.Error("failed to load runtime", "error", err)
		os.Exit(1)
	}

	server := api.NewServer(cfg, controlStore, rt, reload, keyMgr, wsHub, logger)

	// Forward this instance's WhatsApp events to its webhooks/dashboard.
	go server.StartEventForwarder(rt)

	// Skip the whatsmeow auto-pairing prompt when this instance is
	// configured to send OTPs through the Cloud API instead (see
	// project.Settings.Cloud) — it has no whatsmeow number to pair in the
	// first place.
	if !rt.Settings.Cloud.Enabled && !rt.WA.IsConnected() && len(rt.WA.Numbers()) == 0 {
		connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer connectCancel()

		qrChan, err := rt.WA.Pair(connectCtx)
		if err != nil {
			logger.Error("failed to start pairing", "error", err)
			os.Exit(1)
		}
		go func() {
			for qr := range qrChan {
				logger.Info("new QR code available, scan with WhatsApp")
				rt.SetLatestQR(qr)
			}
			// The channel only closes once pairing is done (success or
			// timeout) — clear the QR so the dashboard stops being served a
			// stale/expired code indefinitely.
			rt.SetLatestQR("")
		}()
	}

	// Periodically expire stale OTPs.
	tickerCtx, tickerCancel := context.WithCancel(ctx)
	defer tickerCancel()
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-tickerCtx.Done():
				return
			case <-ticker.C:
				expired, err := rt.Store.ExpireStaleOTPs(ctx, time.Now())
				if err != nil {
					logger.Error("expire stale otps error", "error", err)
				} else if expired > 0 {
					logger.Info("expired stale otps", "count", expired)
				}
			}
		}
	}()

	// Start HTTP server
	httpSrv := server.ListenAndServe()
	fmt.Printf("✔ API ready at http://localhost:%d\n", cfg.API.Port)
	if cfg.API.EnableDashboard {
		fmt.Printf("✔ Dashboard at http://localhost:%d/dashboard\n", cfg.API.Port)
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	tickerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}

	rt.WA.Disconnect()
	logger.Info("wotp-core stopped")
}
