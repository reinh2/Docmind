package retrieval

import (
	"testing"

	"github.com/reinh2/docmind/internal/domain"
)

func TestFuseRewardsAgreement(t *testing.T) {
	a := domain.Chunk{ID: "a"}
	b := domain.Chunk{ID: "b"}
	out := Fuse([]domain.Chunk{a, b}, []domain.Chunk{b}, 2)
	if len(out) != 2 || out[0].ID != "b" {
		t.Fatalf("expected shared result first: %#v", out)
	}
	if out[0].Score <= out[1].Score {
		t.Fatal("RRF scores were not merged")
	}
}

func TestFilterByMinScoreKeepsOnlyRelevantVectorResults(t *testing.T) {
	items := []domain.Chunk{{ID: "relevant", Score: 0.31}, {ID: "noise", Score: 0.09}}
	filtered := FilterByMinScore(items, 0.1)
	if len(filtered) != 1 || filtered[0].ID != "relevant" {
		t.Fatalf("filtered results = %#v, want only the relevant chunk", filtered)
	}
}
