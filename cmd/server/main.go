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
	}{
		DatabaseURL:  requireEnv("DATABASE_URL"),
		APIKey:       requireEnv("MATHGAMES_API_KEY"),
		Port:         getEnv("PORT", "8083"),
		MigrDir:      getEnv("MIGRATIONS_DIR", "internal/db/migrations"),
		PWADir:       getEnv("PWA_DIR", "pwa"),
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		AIModel:      getEnv("AI_MODEL", ai.DefaultModel),
	}

	aiEnabled := cfg.AnthropicKey != ""
	if aiEnabled {
		log.Info("AI configured", "model", cfg.AIModel)
	} else {
		log.Info("AI not configured (set ANTHROPIC_API_KEY to enable)")
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

	svc := game.NewService(database, log)

	var aiGen *ai.Generator
	if aiEnabled {
		aiGen = ai.NewGenerator(database, ai.NewClient(cfg.AnthropicKey, cfg.AIModel), cfg.AIModel, log)
	}
	gameHandler := api.NewGameHandler(svc, aiGen, log)

	r := api.NewRouter(api.Config{
		APIKey: cfg.APIKey,
		AI:     aiEnabled,
		PWADir: cfg.PWADir,
	}, gameHandler, log)

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
