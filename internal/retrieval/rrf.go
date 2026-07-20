package retrieval

import (
	"sort"

	"github.com/reinh2/docmind/internal/domain"
)

const rrfK = 60.0

// FilterByMinScore keeps only vector candidates that clear a real cosine-similarity
// threshold. RRF scores encode rank, not relevance, so they must never be used as
// the evidence gate by themselves.
func FilterByMinScore(chunks []domain.Chunk, minScore float64) []domain.Chunk {
	filtered := make([]domain.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.Score >= minScore {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

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
