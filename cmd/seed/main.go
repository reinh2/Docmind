package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/reinh2/docmind/internal/ai"
	"github.com/reinh2/docmind/internal/config"
	"github.com/reinh2/docmind/internal/ingestion"
	"github.com/reinh2/docmind/internal/platform/id"
	"github.com/reinh2/docmind/internal/platform/migrate"
	"github.com/reinh2/docmind/internal/store/postgres"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer store.Close()
	if err = migrate.Apply(ctx, store.Pool()); err != nil {
		panic(err)
	}
	svc := ingestion.New(store, ai.FakeProvider{}, cfg.ChunkSize, cfg.ChunkOverlap, cfg.QueueCapacity)
	defer svc.Close()
	for title, content := range documents {
		_, job, dup, err := svc.Submit(ctx, ingestion.Request{Title: title, SourceType: "markdown", Content: content}, id.New)
		if err != nil {
			panic(err)
		}
		if dup {
			log.Info("seed already present", "title", title)
			continue
		}
		for {
			j, err := store.Job(ctx, job.ID)
			if err != nil {
				panic(err)
			}
			if j.Status == "completed" {
				break
			}
			if j.Status == "failed" {
				panic(j.Error)
			}
			time.Sleep(50 * time.Millisecond)
		}
		log.Info("seeded document", "title", title)
	}
}

var documents = map[string]string{
	"Services and consultations": "# Initial consultation\nAn initial legal consultation lasts up to 60 minutes and costs EUR 190 plus VAT. We offer appointments in Berlin and by secure video call.\n\n# Employment law\nWe advise employees and small businesses on contracts, dismissal protection, and settlement negotiations.",
	"Price list":                 "# Fees\nInitial consultation: EUR 190 plus VAT for up to 60 minutes. Contract review starts at EUR 350 plus VAT. The final fixed fee is confirmed in writing before work starts.\n\n# Payment\nInvoices are payable within 14 calendar days by bank transfer.",
	"Client FAQ":                 "# Appointments\nTo change an appointment, notify us at least 24 hours in advance.\n\n# Confidentiality\nAll information shared with the firm is handled confidentially under applicable professional obligations.",
}
