ALTER TABLE documents ADD COLUMN status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','deleted'));
ALTER TABLE documents ADD COLUMN source_url text;
ALTER TABLE documents ADD COLUMN deleted_at timestamptz;
ALTER TABLE ingestion_jobs ADD COLUMN attempts integer NOT NULL DEFAULT 0;

CREATE TABLE dead_letters (
  id uuid PRIMARY KEY,
  operation text NOT NULL,
  conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  error text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX dead_letters_created_idx ON dead_letters(created_at DESC);
