-- Admission metadata only: conversations and raw IP addresses are not stored.
CREATE TABLE challenge_tutor_requests (
    id uuid PRIMARY KEY,
    visitor_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL DEFAULT now() + interval '2 minutes',
    finished_at timestamptz
);
CREATE INDEX challenge_tutor_requests_time ON challenge_tutor_requests(created_at);
CREATE INDEX challenge_tutor_requests_visitor ON challenge_tutor_requests(visitor_hash, created_at);
