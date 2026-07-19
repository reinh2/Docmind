//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/docmind/internal/ai"
	"github.com/example/docmind/internal/chat"
	"github.com/example/docmind/internal/ingestion"
	"github.com/example/docmind/internal/metrics"
	"github.com/example/docmind/internal/notify"
	"github.com/example/docmind/internal/platform/id"
	"github.com/example/docmind/internal/platform/migrate"
	"github.com/example/docmind/internal/store/postgres"
)

func TestUploadQuestionAnswerWithSource(t *testing.T) {
	url := os.Getenv("DOCMIND_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DOCMIND_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := migrate.Apply(ctx, store.Pool()); err != nil {
		t.Fatal(err)
	}
	provider := ai.FakeProvider{}
	ingest := ingestion.New(store, provider, 120, 20, 2)
	defer ingest.Close()
	doc, job, duplicate, err := ingest.Submit(ctx, ingestion.Request{Title: "Integration prices", SourceType: "markdown", Content: "# Prices\nAn initial consultation costs EUR 190 plus VAT."}, id.New)
	if err != nil || duplicate {
		t.Fatalf("submit: doc=%+v duplicate=%v err=%v", doc, duplicate, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		j, e := store.Job(ctx, job.ID)
		if e != nil {
			t.Fatal(e)
		}
		if j.Status == "completed" {
			break
		}
		if j.Status == "failed" {
			t.Fatal(j.Error)
		}
		if time.Now().After(deadline) {
			t.Fatal("ingestion timeout")
		}
		time.Sleep(25 * time.Millisecond)
	}
	result, err := chat.New(store, provider, 0.01, metrics.New(), notify.LogNotifier{}, id.New).Ask(ctx, "integration-"+id.New(), "widget", "How much is the initial consultation?")
	if err != nil {
		t.Fatal(err)
	}
	if result.NotFound || len(result.Sources) == 0 || !strings.Contains(result.Sources[0].Document, "Integration prices") {
		t.Fatalf("expected grounded answer, got %+v", result)
	}
}

func TestPublishedVersionReplacesOldChunksAtomically(t *testing.T) {
	url := os.Getenv("DOCMIND_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DOCMIND_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := migrate.Apply(ctx, store.Pool()); err != nil {
		t.Fatal(err)
	}
	p := ai.FakeProvider{}
	doc, _, err := store.CreateOrGetDocument(ctx, id.New(), "Versioned document "+id.New(), "markdown", id.New())
	if err != nil {
		t.Fatal(err)
	}
	old, _ := p.Embed(ctx, "legacy-only-policy")
	if err = store.PublishVersion(ctx, doc.ID, id.New(), []postgres.StagedChunk{{ID: id.New(), Section: "Policy", Content: "legacy-only-policy", Position: 0, Embedding: old}}); err != nil {
		t.Fatal(err)
	}
	newEmbedding, _ := p.Embed(ctx, "replacement-only-policy")
	if err = store.PublishVersion(ctx, doc.ID, id.New(), []postgres.StagedChunk{{ID: id.New(), Section: "Policy", Content: "replacement-only-policy", Position: 0, Embedding: newEmbedding}}); err != nil {
		t.Fatal(err)
	}
	results, err := store.TextSearch(ctx, "legacy-only-policy", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("superseded chunks are still searchable: %+v", results)
	}
}
