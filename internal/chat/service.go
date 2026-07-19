package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/example/docmind/internal/ai"
	"github.com/example/docmind/internal/domain"
	"github.com/example/docmind/internal/metrics"
	"github.com/example/docmind/internal/notify"
	"github.com/example/docmind/internal/retrieval"
	"github.com/example/docmind/internal/store/postgres"
)

type Service struct {
	store     *postgres.Store
	ai        ai.Provider
	threshold float64
	metrics   *metrics.Registry
	notifier  notify.Notifier
	newID     func() string
}
type Result struct {
	ConversationID, MessageID, Answer   string
	NotFound                            bool
	Sources                             []domain.Source
	RetrievalLatency, GenerationLatency time.Duration
}

func New(store *postgres.Store, provider ai.Provider, threshold float64, registry *metrics.Registry, notifier notify.Notifier, newID func() string) *Service {
	return &Service{store: store, ai: provider, threshold: threshold, metrics: registry, notifier: notifier, newID: newID}
}
func (s *Service) Feedback(ctx context.Context, messageID string, rating int, comment string) error {
	if rating != 1 && rating != -1 {
		return fmt.Errorf("rating must be 1 or -1")
	}
	return s.store.AddFeedback(ctx, messageID, rating, comment)
}
func (s *Service) Escalate(ctx context.Context, conversationID, reason string) error {
	id := s.newID()
	if err := s.store.AddEscalation(ctx, id, conversationID, reason); err != nil {
		return err
	}
	s.metrics.Inc("docmind_escalations_total")
	if s.notifier == nil {
		return nil
	}
	return s.notifier.NotifyEscalation(ctx, notify.Escalation{ID: id, ConversationID: conversationID, Reason: reason})
}

// Ask is save-first: the question is durable before rewrite, retrieval, or generation.
func (s *Service) Ask(ctx context.Context, session, channel, question string) (Result, error) {
	conv, err := s.store.GetOrCreateConversation(ctx, s.newID(), session, channel)
	if err != nil {
		return Result{}, err
	}
	questionID := s.newID()
	if err = s.store.AddMessage(ctx, domain.Message{ID: questionID, ConversationID: conv.ID, Role: "user", Content: question}, 0); err != nil {
		return Result{}, err
	}
	fail := func(operation string, cause error) (Result, error) {
		s.metrics.Inc("docmind_provider_errors_total")
		_ = s.store.AddDeadLetter(ctx, s.newID(), operation, conv.ID, questionID, map[string]string{"question": question}, cause)
		return Result{}, cause
	}
	history, err := s.store.Messages(ctx, conv.ID, 12)
	if err != nil {
		return Result{}, err
	}
	query, err := s.ai.RewriteQuery(ctx, history, question)
	if err != nil {
		return fail("query_rewrite", err)
	}
	retrievalStart := time.Now()
	embedding, err := s.ai.Embed(ctx, query)
	if err != nil {
		return fail("embedding", err)
	}
	vector, err := s.store.VectorSearch(ctx, embedding, 8)
	if err != nil {
		return Result{}, err
	}
	text, err := s.store.TextSearch(ctx, query, 8)
	if err != nil {
		return Result{}, err
	}
	chunks := retrieval.Fuse(vector, text, 5)
	retrievalLatency := time.Since(retrievalStart)
	s.metrics.Observe("docmind_retrieval_latency_seconds", retrievalLatency.Seconds())
	notFound := len(chunks) == 0 || chunks[0].Score < s.threshold
	if notFound {
		chunks = nil
		s.metrics.Inc("docmind_not_found_total")
	}
	generationStart := time.Now()
	answer, err := s.ai.Answer(ctx, question, chunks)
	if err != nil {
		return fail("generation", err)
	}
	generationLatency := time.Since(generationStart)
	s.metrics.Observe("docmind_generation_latency_seconds", generationLatency.Seconds())
	messageID := s.newID()
	if err = s.store.AddMessage(ctx, domain.Message{ID: messageID, ConversationID: conv.ID, Role: "assistant", Content: answer, NotFound: notFound}, int(retrievalLatency.Milliseconds()+generationLatency.Milliseconds())); err != nil {
		return Result{}, err
	}
	sources := make([]domain.Source, 0, len(chunks))
	for _, c := range chunks {
		excerpt := c.Content
		if len(excerpt) > 280 {
			excerpt = excerpt[:280] + "…"
		}
		sources = append(sources, domain.Source{ChunkID: c.ID, Document: c.Document, Section: c.Section, Excerpt: excerpt, Score: c.Score})
	}
	if err = s.store.AddSources(ctx, messageID, sources); err != nil {
		return Result{}, err
	}
	return Result{ConversationID: conv.ID, MessageID: messageID, Answer: answer, NotFound: notFound, Sources: sources, RetrievalLatency: retrievalLatency, GenerationLatency: generationLatency}, nil
}
