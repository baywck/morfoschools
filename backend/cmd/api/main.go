package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"morfoschools/backend/internal/app"
	"morfoschools/backend/internal/platform/migrate"
	"morfoschools/backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := app.Config{
		Port:    envOr("PORT", "8080"),
		AppEnv:  envOr("APP_ENV", "development"),
		DBUrl:   envOr("DATABASE_URL", ""),
		Valkey:  envOr("VALKEY_URL", ""),
		NatsUrl: envOr("NATS_URL", ""),
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
		cancel()
		logger.Info("database connected, migrations complete")
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
