package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prompttree/backend/internal/ai"
	"prompttree/backend/internal/core"
	"prompttree/backend/internal/exec"
	"prompttree/backend/internal/httpapi"
	"prompttree/backend/internal/platform/config"
	"prompttree/backend/internal/platform/db"
	"prompttree/backend/internal/realtime"
	"prompttree/backend/internal/tree"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	tokens := core.NewTokenIssuer(cfg.JWTSecret)
	auth, err := core.NewAuthService(pool, tokens, cfg.AllowRegistration)
	if err != nil {
		return err
	}
	hub := realtime.NewHub()
	treeSvc := tree.NewService(pool, hub, log)

	var model ai.Model
	if cfg.GeminiAPIKey != "" {
		model = ai.NewGemini(cfg.GeminiAPIKey, cfg.GeminiModel).WithBaseURL(cfg.GeminiBaseURL)
	} else {
		log.Warn("GEMINI_API_KEY is not set: structuring is disabled")
	}
	aiSvc := ai.NewService(treeSvc, model, cfg.AIDailyLimit, log)
	if err := aiSvc.Start(ctx); err != nil {
		return err
	}

	execSvc := exec.NewService(pool, hub, log)
	go execSvc.RunReaper(ctx, 15*time.Second) // истёкшие lease → повтор (ТЗ п. 8.2)

	api := httpapi.New(auth, tokens, treeSvc, aiSvc, execSvc, hub, log, httpapi.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		TrustProxy:     cfg.TrustProxy,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.ListenAddr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
