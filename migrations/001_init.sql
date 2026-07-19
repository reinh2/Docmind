CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version bigint PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE documents (
  id uuid PRIMARY KEY,
  title text NOT NULL,
  source_type text NOT NULL,
  content_hash text NOT NULL UNIQUE,
  active_version_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE document_versions (
  id uuid PRIMARY KEY,
  document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('staging','active','superseded','failed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz
);

ALTER TABLE documents ADD CONSTRAINT documents_active_version_fk
  FOREIGN KEY (active_version_id) REFERENCES document_versions(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE chunks (
  id uuid PRIMARY KEY,
  version_id uuid NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
  position integer NOT NULL,
  section text NOT NULL DEFAULT '',
  content text NOT NULL,
  content_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
  embedding vector(64) NOT NULL,
  UNIQUE(version_id, position)
);
CREATE INDEX chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX chunks_tsv_idx ON chunks USING gin (content_tsv);

CREATE TABLE ingestion_jobs (
  id uuid PRIMARY KEY,
  document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('queued','processing','completed','failed')),
  source_content text NOT NULL,
  source_type text NOT NULL,
  error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz
);
CREATE INDEX ingestion_jobs_status_idx ON ingestion_jobs(status, created_at);

CREATE TABLE conversations (
  id uuid PRIMARY KEY,
  session_id text NOT NULL UNIQUE,
  channel text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE messages (
  id uuid PRIMARY KEY,
  conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('user','assistant','system')),
  content text NOT NULL,
  status text NOT NULL DEFAULT 'completed',
  latency_ms integer,
  not_found boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX messages_conversation_idx ON messages(conversation_id, created_at);

CREATE TABLE message_sources (
  message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  chunk_id uuid NOT NULL REFERENCES chunks(id),
  rank integer NOT NULL,
  score real NOT NULL,
  PRIMARY KEY(message_id, chunk_id)
);
