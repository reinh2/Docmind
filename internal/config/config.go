package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr                 string
	DatabaseURL              string
	AIMode                   string
	QueueCapacity            int
	ChunkSize                int
	ChunkOverlap             int
	RetrievalThreshold       float64
	AdminToken               string
	OpenRouterAPIKey         string
	OpenRouterChatModel      string
	OpenRouterEmbeddingModel string
	CORSOrigins              string
	RateLimitPerMinute       int
	TelegramBotToken         string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:                 env("DOCMIND_HTTP_ADDR", ":8080"),
		DatabaseURL:              env("DOCMIND_DATABASE_URL", "postgres://docmind:docmind@localhost:5432/docmind?sslmode=disable"),
		AIMode:                   env("DOCMIND_AI_MODE", "fake"),
		QueueCapacity:            envInt("DOCMIND_QUEUE_CAPACITY", 32),
		ChunkSize:                envInt("DOCMIND_CHUNK_SIZE", 900),
		ChunkOverlap:             envInt("DOCMIND_CHUNK_OVERLAP", 160),
		RetrievalThreshold:       envFloat("DOCMIND_RETRIEVAL_THRESHOLD", 0.01),
		AdminToken:               env("DOCMIND_ADMIN_TOKEN", "change-me-before-production"),
		OpenRouterAPIKey:         env("OPENROUTER_API_KEY", ""),
		OpenRouterChatModel:      env("OPENROUTER_CHAT_MODEL", ""),
		OpenRouterEmbeddingModel: env("OPENROUTER_EMBEDDING_MODEL", ""),
		CORSOrigins:              env("DOCMIND_CORS_ORIGINS", "*"),
		RateLimitPerMinute:       envInt("DOCMIND_RATE_LIMIT_PER_MINUTE", 60),
		TelegramBotToken:         env("TELEGRAM_BOT_TOKEN", ""),
	}
	if c.ChunkSize < 100 || c.ChunkOverlap < 0 || c.ChunkOverlap >= c.ChunkSize {
		return Config{}, fmt.Errorf("invalid chunk settings")
	}
	return c, nil
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func envInt(k string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return fallback
}
func envFloat(k string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil && v >= 0 {
		return v
	}
	return fallback
}
