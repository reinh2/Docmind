package retrieval

import (
	"github.com/example/docmind/internal/domain"
	"testing"
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
