package main

import (
	"context"
	"errors"
	"log/slog"
	"nebulaiq/internal/config"
	"nebulaiq/internal/httpapi"
	"nebulaiq/internal/providers"
	"nebulaiq/internal/storage/postgres"
	"nebulaiq/internal/workflow"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	_ = s.Recover(root)
	_ = s.Cleanup(root)
	var m providers.Model
	var q providers.Search
	if c.GeminiKey != "" {
		m, e = providers.NewGemini(root, c.GeminiKey, c.Model)
		if e != nil {
			return errors.New("Gemini initialization failed")
		}
	}
	if c.TavilyKey != "" {
		q, e = providers.NewTavily(c.TavilyKey)
		if e != nil {
			return e
		}
	}
	engine := workflow.New(root, s, m, q, c)
	if c.GroundedEnabled && c.GeminiKey != "" {
		ground, err := providers.NewGroundedGemini(root, c.GeminiKey, c.GroundedModel)
		if err != nil {
			return errors.New("grounded Gemini initialization failed")
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
		_ = server.Shutdown(ctx)
	}()
	slog.Info("Dining notebook ready", "url", "http://"+addr, "auth", "guest; signup/signin deferred")
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		return errors.New("HTTP server failed to start")
	}
	return nil
}
