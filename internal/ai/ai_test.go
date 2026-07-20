package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/reinh2/docmind/internal/domain"
)

func TestFakeRewriteKeepsLatestQuestion(t *testing.T) {
	got, err := FakeProvider{}.RewriteQuery(context.Background(), []domain.Message{{Content: "old"}}, "  What does it cost?  ")
	if err != nil || got != "What does it cost?" {
		t.Fatalf("got %q, %v", got, err)
	}
}
func TestFakeAnswerNeverInventsWithoutContext(t *testing.T) {
	got, _ := FakeProvider{}.Answer(context.Background(), "Where?", nil, nil)
	if got == "" {
		t.Fatal("expected a safe fallback")
	}
}
func TestFakeAnswerStreamsDeltasThatReassembleToAnswer(t *testing.T) {
	var streamed strings.Builder
	got, err := FakeProvider{}.Answer(context.Background(), "cost?",
		[]domain.Chunk{{Content: "The initial consultation costs EUR 190 plus VAT."}},
		func(delta string) { streamed.WriteString(delta) })
	if err != nil {
		t.Fatal(err)
	}
	if streamed.String() != got {
		t.Fatalf("streamed %q != returned %q", streamed.String(), got)
	}
}
