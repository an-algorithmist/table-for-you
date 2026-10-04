package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/httpapi"
	"table-for-you/backend/internal/llm"
	"table-for-you/backend/internal/pipeline"
	"table-for-you/backend/internal/storage/postgres"
	"table-for-you/backend/internal/tools"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error(e.Error())
		os.Exit(1)
	}
}
func run() error {
	envPath := os.Getenv("NEBULA_ENV_FILE")
	if envPath == "" {
		envPath = ".env"
	}
	if e := config.LoadEnv(envPath); e != nil {
		return errors.New("cannot load .env")
	}
	c, e := config.Load()
	if e != nil {
		return e
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(root, 10*time.Second)
	s, e := postgres.Open(ctx, c.DatabaseURL)
	cancel()
	if e != nil {
		return e
	}
	defer s.Close()
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "migrate" {
		ctx, cc := context.WithTimeout(root, 30*time.Second)
		defer cc()
		if e = s.Migrate(ctx); e != nil {
			return errors.New("database migration failed")
		}
		slog.Info("Database migrations applied")
		return nil
	}
	if command != "serve" {
		return errors.New("usage: nebula [migrate|serve]")
	}
	var schemaReady bool
	if e = s.Pool.QueryRow(root, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='003_grounded_usage.sql')").Scan(&schemaReady); e != nil || !schemaReady {
		return errors.New("database schema missing; run nebula migrate first")
	}
	if err := s.Recover(root); err != nil {
		return fmt.Errorf("recover interrupted runs: %w", err)
	}
	if err := s.Cleanup(root); err != nil {
		slog.Warn("startup cleanup failed", "error", err)
	}
	var m pipeline.Model
	var q pipeline.Search
	if c.GeminiKey != "" {
		m, e = llm.NewGemini(root, c.GeminiKey, c.Model)
		if e != nil {
			return errors.New("gemini initialization failed")
		}
	}
	if c.TavilyKey != "" {
		q, e = tools.NewTavily(c.TavilyKey)
		if e != nil {
			return e
		}
	}
	engine := pipeline.New(root, s, m, q, c)
	if c.GroundedEnabled && c.GeminiKey != "" {
		ground, err := llm.NewGroundedGemini(root, c.GeminiKey, c.GroundedModel)
		if err != nil {
			return errors.New("grounded gemini initialization failed")
		}
		engine.Grounded = ground
	}
	if !engine.Ready() {
		slog.Warn("Provider keys missing; UI/history work but research is disabled")
	}
	addr := ":" + c.Port
	if c.Local {
		addr = "127.0.0.1:" + c.Port
	}
	server := &http.Server{Addr: addr, Handler: httpapi.New(s, engine, c), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	go func() {
		<-root.Done()
		ctx, cc := context.WithTimeout(context.Background(), 6*time.Second)
		defer cc()
		if err := server.Shutdown(ctx); err != nil {
			slog.Warn("HTTP shutdown failed", "error", err)
		}
	}()
	slog.Info("Dining notebook ready", "url", "http://"+addr, "auth", "guest; signup/signin deferred")
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		return errors.New("HTTP server failed to start")
	}
	return nil
}
