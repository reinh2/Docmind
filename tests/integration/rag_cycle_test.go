//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/reinh2/docmind/internal/ai"
	"github.com/reinh2/docmind/internal/chat"
	"github.com/reinh2/docmind/internal/domain"
	"github.com/reinh2/docmind/internal/ingestion"
	"github.com/reinh2/docmind/internal/metrics"
	"github.com/reinh2/docmind/internal/notify"
	"github.com/reinh2/docmind/internal/platform/id"
	"github.com/reinh2/docmind/internal/platform/migrate"
	"github.com/reinh2/docmind/internal/store/postgres"
)

func TestUnrelatedQuestionDoesNotReceiveEvidence(t *testing.T) {
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
	const similarityThreshold = 0.10
	question := ""
	// The deterministic demo embedder is intentionally lightweight, so select a unique
	// nonsense query whose nearest stored vector is below the configured evidence gate.
	for attempt := 0; attempt < 64; attempt++ {
		candidate := fmt.Sprintf("zebracorn-helipad-%s-%d", id.New(), attempt)
		embedding, embedErr := provider.Embed(ctx, candidate)
		if embedErr != nil {
			t.Fatal(embedErr)
		}
		matches, searchErr := store.VectorSearch(ctx, embedding, 1)
		if searchErr != nil {
			t.Fatal(searchErr)
		}
		if len(matches) == 0 || matches[0].Score < similarityThreshold {
			question = candidate
			break
		}
	}
	if question == "" {
		t.Fatal("could not construct an unrelated query below the evidence threshold")
	}

	result, err := chat.New(store, provider, similarityThreshold, metrics.New(), notify.LogNotifier{}, id.New).Ask(ctx, "not-found-"+id.New(), "widget", question, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotFound || len(result.Sources) != 0 {
		t.Fatalf("unrelated question must be rejected, got %+v", result)
	}
}

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
	var sourcesFirst bool
	var streamed strings.Builder
	stream := &chat.Stream{
		OnSources: func([]domain.Source) {
			// Sources must be published before any token is streamed.
			sourcesFirst = streamed.Len() == 0
		},
		OnToken: func(delta string) { streamed.WriteString(delta) },
	}
	result, err := chat.New(store, provider, 0.10, metrics.New(), notify.LogNotifier{}, id.New).Ask(ctx, "integration-"+id.New(), "widget", "How much is the initial consultation?", stream)
	if err != nil {
		t.Fatal(err)
	}
	if result.NotFound || len(result.Sources) == 0 || !strings.Contains(result.Sources[0].Document, "Integration prices") {
		t.Fatalf("expected grounded answer, got %+v", result)
	}
	if !sourcesFirst {
		t.Fatal("expected sources to stream before the first answer token")
	}
	if streamed.String() != result.Answer {
		t.Fatalf("streamed answer %q != persisted answer %q", streamed.String(), result.Answer)
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
