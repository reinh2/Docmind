package main

import (
	"context"
	"github.com/example/docmind/internal/ai"
	"github.com/example/docmind/internal/chat"
	"github.com/example/docmind/internal/config"
	"github.com/example/docmind/internal/metrics"
	"github.com/example/docmind/internal/notify"
	"github.com/example/docmind/internal/platform/id"
	"github.com/example/docmind/internal/platform/migrate"
	"github.com/example/docmind/internal/store/postgres"
	"github.com/example/docmind/internal/telegram"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration error", "error", err)
		os.Exit(1)
	}
	store, err := postgres.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("database failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	if err := migrate.Apply(context.Background(), store.Pool()); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	var provider ai.Provider
	if cfg.AIMode == "openrouter" {
		base, err := ai.NewOpenRouter(cfg.OpenRouterAPIKey, cfg.OpenRouterChatModel, cfg.OpenRouterEmbeddingModel, nil)
		if err != nil {
			log.Error("OpenRouter configuration error", "error", err)
			os.Exit(1)
		}
		provider = ai.WithRetries(base, 3)
	} else {
		provider = ai.FakeProvider{}
	}
	service := chat.New(store, provider, cfg.RetrievalThreshold, metrics.New(), notify.LogNotifier{Log: log}, id.New)
	bot := telegram.New(cfg.TelegramBotToken, service, log)
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() { <-stop; cancel() }()
	if err := bot.Run(ctx); err != nil && err != context.Canceled {
		log.Error("Telegram bot stopped", "error", err)
		os.Exit(1)
	}
}
