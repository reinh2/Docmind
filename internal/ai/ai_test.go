package ai

import (
	"context"
	"github.com/example/docmind/internal/domain"
	"testing"
)

func TestFakeRewriteKeepsLatestQuestion(t *testing.T) {
	got, err := FakeProvider{}.RewriteQuery(context.Background(), []domain.Message{{Content: "old"}}, "  What does it cost?  ")
	if err != nil || got != "What does it cost?" {
		t.Fatalf("got %q, %v", got, err)
	}
}
func TestFakeAnswerNeverInventsWithoutContext(t *testing.T) {
	got, _ := FakeProvider{}.Answer(context.Background(), "Where?", nil)
	if got == "" {
		t.Fatal("expected a safe fallback")
	}
}
