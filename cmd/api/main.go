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

	"github.com/example/docmind/internal/ai"
	"github.com/example/docmind/internal/chat"
	"github.com/example/docmind/internal/config"
	"github.com/example/docmind/internal/httpapi"
	"github.com/example/docmind/internal/ingestion"
	"github.com/example/docmind/internal/metrics"
	"github.com/example/docmind/internal/notify"
	"github.com/example/docmind/internal/platform/id"
	"github.com/example/docmind/internal/platform/migrate"
	"github.com/example/docmind/internal/platform/ratelimit"
	"github.com/example/docmind/internal/store/postgres"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration error", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	if err := migrate.Apply(ctx, store.Pool()); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	var provider ai.Provider
	switch cfg.AIMode {
	case "fake":
		provider = ai.FakeProvider{}
	case "openrouter":
		base, err := ai.NewOpenRouter(cfg.OpenRouterAPIKey, cfg.OpenRouterChatModel, cfg.OpenRouterEmbeddingModel, nil)
		if err != nil {
			log.Error("OpenRouter configuration error", "error", err)
			os.Exit(1)
		}
		provider = ai.WithRetries(base, 3)
	default:
		log.Error("unsupported AI mode", "mode", cfg.AIMode)
		os.Exit(1)
	}
	ingest := ingestion.New(store, provider, cfg.ChunkSize, cfg.ChunkOverlap, cfg.QueueCapacity)
	defer ingest.Close()
	if err := ingest.ResumeQueued(ctx); err != nil {
		log.Error("resume ingestion jobs failed", "error", err)
	}
	registry := metrics.New()
	registry.Set("docmind_ingestion_queue_depth", float64(ingest.QueueDepth()))
	chatService := chat.New(store, provider, cfg.RetrievalThreshold, registry, notify.LogNotifier{Log: log}, id.New)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(store, ingest, chatService, cfg.AdminToken, cfg.CORSOrigins, registry, ratelimit.New(cfg.RateLimitPerMinute, time.Minute), log).Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Info("DocMind API listening", "addr", cfg.HTTPAddr, "ai_mode", cfg.AIMode)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "error", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown failed", "error", err)
	}
}
