package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jimgcampbell/mathgames/internal/ai"
	"github.com/jimgcampbell/mathgames/internal/api"
	"github.com/jimgcampbell/mathgames/internal/db"
	"github.com/jimgcampbell/mathgames/internal/game"
	"github.com/jimgcampbell/mathgames/internal/mailer"
	"github.com/jimgcampbell/mathgames/internal/storage"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(log); err != nil {
		log.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := struct {
		DatabaseURL string
		APIKey      string
		Port        string
		MigrDir     string
		PWADir      string
		// Optional — used only to report configured/not-configured in this phase.
		AnthropicKey string
		AIModel      string
		R2AccountID  string
		R2AccessKey  string
		R2SecretKey  string
		R2Bucket     string
		R2PublicURL  string
		SMTPHost     string
		SMTPPort     string
		SMTPUser     string
		SMTPPass     string
		MessageTo    string
	}{
		DatabaseURL:  requireEnv("DATABASE_URL"),
		APIKey:       requireEnv("MATHGAMES_API_KEY"),
		Port:         getEnv("PORT", "8083"),
		MigrDir:      getEnv("MIGRATIONS_DIR", "internal/db/migrations"),
		PWADir:       getEnv("PWA_DIR", "pwa"),
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		AIModel:      getEnv("AI_MODEL", ai.DefaultModel),
		R2AccountID:  os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKey:  os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:     os.Getenv("R2_BUCKET"),
		R2PublicURL:  os.Getenv("R2_PUBLIC_URL"),
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPass:     os.Getenv("SMTP_PASS"),
		MessageTo:    os.Getenv("MESSAGE_TO"), // defaults to SMTP_USER inside mailer.New
	}

	aiEnabled := cfg.AnthropicKey != ""
	if aiEnabled {
		log.Info("AI configured", "model", cfg.AIModel)
	} else {
		log.Info("AI not configured (set ANTHROPIC_API_KEY to enable)")
	}

	videoEnabled := cfg.R2AccountID != "" && cfg.R2AccessKey != "" &&
		cfg.R2SecretKey != "" && cfg.R2Bucket != "" && cfg.R2PublicURL != ""
	if videoEnabled {
		log.Info("R2 video storage configured")
	} else {
		log.Info("R2 video storage not configured (set R2_* vars to enable video clips)")
	}

	messagingEnabled := cfg.SMTPUser != "" && cfg.SMTPPass != ""
	if messagingEnabled {
		log.Info("messaging configured", "smtp_host", cfg.SMTPHost)
	} else {
		log.Info("messaging not configured (set SMTP_USER and SMTP_PASS to enable email delivery; messages are still saved)")
	}

	ctx := context.Background()

	log.Info("connecting to database")
	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database init: %w", err)
	}
	defer database.Close()
	log.Info("database connected")

	log.Info("running migrations", "dir", cfg.MigrDir)
	if err := db.RunMigrations(ctx, database, cfg.MigrDir); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	log.Info("migrations complete")

	if err := db.SeedSkillState(ctx, database, game.Skills); err != nil {
		return fmt.Errorf("seed skill state: %w", err)
	}
	log.Info("skill state seeded")

	mail := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.MessageTo)
	svc := game.NewService(database, mail, log)

	var aiGen *ai.Generator
	if aiEnabled {
		aiGen = ai.NewGenerator(database, ai.NewClient(cfg.AnthropicKey, cfg.AIModel), cfg.AIModel, log)
	}
	gameHandler := api.NewGameHandler(svc, aiGen, log)

	// clipStore is left nil (not a typed nil) when R2 isn't configured, so
	// ClipHandler's h.store == nil check works.
	var clipStore api.ClipStore
	if videoEnabled {
		clipStore = storage.NewR2Client(cfg.R2AccountID, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket, cfg.R2PublicURL)
	}
	clipHandler := api.NewClipHandler(svc, clipStore, log)

	r := api.NewRouter(api.Config{
		APIKey:    cfg.APIKey,
		AI:        aiEnabled,
		Video:     videoEnabled,
		Messaging: messagingEnabled,
		PWADir:    cfg.PWADir,
	}, gameHandler, clipHandler, log)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 2 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	log.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
