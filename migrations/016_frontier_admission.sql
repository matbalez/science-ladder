-- Claims are untrusted metadata. Only consumed tickets authorize solver preparation.
CREATE TABLE frontier_tickets (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES users(id),
 version_id uuid NOT NULL REFERENCES challenge_versions(id),
 repository text NOT NULL,
 ref text NOT NULL,
 artifact_digest text NOT NULL,
 claim jsonb NOT NULL,
 frontier_ticks text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now() + interval '15 minutes',
 consumed_at timestamptz,
 intent_id uuid UNIQUE REFERENCES submission_intents(id)
);
CREATE INDEX frontier_tickets_owner_time ON frontier_tickets(owner_id,created_at);
CREATE INDEX frontier_tickets_time ON frontier_tickets(created_at);
CREATE INDEX frontier_tickets_artifact ON frontier_tickets(owner_id,version_id,artifact_digest);
