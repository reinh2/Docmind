package ingestion

import (
	"strings"
	"testing"
)

func TestChunkTextKeepsSentencesAndSections(t *testing.T) {
	text := "# Fees\nInitial consultation costs EUR 190. It lasts one hour.\n# Payment\nInvoices are payable in fourteen days."
	chunks := ChunkText(text, 45, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if !strings.HasSuffix(c.Content, ".") {
			t.Fatalf("chunk cuts a sentence: %q", c.Content)
		}
	}
	if chunks[0].Section != "Fees" {
		t.Fatalf("got section %q", chunks[0].Section)
	}
	foundPayment := false
	for _, c := range chunks {
		if c.Section == "Payment" {
			foundPayment = true
		}
	}
	if !foundPayment {
		t.Fatal("payment section missing")
	}
}

func TestChunkTextOverlapsWholeSentence(t *testing.T) {
	chunks := ChunkText("One short sentence. Two short sentence. Three short sentence.", 45, 20)
	if len(chunks) < 2 {
		t.Fatal("expected chunks")
	}
	if !strings.Contains(chunks[1].Content, "Two short sentence.") {
		t.Fatalf("expected a whole-sentence overlap, got %q", chunks[1].Content)
	}
}
