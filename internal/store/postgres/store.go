package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reinh2/docmind/internal/domain"
)

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Store{pool: p}, nil
}
func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Pool() *pgxpool.Pool            { return s.pool }

func (s *Store) CreateOrGetDocument(ctx context.Context, id, title, sourceType, hash string) (domain.Document, bool, error) {
	var d domain.Document
	err := s.pool.QueryRow(ctx, `SELECT id::text,title,source_type,content_hash,COALESCE(active_version_id::text,''),created_at FROM documents WHERE content_hash=$1`, hash).
		Scan(&d.ID, &d.Title, &d.SourceType, &d.ContentHash, &d.ActiveVersionID, &d.CreatedAt)
	if err == nil {
		return d, true, nil
	}
	if err != pgx.ErrNoRows {
		return d, false, err
	}
	err = s.pool.QueryRow(ctx, `INSERT INTO documents(id,title,source_type,content_hash) VALUES($1,$2,$3,$4) RETURNING created_at`, id, title, sourceType, hash).Scan(&d.CreatedAt)
	if err != nil {
		return d, false, err
	}
	d.ID, d.Title, d.SourceType, d.ContentHash = id, title, sourceType, hash
	return d, false, nil
}

func (s *Store) CreateJob(ctx context.Context, job domain.Job, sourceType, content string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO ingestion_jobs(id,document_id,status,source_content,source_type) VALUES($1,$2,'queued',$3,$4)`, job.ID, job.DocumentID, content, sourceType)
	return err
}
func (s *Store) Job(ctx context.Context, id string) (domain.Job, error) {
	var j domain.Job
	var finished *time.Time
	err := s.pool.QueryRow(ctx, `SELECT id::text,document_id::text,status,COALESCE(error,''),created_at,completed_at FROM ingestion_jobs WHERE id=$1`, id).Scan(&j.ID, &j.DocumentID, &j.Status, &j.Error, &j.CreatedAt, &finished)
	j.CompletedAt = finished
	return j, err
}
func (s *Store) SetJobProcessing(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE ingestion_jobs SET status='processing',started_at=now() WHERE id=$1`, id)
	return err
}
func (s *Store) CompleteJob(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE ingestion_jobs SET status='completed',completed_at=now() WHERE id=$1`, id)
	return err
}
func (s *Store) FailJob(ctx context.Context, id string, cause error) error {
	_, err := s.pool.Exec(ctx, `UPDATE ingestion_jobs SET status='failed', error=$2,completed_at=now() WHERE id=$1`, id, cause.Error())
	return err
}

type Work struct{ JobID, DocumentID, Content, SourceType string }
type AdminConversation struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	Channel       string    `json:"channel"`
	CreatedAt     time.Time `json:"created_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	MessageCount  int       `json:"message_count"`
}
type AdminMessage struct {
	domain.Message
	Sources []domain.Source `json:"sources"`
}
type Dashboard struct {
	QuestionsToday   int             `json:"questions_today"`
	NotFoundRatio    float64         `json:"not_found_ratio"`
	EscalationRatio  float64         `json:"escalation_ratio"`
	AverageLatencyMS float64         `json:"average_latency_ms"`
	PositiveFeedback int             `json:"positive_feedback"`
	NegativeFeedback int             `json:"negative_feedback"`
	TopQuestions     []QuestionCount `json:"top_questions"`
}
type QuestionCount struct {
	Question string `json:"question"`
	Count    int    `json:"count"`
}

func (s *Store) QueuedJobs(ctx context.Context, limit int) ([]Work, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,document_id::text,source_content,source_type FROM ingestion_jobs WHERE status='queued' ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Work
	for rows.Next() {
		var w Work
		if err := rows.Scan(&w.JobID, &w.DocumentID, &w.Content, &w.SourceType); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (s *Store) Documents(ctx context.Context) ([]domain.Document, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id::text,d.title,d.source_type,d.content_hash,COALESCE(d.active_version_id::text,''),d.created_at FROM documents d WHERE d.status='active' ORDER BY d.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []domain.Document
	for rows.Next() {
		var d domain.Document
		if err := rows.Scan(&d.ID, &d.Title, &d.SourceType, &d.ContentHash, &d.ActiveVersionID, &d.CreatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}
func (s *Store) DocumentSource(ctx context.Context, documentID string) (Work, error) {
	var w Work
	err := s.pool.QueryRow(ctx, `SELECT id::text,document_id::text,source_content,source_type FROM ingestion_jobs WHERE document_id=$1 AND status='completed' ORDER BY completed_at DESC LIMIT 1`, documentID).Scan(&w.JobID, &w.DocumentID, &w.Content, &w.SourceType)
	return w, err
}
func (s *Store) DeleteDocument(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE documents SET status='deleted',active_version_id=NULL,deleted_at=now(),updated_at=now() WHERE id=$1 AND status='active'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
func (s *Store) AddDeadLetter(ctx context.Context, id, operation, conversationID, messageID string, payload any, cause error) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO dead_letters(id,operation,conversation_id,message_id,payload,error) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5::jsonb,$6)`, id, operation, conversationID, messageID, string(data), cause.Error())
	return err
}

type StagedChunk struct {
	ID, Section, Content string
	Position             int
	Embedding            []float64
}

func (s *Store) PublishVersion(ctx context.Context, documentID, versionID string, chunks []StagedChunk) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO document_versions(id,document_id,status) VALUES($1,$2,'staging')`, versionID, documentID); err != nil {
		return err
	}
	for _, c := range chunks {
		if _, err = tx.Exec(ctx, `INSERT INTO chunks(id,version_id,position,section,content,embedding) VALUES($1,$2,$3,$4,$5,$6::vector)`, c.ID, versionID, c.Position, c.Section, c.Content, vector(c.Embedding)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE document_versions SET status='superseded' WHERE document_id=$1 AND status='active'`, documentID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE document_versions SET status='active',published_at=now() WHERE id=$1`, versionID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE documents SET active_version_id=$2,updated_at=now() WHERE id=$1`, documentID, versionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) VectorSearch(ctx context.Context, embedding []float64, limit int) ([]domain.Chunk, error) {
	return s.search(ctx, `SELECT c.id::text,d.title,c.section,c.content,c.position,1-(c.embedding <=> $1::vector) AS score FROM chunks c JOIN document_versions v ON v.id=c.version_id AND v.status='active' JOIN documents d ON d.id=v.document_id AND d.status='active' ORDER BY c.embedding <=> $1::vector LIMIT $2`, vector(embedding), limit)
}
func (s *Store) TextSearch(ctx context.Context, query string, limit int) ([]domain.Chunk, error) {
	return s.search(ctx, `SELECT c.id::text,d.title,c.section,c.content,c.position,ts_rank(c.content_tsv,websearch_to_tsquery('simple',$1)) AS score FROM chunks c JOIN document_versions v ON v.id=c.version_id AND v.status='active' JOIN documents d ON d.id=v.document_id AND d.status='active' WHERE c.content_tsv @@ websearch_to_tsquery('simple',$1) ORDER BY score DESC LIMIT $2`, query, limit)
}
func (s *Store) search(ctx context.Context, query string, arg any, limit int) ([]domain.Chunk, error) {
	rows, err := s.pool.Query(ctx, query, arg, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Chunk
	for rows.Next() {
		var c domain.Chunk
		if err := rows.Scan(&c.ID, &c.Document, &c.Section, &c.Content, &c.Position, &c.Score); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetOrCreateConversation(ctx context.Context, id, session, channel string) (domain.Conversation, error) {
	var c domain.Conversation
	err := s.pool.QueryRow(ctx, `SELECT id::text,session_id,channel FROM conversations WHERE session_id=$1`, session).Scan(&c.ID, &c.SessionID, &c.Channel)
	if err == nil {
		return c, nil
	}
	if err != pgx.ErrNoRows {
		return c, err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO conversations(id,session_id,channel) VALUES($1,$2,$3)`, id, session, channel)
	return domain.Conversation{ID: id, SessionID: session, Channel: channel}, err
}
func (s *Store) AddMessage(ctx context.Context, m domain.Message, latencyMS int) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO messages(id,conversation_id,role,content,status,latency_ms,not_found) VALUES($1,$2,$3,$4,'completed',$5,$6)`, m.ID, m.ConversationID, m.Role, m.Content, latencyMS, m.NotFound)
	return err
}
func (s *Store) Messages(ctx context.Context, conversationID string, limit int) ([]domain.Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,conversation_id::text,role,content,not_found,created_at FROM messages WHERE conversation_id=$1 ORDER BY created_at DESC LIMIT $2`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rev []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.NotFound, &m.CreatedAt); err != nil {
			return nil, err
		}
		rev = append(rev, m)
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, rows.Err()
}
func (s *Store) AddSources(ctx context.Context, messageID string, sources []domain.Source) error {
	for i, src := range sources {
		if _, err := s.pool.Exec(ctx, `INSERT INTO message_sources(message_id,chunk_id,rank,score) VALUES($1,$2,$3,$4)`, messageID, src.ChunkID, i+1, src.Score); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) AddFeedback(ctx context.Context, messageID string, rating int, comment string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO feedback(message_id,rating,comment) VALUES($1,$2,$3) ON CONFLICT(message_id) DO UPDATE SET rating=EXCLUDED.rating,comment=EXCLUDED.comment,created_at=now()`, messageID, rating, comment)
	return err
}
func (s *Store) AddEscalation(ctx context.Context, id, conversationID, reason string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO escalations(id,conversation_id,reason) VALUES($1,$2,$3)`, id, conversationID, reason)
	return err
}
func (s *Store) Conversations(ctx context.Context, channel string, limit int) ([]AdminConversation, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT c.id::text,c.session_id,c.channel,c.created_at,COALESCE(MAX(m.created_at),c.created_at),COUNT(m.id) FROM conversations c LEFT JOIN messages m ON m.conversation_id=c.id`
	args := []any{}
	if channel != "" {
		query += ` WHERE c.channel=$1`
		args = append(args, channel)
	}
	query += ` GROUP BY c.id ORDER BY MAX(m.created_at) DESC NULLS LAST LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminConversation
	for rows.Next() {
		var c AdminConversation
		if err := rows.Scan(&c.ID, &c.SessionID, &c.Channel, &c.CreatedAt, &c.LastMessageAt, &c.MessageCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) Conversation(ctx context.Context, id string) (AdminConversation, []AdminMessage, error) {
	var conversation AdminConversation
	err := s.pool.QueryRow(ctx, `SELECT c.id::text,c.session_id,c.channel,c.created_at,COALESCE(MAX(m.created_at),c.created_at),COUNT(m.id) FROM conversations c LEFT JOIN messages m ON m.conversation_id=c.id WHERE c.id=$1 GROUP BY c.id`, id).Scan(&conversation.ID, &conversation.SessionID, &conversation.Channel, &conversation.CreatedAt, &conversation.LastMessageAt, &conversation.MessageCount)
	if err != nil {
		return conversation, nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,conversation_id::text,role,content,not_found,created_at FROM messages WHERE conversation_id=$1 ORDER BY created_at`, id)
	if err != nil {
		return conversation, nil, err
	}
	defer rows.Close()
	var messages []AdminMessage
	for rows.Next() {
		var m AdminMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.NotFound, &m.CreatedAt); err != nil {
			return conversation, nil, err
		}
		sourceRows, err := s.pool.Query(ctx, `SELECT ms.chunk_id::text,d.title,c.section,c.content,ms.score FROM message_sources ms JOIN chunks c ON c.id=ms.chunk_id JOIN document_versions v ON v.id=c.version_id JOIN documents d ON d.id=v.document_id WHERE ms.message_id=$1 ORDER BY ms.rank`, m.ID)
		if err != nil {
			return conversation, nil, err
		}
		for sourceRows.Next() {
			var source domain.Source
			if err := sourceRows.Scan(&source.ChunkID, &source.Document, &source.Section, &source.Excerpt, &source.Score); err != nil {
				sourceRows.Close()
				return conversation, nil, err
			}
			m.Sources = append(m.Sources, source)
		}
		if err := sourceRows.Err(); err != nil {
			sourceRows.Close()
			return conversation, nil, err
		}
		sourceRows.Close()
		messages = append(messages, m)
	}
	return conversation, messages, rows.Err()
}
func (s *Store) Dashboard(ctx context.Context) (Dashboard, error) {
	var d Dashboard
	err := s.pool.QueryRow(ctx, `WITH counts AS (SELECT COUNT(*) FILTER (WHERE role='user')::float AS questions,COUNT(*) FILTER (WHERE role='assistant' AND not_found)::float AS not_found,COUNT(*) FILTER (WHERE role='assistant')::float AS answers,COALESCE(AVG(latency_ms) FILTER (WHERE role='assistant'),0) AS avg_latency FROM messages), feedback_counts AS (SELECT COUNT(*) FILTER (WHERE rating=1) AS positive,COUNT(*) FILTER (WHERE rating=-1) AS negative FROM feedback) SELECT (SELECT COUNT(*) FROM messages WHERE role='user' AND created_at >= date_trunc('day',now())),COALESCE(not_found/NULLIF(answers,0),0),COALESCE((SELECT COUNT(*)::float FROM escalations)/NULLIF(questions,0),0),avg_latency,positive,negative FROM counts,feedback_counts`).Scan(&d.QuestionsToday, &d.NotFoundRatio, &d.EscalationRatio, &d.AverageLatencyMS, &d.PositiveFeedback, &d.NegativeFeedback)
	if err != nil {
		return d, err
	}
	rows, err := s.pool.Query(ctx, `SELECT content,COUNT(*) FROM messages WHERE role='user' GROUP BY content ORDER BY COUNT(*) DESC,content LIMIT 5`)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var q QuestionCount
		if err := rows.Scan(&q.Question, &q.Count); err != nil {
			return d, err
		}
		d.TopQuestions = append(d.TopQuestions, q)
	}
	return d, rows.Err()
}

func vector(v []float64) string {
	parts := make([]string, len(v))
	for i, n := range v {
		parts[i] = fmt.Sprintf("%.8f", n)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
