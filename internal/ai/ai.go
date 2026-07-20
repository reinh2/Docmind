package ai

import (
	"context"
	"crypto/sha256"
	"math"
	"regexp"
	"strings"

	"github.com/reinh2/docmind/internal/domain"
)

const EmbeddingDimensions = 64

// Provider is the port used by retrieval and chat services. FakeProvider keeps the
// full demo deterministic and offline; OpenRouter implements the same port for
// production model calls.
//
// Answer streams: when onDelta is non-nil the provider invokes it with incremental
// answer fragments as they are produced, and still returns the complete answer.
// A nil onDelta requests a non-streaming completion.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	RewriteQuery(ctx context.Context, history []domain.Message, question string) (string, error)
	Answer(ctx context.Context, question string, context []domain.Chunk, onDelta func(string)) (string, error)
}

type FakeProvider struct{}

var wordPattern = regexp.MustCompile(`[\p{L}\p{N}]{2,}`)

func (FakeProvider) Embed(_ context.Context, text string) ([]float64, error) {
	v := make([]float64, EmbeddingDimensions)
	for _, token := range wordPattern.FindAllString(strings.ToLower(text), -1) {
		h := sha256.Sum256([]byte(token))
		i := int(h[0]) % EmbeddingDimensions
		weight := 1.0
		if len(token) > 7 {
			weight = 1.4
		}
		v[i] += weight * (1 - float64(h[1]&1)*2)
	}
	var norm float64
	for _, n := range v {
		norm += n * n
	}
	if norm == 0 {
		return v, nil
	}
	norm = math.Sqrt(norm)
	for i := range v {
		v[i] /= norm
	}
	return v, nil
}

func (FakeProvider) RewriteQuery(_ context.Context, history []domain.Message, question string) (string, error) {
	// The fake adapter intentionally preserves the latest question. It is deterministic
	// and makes the contract testable; a real provider may expand references using history.
	return strings.TrimSpace(question), nil
}

func (FakeProvider) Answer(_ context.Context, question string, context []domain.Chunk, onDelta func(string)) (string, error) {
	var answer string
	if len(context) == 0 {
		answer = "I could not find this in the knowledge base. Would you like me to connect you with a human?"
	} else {
		text := strings.TrimSpace(context[0].Content)
		if len(text) > 480 {
			text = text[:480] + "…"
		}
		answer = "Based on the available knowledge-base material: " + text
	}
	if onDelta != nil {
		// Emit word-sized deltas so the streaming contract is exercised offline too.
		for _, word := range strings.SplitAfter(answer, " ") {
			onDelta(word)
		}
	}
	return answer, nil
}
