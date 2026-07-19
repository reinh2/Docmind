CREATE TABLE feedback (message_id uuid PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,rating smallint NOT NULL CHECK (rating IN (-1,1)),comment text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE escalations (id uuid PRIMARY KEY,conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,reason text NOT NULL,status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','closed')),created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX escalations_status_idx ON escalations(status,created_at DESC);
