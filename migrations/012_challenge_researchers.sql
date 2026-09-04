-- Curated people/work context is editorial metadata, never part of a scoring lock.
CREATE TABLE challenge_researcher_editions (
 id uuid PRIMARY KEY,
 edition_sequence bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 version_id uuid NOT NULL REFERENCES challenge_versions(id),
 editor_id uuid NOT NULL REFERENCES users(id),
 editor_github_id bigint NOT NULL CHECK(editor_github_id > 0),
 editor_login text NOT NULL,
 researchers jsonb NOT NULL CHECK(jsonb_typeof(researchers)='array' AND jsonb_array_length(researchers)<=6),
 reason text NOT NULL CHECK(length(reason) BETWEEN 20 AND 2000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX challenge_researcher_latest ON challenge_researcher_editions(version_id,edition_sequence DESC);
CREATE TRIGGER immutable_challenge_researchers BEFORE UPDATE OR DELETE ON challenge_researcher_editions FOR EACH ROW EXECUTE FUNCTION deny_audit_mutation();
