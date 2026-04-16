package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"slopyard/internal/server"
	"slopyard/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := server.Config{
		Addr:              env("ADDR", ":8080"),
		FingerprintSecret: env("FINGERPRINT_SECRET", "dev-secret-change-me"),
		TrustProxyHeaders: envBool("TRUST_PROXY_HEADERS", false),
		GlobalLimit:       envInt("GLOBAL_RATE_LIMIT", 20),
		GlobalWindow:      envDuration("GLOBAL_RATE_WINDOW", time.Minute),
		HostWindow:        envDuration("HOST_REPORT_WINDOW", 24*time.Hour),
	}

	if cfg.FingerprintSecret == "dev-secret-change-me" {
		logger.Warn("FINGERPRINT_SECRET is not set; using development fallback")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	pg, err := store.NewPostgres(ctx, databaseURL)
	if err != nil {
		logger.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pg.Close()

	var limiter store.Limiter = store.AllowAllLimiter{}
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		redisLimiter, err := store.NewRedisLimiter(redisURL, cfg.GlobalLimit, cfg.GlobalWindow, cfg.HostWindow)
		if err != nil {
			logger.Error("connect redis", "err", err)
			os.Exit(1)
		}
		defer redisLimiter.Close()
		limiter = redisLimiter
	} else {
		logger.Warn("REDIS_URL is not set; rate limiting is disabled")
	}

	app, err := server.New(cfg, pg, limiter, logger)
	if err != nil {
		logger.Error("build server", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
