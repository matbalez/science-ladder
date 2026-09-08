CREATE TABLE review_notifications (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 candidate_id uuid REFERENCES candidates(id),
 version_id uuid REFERENCES challenge_versions(id),
 is_test boolean NOT NULL DEFAULT false,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN('pending','sending','accepted','failed','uncertain','cancelled')),
 attempts integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL DEFAULT now(),
 lease_token uuid,
 lease_expires_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 accepted_at timestamptz,
 provider_message_id text,
 last_error text,
 CHECK((is_test AND candidate_id IS NULL AND version_id IS NULL) OR (NOT is_test AND ((candidate_id IS NOT NULL)::int+(version_id IS NOT NULL)::int)=1))
);
CREATE INDEX review_notifications_pending ON review_notifications(status,available_at);
-- Transactional outbox: transitions and notifications commit or roll back together.
CREATE FUNCTION notify_candidate_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status='human_review_required' AND (TG_OP='INSERT' OR OLD.status IS DISTINCT FROM NEW.status) THEN
  INSERT INTO review_notifications(candidate_id) VALUES(NEW.id);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER candidate_review_notification AFTER INSERT OR UPDATE OF status ON candidates FOR EACH ROW EXECUTE FUNCTION notify_candidate_review();
CREATE FUNCTION notify_scientific_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.review_status='human_review_required' AND (TG_OP='INSERT' OR OLD.review_status IS DISTINCT FROM NEW.review_status) THEN
  INSERT INTO review_notifications(version_id) VALUES(NEW.id);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER scientific_review_notification AFTER INSERT OR UPDATE OF review_status ON challenge_versions FOR EACH ROW EXECUTE FUNCTION notify_scientific_review();
-- Existing actionable reviews should not silently miss notification when email is connected.
INSERT INTO review_notifications(version_id) SELECT id FROM challenge_versions WHERE review_status='human_review_required' AND status NOT IN('withdrawn','closed','superseded','rejected','compromised');
INSERT INTO review_notifications(candidate_id) SELECT ca.id FROM candidates ca WHERE ca.status='human_review_required' AND NOT EXISTS(SELECT 1 FROM challenges c WHERE c.candidate_id=ca.id);
