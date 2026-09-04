CREATE TABLE scientific_review_requests(
 id uuid PRIMARY KEY,
 version_id uuid NOT NULL REFERENCES challenge_versions(id),
 requested_by uuid NOT NULL REFERENCES users(id),
 reason text NOT NULL CHECK(length(reason) BETWEEN 20 AND 2000),
 status text NOT NULL DEFAULT 'queued' CHECK(status IN('queued','completed')),
 review_id uuid REFERENCES review_runs(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 completed_at timestamptz,
 CHECK((status='completed')=(review_id IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE UNIQUE INDEX one_pending_scientific_review ON scientific_review_requests(version_id) WHERE status='queued';
CREATE TRIGGER immutable_scientific_review BEFORE UPDATE OR DELETE ON review_runs FOR EACH ROW EXECUTE FUNCTION deny_audit_mutation();
