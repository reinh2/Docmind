package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/reinh2/docmind/internal/ai"
	"github.com/reinh2/docmind/internal/domain"
	"github.com/reinh2/docmind/internal/store/postgres"
)

type Service struct {
	store         *postgres.Store
	ai            ai.Provider
	size, overlap int
	queue         chan Work
	wg            sync.WaitGroup
}
type Work struct{ JobID, DocumentID, Content, SourceType string }
type Request struct{ Title, SourceType, Content string }

func New(store *postgres.Store, provider ai.Provider, size, overlap, capacity int) *Service {
	s := &Service{store: store, ai: provider, size: size, overlap: overlap, queue: make(chan Work, capacity)}
	s.wg.Add(1)
	go s.run()
	return s
}
func (s *Service) Close()          { close(s.queue); s.wg.Wait() }
func (s *Service) QueueDepth() int { return len(s.queue) }
func (s *Service) Submit(ctx context.Context, req Request, newID func() string) (domain.Document, domain.Job, bool, error) {
	text, err := ExtractText(req.SourceType, req.Content)
	if err != nil {
		return domain.Document{}, domain.Job{}, false, err
	}
	h := sha256.Sum256([]byte(text))
	hash := hex.EncodeToString(h[:])
	doc, dup, err := s.store.CreateOrGetDocument(ctx, newID(), req.Title, req.SourceType, hash)
	if err != nil {
		return doc, domain.Job{}, false, err
	}
	if dup {
		return doc, domain.Job{}, true, nil
	}
	job := domain.Job{ID: newID(), DocumentID: doc.ID, Status: "queued", CreatedAt: time.Now().UTC()}
	if err = s.store.CreateJob(ctx, job, req.SourceType, req.Content); err != nil {
		return doc, job, false, err
	}
	select {
	case s.queue <- Work{job.ID, doc.ID, req.Content, req.SourceType}:
		return doc, job, false, nil
	default:
		_ = s.store.FailJob(ctx, job.ID, fmt.Errorf("ingestion queue is full"))
		return doc, job, false, fmt.Errorf("ingestion queue is full; retry shortly")
	}
}
func (s *Service) Reindex(ctx context.Context, documentID string, newID func() string) (domain.Job, error) {
	source, err := s.store.DocumentSource(ctx, documentID)
	if err != nil {
		return domain.Job{}, err
	}
	job := domain.Job{ID: newID(), DocumentID: documentID, Status: "queued", CreatedAt: time.Now().UTC()}
	if err = s.store.CreateJob(ctx, job, source.SourceType, source.Content); err != nil {
		return job, err
	}
	select {
	case s.queue <- Work{job.ID, documentID, source.Content, source.SourceType}:
		return job, nil
	default:
		_ = s.store.FailJob(ctx, job.ID, fmt.Errorf("ingestion queue is full"))
		return job, fmt.Errorf("ingestion queue is full; retry shortly")
	}
}
func (s *Service) ResumeQueued(ctx context.Context) error {
	jobs, err := s.store.QueuedJobs(ctx, cap(s.queue))
	if err != nil {
		return err
	}
	for _, j := range jobs {
		select {
		case s.queue <- Work{j.JobID, j.DocumentID, j.Content, j.SourceType}:
		default:
			return nil
		}
	}
	return nil
}
func (s *Service) run() {
	defer s.wg.Done()
	for w := range s.queue {
		s.process(w)
	}
}
func (s *Service) process(w Work) {
	ctx := context.Background()
	if err := s.store.SetJobProcessing(ctx, w.JobID); err != nil {
		return
	}
	text, err := ExtractText(w.SourceType, w.Content)
	if err == nil {
		err = s.index(ctx, w.DocumentID, text)
	}
	if err != nil {
		_ = s.store.AddDeadLetter(ctx, newUUID(), "ingestion", "", "", map[string]string{"document_id": w.DocumentID, "job_id": w.JobID}, err)
		_ = s.store.FailJob(ctx, w.JobID, err)
		return
	}
	_ = s.store.CompleteJob(ctx, w.JobID)
}
func (s *Service) index(ctx context.Context, documentID, text string) error {
	parts := ChunkText(text, s.size, s.overlap)
	if len(parts) == 0 {
		return fmt.Errorf("document contains no indexable text")
	}
	staged := make([]postgres.StagedChunk, 0, len(parts))
	for _, p := range parts {
		v, err := s.ai.Embed(ctx, p.Content)
		if err != nil {
			return err
		}
		staged = append(staged, postgres.StagedChunk{ID: newUUID(), Section: p.Section, Content: p.Content, Position: p.Position, Embedding: v})
	}
	return s.store.PublishVersion(ctx, documentID, newUUID(), staged)
}
