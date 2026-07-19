package retrieval

import (
	"sort"

	"github.com/example/docmind/internal/domain"
)

const rrfK = 60.0

// Fuse merges independently-ranked lists. RRF deliberately uses rank instead of raw
// vector/tsquery scores, which are not naturally comparable.
func Fuse(vector, text []domain.Chunk, limit int) []domain.Chunk {
	type entry struct {
		chunk domain.Chunk
		score float64
	}
	merged := map[string]entry{}
	add := func(items []domain.Chunk) {
		for i, c := range items {
			e := merged[c.ID]
			if e.chunk.ID == "" {
				e.chunk = c
			}
			e.score += 1.0 / (rrfK + float64(i+1))
			merged[c.ID] = e
		}
	}
	add(vector)
	add(text)
	out := make([]domain.Chunk, 0, len(merged))
	for _, e := range merged {
		e.chunk.Score = e.score
		out = append(out, e.chunk)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
