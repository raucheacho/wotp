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
	"github.com/wotp/core/internal/otp"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/templates"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config.toml")
	templatesPath := flag.String("templates", "templates.toml", "path to templates.toml")
	dataDir := flag.String("data", "./data", "path to data directory (db + session)")
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

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	logger.Info("config loaded",
		"project", cfg.Project.Name,
		"port", cfg.API.Port,
		"driver", cfg.Storage.Driver,
	)

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir+"/db", 0755); err != nil {
		logger.Error("failed to create data dir", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*dataDir+"/session", 0755); err != nil {
		logger.Error("failed to create session dir", "error", err)
		os.Exit(1)
	}

	// Initialize store
	var dataStore store.Store
	switch cfg.Storage.Driver {
	case "sqlite":
		dbPath := *dataDir + "/db/wotp.db"
		dataStore, err = store.NewSQLiteStore(dbPath, logger)
		if err != nil {
			logger.Error("failed to init sqlite store", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("unsupported storage driver", "driver", cfg.Storage.Driver)
		os.Exit(1)
	}
	defer dataStore.Close()

	// Initialize OTP engine
	otpEngine := otp.NewEngine(dataStore, otp.EngineConfig{
		CodeLength:             cfg.OTP.CodeLength,
		ExpiryMinutes:          cfg.OTP.ExpiryMinutes,
		MaxAttempts:            cfg.OTP.MaxAttempts,
		RateLimitPerPhonePerHr: cfg.OTP.RateLimitPerPhonePerHr,
	})

	// Initialize API key manager and ensure keys exist
	keyMgr := keys.NewManager(dataStore)
	ctx := context.Background()
	anonKey, serviceKey, err := keyMgr.EnsureKeys(ctx)
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

	// Load templates
	tmplStore, err := templates.NewStore(*templatesPath, cfg.Templates.DefaultLocale)
	if err != nil {
		logger.Error("failed to load templates", "error", err)
		os.Exit(1)
	}
	logger.Info("templates loaded", "locales", tmplStore.Locales())

	// Initialize WhatsApp client
	waClient, err := whatsapp.NewMeowClient(whatsapp.MeowConfig{
		DBPath:     *dataDir + "/session/whatsapp.db",
		DeviceName: cfg.WhatsApp.DeviceName,
		Backoff:    cfg.WhatsApp.ReconnectBackoffSeconds,
		Logger:     logger,
	})
	if err != nil {
		logger.Error("failed to init whatsapp client", "error", err)
		os.Exit(1)
	}

	// Connect to WhatsApp
	connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer connectCancel()

	qrChan, err := waClient.Connect(connectCtx)
	if err != nil {
		logger.Error("failed to connect whatsapp", "error", err)
		os.Exit(1)
	}

	// Initialize WebSocket hub
	wsHub := ws.NewHub(logger)

	// Create API server
	server := api.NewServer(cfg, otpEngine, keyMgr, waClient, tmplStore, wsHub, logger)

	// Handle QR codes in background
	if qrChan != nil {
		go func() {
			for qr := range qrChan {
				logger.Info("new QR code available, scan with WhatsApp")
				server.SetLatestQR(qr)
			}
		}()
	}

	// Start event forwarder (WhatsApp → WebSocket)
	evtCtx, evtCancel := context.WithCancel(ctx)
	defer evtCancel()
	go server.StartEventForwarder(evtCtx)

	// Start OTP expiry cleanup ticker
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-evtCtx.Done():
				return
			case <-ticker.C:
				expired, err := dataStore.ExpireStaleOTPs(ctx, time.Now())
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
	evtCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}

	waClient.Disconnect()
	logger.Info("wotp-core stopped")
}
