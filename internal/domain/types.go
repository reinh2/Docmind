package domain

import "time"

type Document struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	SourceType      string    `json:"source_type"`
	ContentHash     string    `json:"content_hash"`
	ActiveVersionID string    `json:"active_version_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Job struct {
	ID          string     `json:"id"`
	DocumentID  string     `json:"document_id"`
	Status      string     `json:"status"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Chunk struct {
	ID       string  `json:"id"`
	Document string  `json:"document"`
	Section  string  `json:"section"`
	Content  string  `json:"content"`
	Position int     `json:"position"`
	Score    float64 `json:"score"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	NotFound       bool      `json:"not_found"`
	CreatedAt      time.Time `json:"created_at"`
}

type Source struct {
	ChunkID  string  `json:"chunk_id"`
	Document string  `json:"document"`
	Section  string  `json:"section"`
	Excerpt  string  `json:"excerpt"`
	Score    float64 `json:"score"`
}

type Conversation struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Channel   string `json:"channel"`
}
