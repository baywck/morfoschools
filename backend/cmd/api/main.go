package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"morfoschools/backend/internal/app"
	"morfoschools/backend/internal/platform/devseed"
	"morfoschools/backend/internal/platform/migrate"
	"morfoschools/backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := app.Config{
		Port:           envOr("PORT", "8080"),
		AppEnv:         envOr("APP_ENV", "development"),
		DBUrl:          envOr("DATABASE_URL", ""),
		Valkey:         envOr("VALKEY_URL", ""),
		NatsUrl:        envOr("NATS_URL", ""),
		AllowedOrigins: parseAllowedOrigins(envOr("ALLOWED_ORIGINS", ""), envOr("APP_ENV", "development")),
	}

	// Fail-fast configuration checks for non-development environments.
	// Catch the common deploy footguns before the server binds a port.
	if cfg.AppEnv != "development" {
		if err := validateProductionConfig(cfg); err != nil {
			logger.Error("refusing to start with insecure configuration", "error", err, "env", cfg.AppEnv)
			os.Exit(1)
		}
	}

	// Connect to database
	var db *sql.DB
	if cfg.DBUrl != "" {
		var err error
		db, err = sql.Open("pgx", cfg.DBUrl)
		if err != nil {
			logger.Error("failed to open database", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Verify connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := db.PingContext(ctx); err != nil {
			cancel()
			logger.Error("failed to ping database", "error", err)
			os.Exit(1)
		}

		// Run migrations
		if err := migrate.Run(ctx, db, logger, migrations.FS); err != nil {
			cancel()
			logger.Error("failed to run migrations", "error", err)
			os.Exit(1)
		}
		logger.Info("database connected, migrations complete")

		// Dev seed
		if envOr("SEED_DEV_DATA", "false") == "true" {
			if err := devseed.Run(ctx, db, logger); err != nil {
				cancel()
				logger.Error("failed to seed dev data", "error", err)
				os.Exit(1)
			}
		}
		cancel()
	}

	application, err := app.New(cfg, logger, db)
	if err != nil {
		logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      application.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", "port", cfg.Port, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseAllowedOrigins splits a comma-separated ALLOWED_ORIGINS env
// var into a list of exact origin strings. When the variable is empty
// AND APP_ENV is development, it falls back to the local SPA origins
// so dev workflows keep working out of the box. In any other env an
// empty list is returned — the CORS middleware will then reject all
// cross-origin browser requests, which is the correct fail-closed
// behaviour for production where the operator must opt in explicitly.
func parseAllowedOrigins(raw, appEnv string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if appEnv == "development" {
			return []string{
				"http://localhost:1666",
				"http://127.0.0.1:1666",
			}
		}
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// validateProductionConfig refuses dangerous defaults in non-development
// environments. Each rule maps to an audit finding (H-2 / H-3 / L-5).
// Adding a new "checked" key here is the cheapest way to keep the
// fail-closed surface honest as the codebase grows.
func validateProductionConfig(cfg app.Config) error {
	dbPass := os.Getenv("POSTGRES_PASSWORD")
	switch dbPass {
	case "":
		return fmt.Errorf("POSTGRES_PASSWORD is required outside development")
	case "change-me", "changeme", "password", "postgres":
		return fmt.Errorf("POSTGRES_PASSWORD is set to a known-weak default; pick a strong random value")
	}
	if len(dbPass) < 16 {
		return fmt.Errorf("POSTGRES_PASSWORD must be at least 16 characters in non-development environments")
	}
	if cfg.DBUrl == "" {
		return fmt.Errorf("DATABASE_URL is required outside development")
	}
	if len(cfg.AllowedOrigins) == 0 {
		return fmt.Errorf("ALLOWED_ORIGINS must list at least one origin outside development")
	}
	for _, origin := range cfg.AllowedOrigins {
		if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
			return fmt.Errorf("ALLOWED_ORIGINS entry %q must include scheme (http:// or https://)", origin)
		}
	}
	return nil
}
