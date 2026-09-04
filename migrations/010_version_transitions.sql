ALTER TABLE challenge_versions ADD COLUMN predecessor_id uuid REFERENCES challenge_versions(id);
ALTER TABLE challenge_versions ADD COLUMN transition_kind text NOT NULL DEFAULT 'season' CHECK(transition_kind IN('season','security_migration'));
ALTER TABLE challenge_versions ADD COLUMN prior_frontier_digest text;
ALTER TABLE challenge_versions ADD COLUMN migration_receipt_digest text REFERENCES receipts(digest);
CREATE UNIQUE INDEX one_pending_security_migration ON challenge_versions(predecessor_id) WHERE transition_kind='security_migration' AND status NOT IN('published','closed','superseded','compromised','rejected');
