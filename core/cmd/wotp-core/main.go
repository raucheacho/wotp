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
	dataDir := flag.String("data", "./data", "path to data directory (control db + per-project data/session)")
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

	// Load config (instance-wide settings only — see core/internal/config)
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

	// wotp-core manages its own layout underneath dataDir (control.db +
	// per-project subdirectories — see core/internal/project/registry.go),
	// but the top-level directory itself must exist before we can open
	// control.db in it.
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		logger.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Control store: projects, api keys, number registry — shared across
	// the whole instance (see core/internal/store/control.go).
	controlStore, err := store.NewSQLiteControlStore(*dataDir+"/control.db", logger)
	if err != nil {
		logger.Error("failed to init control store", "error", err)
		os.Exit(1)
	}
	defer controlStore.Close()

	// WebSocket hub is instance-wide; broadcasts are scoped per-project via
	// ws.Event.ProjectID (see core/internal/ws/hub.go).
	wsHub := ws.NewHub(logger)

	registry := project.NewRegistry(controlStore, *dataDir, *templatesPath, wsHub, logger)

	keyMgr := keys.NewManager(controlStore)
	server := api.NewServer(cfg, controlStore, registry, keyMgr, wsHub, logger)

	// Forward each project's WhatsApp events to its webhooks/dashboard as
	// soon as that project's Runtime is first loaded.
	registry.SetOnRuntimeLoaded(server.StartEventForwarder)

	// Ensure an instance root key exists (authorizes /v1/projects*).
	rootKey, err := keyMgr.EnsureRootKey(ctx)
	if err != nil {
		logger.Error("failed to ensure root key", "error", err)
		os.Exit(1)
	}
	if rootKey != nil {
		fmt.Printf("✔ Root key:    %s\n", rootKey.FullKey)
	}

	// Ensure a "default" project exists so a fresh install works out of the
	// box without an explicit `wotp project create` step.
	defaultProject, err := controlStore.GetProjectBySlug(ctx, "default")
	if err != nil {
		logger.Error("failed to look up default project", "error", err)
		os.Exit(1)
	}
	if defaultProject == nil {
		defaultProject, err = registry.Create(ctx, "default", cfg.Project.Name)
		if err != nil {
			logger.Error("failed to create default project", "error", err)
			os.Exit(1)
		}
	}

	anonKey, serviceKey, err := keyMgr.EnsureKeysWithEnvFallback(ctx, defaultProject.ID)
	if err != nil {
		logger.Error("failed to ensure api keys for default project", "error", err)
		os.Exit(1)
	}
	if anonKey != nil {
		fmt.Printf("✔ Anon key:    %s\n", anonKey.FullKey)
	}
	if serviceKey != nil {
		fmt.Printf("✔ Service key: %s\n", serviceKey.FullKey)
	}

	// Eagerly load and connect the default project so a fresh install
	// immediately shows a QR to scan, matching today's zero-config UX.
	defaultRuntime, err := registry.Get(ctx, defaultProject.ID)
	if err != nil {
		logger.Error("failed to load default project", "error", err)
		os.Exit(1)
	}
	// Skip the whatsmeow auto-pairing prompt when this project is configured
	// to send OTPs through the Cloud API instead (see project.Settings.Cloud)
	// — it has no whatsmeow number to pair in the first place.
	if !defaultRuntime.Settings.Cloud.Enabled && !defaultRuntime.WA.IsConnected() && len(defaultRuntime.WA.Numbers()) == 0 {
		connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer connectCancel()

		qrChan, err := defaultRuntime.WA.Pair(connectCtx)
		if err != nil {
			logger.Error("failed to start pairing default project", "error", err)
			os.Exit(1)
		}
		go func() {
			for qr := range qrChan {
				logger.Info("new QR code available, scan with WhatsApp")
				defaultRuntime.SetLatestQR(qr)
			}
		}()
	}

	// Periodically expire stale OTPs for every project currently loaded in
	// memory (dormant, never-accessed projects have nothing to sweep).
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
				for _, rt := range registry.LoadedRuntimes() {
					expired, err := rt.Store.ExpireStaleOTPs(ctx, time.Now())
					if err != nil {
						logger.Error("expire stale otps error", "project_id", rt.Project.ID, "error", err)
					} else if expired > 0 {
						logger.Info("expired stale otps", "project_id", rt.Project.ID, "count", expired)
					}
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

	for _, rt := range registry.LoadedRuntimes() {
		rt.WA.Disconnect()
	}
	logger.Info("wotp-core stopped")
}
